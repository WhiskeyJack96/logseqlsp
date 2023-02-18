package main

import (
	"errors"
	"fmt"
	"github.com/WhiskeyJack96/logseqlsp/document"
	"github.com/WhiskeyJack96/logseqlsp/files"
	"github.com/WhiskeyJack96/logseqlsp/logseq"
	"github.com/spf13/cobra"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
	"golang.org/x/exp/slog"
	"io"
	"os"
	"path"
	"strings"
)

const lsName = "logSeq"

var version = "0.0.1"

// TODO get pages/journals path from the logseq api or make them config values
type graphInfo struct {
	name string
	path string

	pagesPath    string
	journalsPath string

	client  logseq.Client
	logger  *slog.Logger
	handler protocol.Handler
	config  config
}

type config struct {
	logging bool
	port    int32
	token   string
	logFile string
}

func main() {
	root := cobra.Command{
		Use:  "",
		RunE: run,
	}
	root.Flags().StringP("token", "t", "", "token to auth to logseq")
	root.Flags().BoolP("logging", "l", true, "enable/disable logging")
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr)).Error("unexpected error", err)
	}
	root.Flags().String("log-file", path.Join(userHomeDir, ".config/logseqlsp/log.json"), "file to log too defaults to (~/.config/logseqlsp/log.json)")
	root.Flags().Int32P("port", "p", 12315, "port logseq is listening on")

	err = root.Execute()
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr)).Error("unexpected error", err)
	}
}
func run(cmd *cobra.Command, args []string) error {
	port, err := cmd.Flags().GetInt32("port")
	if err != nil {
		return err
	}
	logging, err := cmd.Flags().GetBool("logging")
	if err != nil {
		return err
	}
	token, err := cmd.Flags().GetString("token")
	if err != nil {
		return err
	}
	logFile, err := cmd.Flags().GetString("log-file")
	if err != nil {
		return err
	}

	logger, err := newLogger(logging, logFile)
	if err != nil {
		return err
	}
	logger.Debug("staring up", slog.String("version", version))
	defer func() {
		a := recover()
		if a != nil {
			logger.Warn("panic recovered", slog.Any("r", a))
		}
	}()
	client, err := logseq.NewClient(logger, logseq.WithToken(token), logseq.WithBaseUrl(fmt.Sprintf("http://localhost:%d/api", port)))
	if err != nil {
		return err
	}

	graph, err := client.CurrentGraph()
	if err != nil {
		return err
	}

	info := graphInfo{
		name:         graph.Name,
		path:         graph.Path,
		pagesPath:    "pages",
		journalsPath: "journals",
		client:       client,
		logger:       logger,
		config: config{
			logging: logging,
			port:    port,
			token:   token,
			logFile: logFile,
		},
	}

	info.handler = protocol.Handler{
		Initialize:  info.initialize,
		Initialized: info.initialized,
		Shutdown:    info.shutdown,
		SetTrace:    info.setTrace,
		TextDocumentDidOpen: func(context *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
			info.logger.Info(context.Method, slog.String("file", params.TextDocument.URI))

			return nil
		},
		TextDocumentDidChange: func(context *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
			info.logger.Info(context.Method, slog.String("file", params.TextDocument.URI))
			return nil
		},
		TextDocumentDidClose: func(context *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
			info.logger.Info(context.Method, slog.String("file", params.TextDocument.URI))
			return nil
		},
		TextDocumentWillSave: func(context *glsp.Context, params *protocol.WillSaveTextDocumentParams) error {
			info.logger.Info(context.Method, slog.String("file", params.TextDocument.URI))
			return nil
		},
		TextDocumentWillSaveWaitUntil: func(context *glsp.Context, params *protocol.WillSaveTextDocumentParams) ([]protocol.TextEdit, error) {
			info.logger.Info(context.Method, slog.String("file", params.TextDocument.URI))
			return nil, nil
		},
		TextDocumentDidSave: func(context *glsp.Context, params *protocol.DidSaveTextDocumentParams) error {
			info.logger.Info(context.Method, slog.String("file", params.TextDocument.URI))
			return nil
		},
		TextDocumentHover:             info.hover,
		TextDocumentDefinition:        info.definition,
		TextDocumentDocumentHighlight: info.highlight,
		TextDocumentCodeAction:        info.codeAction,
		TextDocumentDocumentLink:      info.links,
	}
	logger.Info("serving")

	s := server.NewServer(&info.handler, lsName, false)
	err = s.RunStdio()
	if err != nil {
		logger.Error("run error: ", err)
	}
	return nil
}

