package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/WhiskeyJack96/logseqlsp/document"
	"io"
	"net/url"
	"os"
	"path"
	"regexp"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
	"golang.org/x/exp/slog"
)

const lsName = "logSeq"

var version string = "0.0.1"

type graphInfo struct {
	name string
	path string

	pagesPath string

	journalsPath   string
	journalPattern string
	journalRegex   *regexp.Regexp

	client  *resty.Client
	logger  *slog.Logger
	handler protocol.Handler
}

func main() {
	lf, err := os.Create("/Users/jacobmikesell/Workspace/logseqlsp/lsp.json")
	if err != nil {
		panic(err)
	}

	logger := slog.New(slog.NewJSONHandler(lf))
	logger.Info("test", slog.String("version", version))
	defer func() {
		a := recover()
		logger.Info("panic recovered", slog.Any("r", a))
	}()

	lsClient := resty.New().SetBaseURL("http://localhost:12315").SetAuthToken("test")
	lsClient.HeaderAuthorizationKey = "Authorization"

	response, err := lsClient.R().SetBody(map[string]any{
		"method": "logseq.App.getCurrentGraph",
		"args":   nil,
	}).Post("/api")
	respMap := make(map[string]string)
	if err := json.Unmarshal(response.Body(), &respMap); err != nil {
		logger.Error("failed to read response exiting", err)
		return
	}
	info := graphInfo{
		name:           respMap["name"],
		path:           respMap["path"],
		pagesPath:      "pages",
		journalsPath:   "journals",
		journalPattern: "2006_01_02.md",
		journalRegex:   regexp.MustCompile(`\d{4}_\d{2}_\d{2}\.md`),
		client:         lsClient,
		logger:         logger,
	}

	info.handler = protocol.Handler{
		Initialize:             info.initialize,
		Initialized:            info.initialized,
		Shutdown:               info.shutdown,
		SetTrace:               info.setTrace,
		TextDocumentDefinition: info.definition,

		TextDocumentHover: func(context *glsp.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
			logger.Info("hover", slog.Any("params", params))
			readCloser, err := uriToReader(info.logger, params.TextDocument.URI)
			if err != nil {
				return nil, err
			}
			defer readCloser.Close()
			d, err := document.New(info.logger, readCloser)
			if err != nil {
				return nil, err
			}
			for _, l := range d.Links {
				if positionInRange(d.Contents, l.Range, params.Position) {
					switch l.Type {
					case document.Wiki, document.Tag, document.Prop:
						//handle journal link?? maybe call to api to determine if page exists+ is journal
						readCloser, err := os.Open(path.Join(info.path, info.pagesPath, l.Target))
						if err != nil {
							return nil, err
						}
						defer readCloser.Close()
						hoverDoc, err := document.New(logger, readCloser)
						if err != nil {
							return nil, err
						}
						readCloser.Close()
						return &protocol.Hover{Contents: hoverDoc.Contents}, nil

					case document.BlockEmbed:
						response, err := info.client.R().SetBody(map[string]any{
							"method": "logseq.App.getBlock",
							"args":   []string{l.Target},
						}).Post("/api")
						if err != nil {
							return nil, err
						}
						respMap := make(map[string]any)
						if err := json.Unmarshal(response.Body(), &respMap); err != nil {
							logger.Error("failed to read response exiting", err)
							return nil, err
						}
						return &protocol.Hover{Contents: respMap["content"]}, nil
					}
					return nil, nil
				}
			}

			return nil, nil
		},
		TextDocumentCodeAction: info.codeAction,
	}
	logger.Info("serving")

	s := server.NewServer(&info.handler, lsName, false)

	logger.Error("run error", s.RunStdio())
}

func (gi *graphInfo) initialize(context *glsp.Context, params *protocol.InitializeParams) (any, error) {
	capabilities := gi.handler.CreateServerCapabilities()
	capabilities.CodeActionProvider = true
	capabilities.DefinitionProvider = true
	capabilities.HoverProvider = true
	gi.logger.Info("initialize", slog.Any("caps", capabilities))

	return protocol.InitializeResult{
		Capabilities: capabilities,
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name:    lsName,
			Version: &version,
		},
	}, nil
}

func (gi *graphInfo) initialized(context *glsp.Context, params *protocol.InitializedParams) error {
	return nil
}