func newLogger(logging bool, logFile string) (*slog.Logger, error) {
	if !logging {
		return slog.New(slog.NewJSONHandler(io.Discard)), nil
	}
	if _, err := os.ReadDir(path.Dir(logFile)); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		err = os.MkdirAll(path.Dir(logFile), 0744)
		if err != nil {
			return nil, err
		}
	}
	lf, err := os.Create(logFile)
	if err != nil {
		return nil, err
	}
	return slog.New(slog.NewJSONHandler(lf)), nil
}

func (gi *graphInfo) initialize(context *glsp.Context, params *protocol.InitializeParams) (any, error) {
	capabilities := gi.handler.CreateServerCapabilities()
	capabilities.CodeActionProvider = true
	capabilities.DefinitionProvider = true
	capabilities.HoverProvider = true
	capabilities.DocumentHighlightProvider = true
	capabilities.TextDocumentSync = &protocol.TextDocumentSyncOptions{
		OpenClose:         &protocol.True,
		WillSave:          &protocol.True,
		WillSaveWaitUntil: &protocol.True,
		Save:              &protocol.SaveOptions{IncludeText: &protocol.True},
	}
	capabilities.DocumentLinkProvider = &protocol.DocumentLinkOptions{
		ResolveProvider: &protocol.True,
	}
	gi.logger.Info("initialize", slog.Any("caps", capabilities), slog.Any("client", params.Capabilities))

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
	gi.logger.Info("definition", slog.String("uri", params.TextDocument.URI), slog.Any("position", params.Position))
	d, err := readDocumentIdentifier(params.TextDocument)
	if err != nil {
		return nil, err
	}
	l, err := d.FindLinkForPosition(params.Position)
	if err != nil {
		gi.logger.Error("find link", err)
		if errors.Is(err, document.ErrLinkNotFound) {
			return nil, nil
		}
		return nil, err
	}

	s, err := gi.linkToURI(l)
	if err != nil {
		return nil, err
	}
	gi.logger.Info("found link uri", slog.Any("uri", s))

	return &protocol.Location{
		URI: *s,
		//TODO compute range when travelling to a definition that isnt a page
		//Range: protocol.Range{
		//	Start: protocol.Position{
		//		Line:      3,
		//		Character: 0,
		//	},
		//},
	}, nil

}

func (gi *graphInfo) links(ctx *glsp.Context, params *protocol.DocumentLinkParams) ([]protocol.DocumentLink, error) {
	d, err := readDocumentIdentifier(params.TextDocument)
	if err != nil {
		return nil, err
	}
	var dlinks []protocol.DocumentLink
	for _, link := range d.Links {
		uri, err := gi.linkToURI(link)
		if err != nil {
			return nil, err
		}
		dlinks = append(dlinks, protocol.DocumentLink{
			Range:  link.Range,
			Target: uri,
			//Tooltip: nil,
		})
	}
	return dlinks, nil
}

func (gi *graphInfo) hover(context *glsp.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	gi.logger.Info("hover", slog.Any("params", params))
	d, err := readDocumentIdentifier(params.TextDocument)
	if err != nil {
		return nil, err
	}
	l, err := d.FindLinkForPosition(params.Position)
	if err != nil {
		if errors.Is(err, document.ErrLinkNotFound) {
			return nil, nil
		}
		return nil, err
	}
	switch l.Type {
	case document.Wiki, document.Tag, document.Prop:
		hoverDoc, err := gi.linkToDocument(l)
		if err != nil {
			gi.logger.Error("could not find file", err, slog.Any("link", l))
			if errors.Is(err, os.ErrNotExist) {
				return nil, nil
			}
			return nil, err
		}
		return &protocol.Hover{Contents: hoverDoc.Contents, Range: &l.Range}, nil
	case document.Query:
		response, err := gi.client.Query(l.Target)
		if err != nil {
			return nil, err
		}
		return &protocol.Hover{Contents: gi.queryToMarkup(response), Range: &l.Range}, nil
	case document.BlockEmbed:
		response, err := gi.client.GetBlock(l.Target)
		if err != nil {
			return nil, err
		}
		return &protocol.Hover{Contents: gi.blockToMarkup(response), Range: &l.Range}, nil
	}
	return nil, nil
}