func (gi *graphInfo) shutdown(context *glsp.Context) error {
	protocol.SetTraceValue(protocol.TraceValueOff)
	return nil
}

func (gi *graphInfo) setTrace(context *glsp.Context, params *protocol.SetTraceParams) error {
	protocol.SetTraceValue(params.Value)
	return nil
}

func (gi *graphInfo) codeAction(context *glsp.Context, params *protocol.CodeActionParams) (interface{}, error) {
	gi.logger.Info("code action fired", params.Range)
	return nil, nil
}

func (gi *graphInfo) definition(context *glsp.Context, params *protocol.DefinitionParams) (interface{}, error) {
	fileName := path.Base(params.TextDocument.URI)
	dir := path.Dir(params.TextDocument.URI)
	gi.logger.Info("definition", slog.String("uri", params.TextDocument.URI), slog.Any("position", params.Position))
	t, err := time.Parse("2006_01_02", fileName[:10])
	if err != nil {
		gi.logger.Info("failed to query ls api for file", err)
	}
	pageName := formatDate(t)
	gi.logger.Info("code action fired", slog.String("pageName", pageName), slog.Any("position", params.Position))
	readCloser, err := uriToReader(gi.logger, params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	defer readCloser.Close()
	d, err := document.New(gi.logger, readCloser)
	if err != nil {
		return nil, err
	}
	for _, l := range d.Links {
		if positionInRange(d.Contents, l.Range, params.Position) {
			switch l.Type {
			case document.Wiki, document.Tag, document.Prop:
				gi.logger.Info(path.Join(gi.path, gi.pagesPath, l.Target))
				gi.logger.Info(path.Join(dir, l.Target))
				s := (&url.URL{Scheme: "file", Path: path.Join(gi.path, gi.pagesPath, l.Target)}).String()
				return &protocol.Location{URI: s}, nil
			case document.BlockEmbed:
				response, err := gi.client.R().SetBody(map[string]any{
					"method": "logseq.App.getBlock",
					"args":   []string{l.Target},
				}).Post("/api")
				if err != nil {
					return nil, err
				}
				respMap := make(map[string]any)
				if err := json.Unmarshal(response.Body(), &respMap); err != nil {
					gi.logger.Error("failed to read response exiting", err)
					return nil, err
				}
				gi.logger.Info("getblock", slog.Any("resp", respMap))
				page, ok := respMap["page"].(map[string]any)
				if !ok {
					return nil, errors.New("type mismatch")
				}
				response, err = gi.client.R().SetBody(map[string]any{
					"method": "logseq.App.getPage",
					"args":   []any{page["id"]},
				}).Post("/api")
				if err != nil {
					return nil, err
				}
				respMap = make(map[string]any)
				if err := json.Unmarshal(response.Body(), &respMap); err != nil {
					gi.logger.Error("failed to read response exiting", err)
					return nil, err
				}
				gi.logger.Info("getpage", slog.Any("resp", respMap))
				s := (&url.URL{Scheme: "file", Path: path.Join(gi.path, gi.pagesPath, respMap["name"].(string)+".md")}).String()

				return &protocol.Location{URI: s}, nil
			}
		}
	}
	return nil, nil
}

func uriToReader(logger *slog.Logger, uri string) (io.ReadCloser, error) {
	requestURI, err := url.ParseRequestURI(uri)
	if err != nil {
		return nil, err
	}
	if requestURI.Scheme != "file" {
		return nil, fmt.Errorf("unsupported uri scheme: %s", requestURI.Scheme)
	}
	file, err := os.Open(requestURI.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logger.Info("could not find file", slog.String("uri", uri))
			return nil, err
		}
		return nil, err
	}
	return file, nil
}

func positionInRange(content string, rng protocol.Range, pos protocol.Position) bool {
	start, end := rng.IndexesIn(content)
	i := pos.IndexIn(content)
	//logger.Info("indexes", slog.Int("start", start), slog.Int("end", end), slog.Int("i", i))
	return i >= start && i <= end
}

func formatDate(t time.Time) string {
	suffix := "th"
	switch t.Day() {
	case 1, 21, 31:
		suffix = "st"
	case 2, 22:
		suffix = "nd"
	case 3, 23:
		suffix = "rd"
	}
	return t.Format("Jan 2" + suffix + ", 2006")
}