func (gi *graphInfo) highlight(context *glsp.Context, params *protocol.DocumentHighlightParams) ([]protocol.DocumentHighlight, error) {
	d, err := readDocumentIdentifier(params.TextDocument)
	if err != nil {
		return nil, err
	}
	var highlights []protocol.DocumentHighlight
	var primaryLink string
	link, err := d.FindLinkForPosition(params.Position)
	if err != nil {
		if errors.Is(err, document.ErrLinkNotFound) {
			return nil, nil
		}
		return nil, err
	}
	kindText := protocol.DocumentHighlightKindText
	primaryLink = link.Target
	highlights = append(highlights, protocol.DocumentHighlight{
		Range: link.Range,
		Kind:  &kindText,
	})
	for _, l := range d.Links {
		if l.Target == primaryLink && !positionInRange(d.Contents, l.Range, params.Position) {
			highlights = append(highlights, protocol.DocumentHighlight{
				Range: l.Range,
				Kind:  &kindText,
			})
		}
	}
	return highlights, nil
}

func readDocumentIdentifier(td protocol.TextDocumentIdentifier) (document.Document, error) {
	readCloser, err := files.URIToReader(td.URI)
	if err != nil {
		return document.Document{}, err
	}
	defer readCloser.Close()
	d, err := document.New(readCloser)
	if err != nil {
		return document.Document{}, err
	}
	return d, nil
}

func (gi *graphInfo) linkToURI(l document.Link) (*protocol.DocumentUri, error) {
	var page logseq.Page
	switch l.Type {
	case document.Wiki, document.Tag, document.Prop:
		if l.Target == "" {
			return nil, nil
		}
		var err error
		page, err = gi.client.GetPageByName(l.Target)
		if err != nil {
			return nil, err
		}
	case document.BlockEmbed:
		block, err := gi.client.GetBlock(l.Target)
		if err != nil {
			gi.logger.Error("error in linkToUri", err, slog.Any("link", l))
			return nil, fmt.Errorf("error calling getBlock: %w", err)
		}
		page, err = gi.client.GetPageById(block.Page.ID)
		if err != nil {
			gi.logger.Error("error in linkToUri", fmt.Errorf("error calling getPage: %w", err))
			return nil, fmt.Errorf("error calling getPage: %w", err)
		}
		gi.logger.Info("found page", slog.Any("page", page), slog.Any("block", block.Page.ID))
	default:
		gi.logger.Error("error in linkToUri", fmt.Errorf("unsupported link type: %s", l.Type))
		return nil, fmt.Errorf("unsupported link type: %s", l.Type)
	}
	uri, err := page.ToURI(gi.path, gi.journalsPath, gi.pagesPath)
	if err != nil {
		gi.logger.Error("error converting page to URI", err)
		return nil, err
	}
	return &uri, nil
}

func (gi *graphInfo) queryToMarkup(response logseq.Query) protocol.MarkupContent {
	s := protocol.MarkupContent{
		Kind:  protocol.MarkupKindMarkdown,
		Value: "",
	}
	for i, m := range response {
		s.Value = s.Value + fmt.Sprintf("Result %d:\n", i) + m.Content + "\n" + "\n" + strings.Repeat("_", len(m.Content)+1) + "\n" + "\n"
	}
	return s
}

func (gi *graphInfo) blockToMarkup(response logseq.Block) protocol.MarkupContent {
	s := protocol.MarkupContent{
		Kind:  protocol.MarkupKindMarkdown,
		Value: "- " + response.Content + "\n",
	}
	for _, m := range response.Children {
		s.Value = s.Value + "\t- " + m.Content + "\n"
	}
	return s
}

func (gi *graphInfo) linkToDocument(l document.Link) (document.Document, error) {
	uri, err := gi.linkToURI(l)
	if err != nil {
		return document.Document{}, err
	}
	file, err := files.URIToReader(*uri)
	if err != nil {
		return document.Document{}, err
	}
	defer file.Close()
	doc, err := document.New(file)
	if err != nil {
		return document.Document{}, err
	}
	return doc, nil
}

func positionInRange(content string, rng protocol.Range, pos protocol.Position) bool {
	start, end := rng.IndexesIn(content)
	i := pos.IndexIn(content)
	//logger.Info("indexes", slog.Int("start", start), slog.Int("end", end), slog.Int("i", i))
	return i >= start && i <= end
}
