package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
	"golang.org/x/exp/slog"
)

const lsName = "logSeq"

var version string = "0.0.1"
var handler protocol.Handler
var logger *slog.Logger
var lsClient *resty.Client

func main() {
	lf, err := os.Create("/Users/jacobmikesell/Workspace/logseqlsp/lsp.json")
	if err != nil {
		panic(err)
	}
	logger = slog.New(slog.NewJSONHandler(lf))
	logger.Info("test", slog.String("version", version))

	lsClient = resty.New().SetBaseURL("http://localhost:12315").SetAuthToken("test")
	lsClient.HeaderAuthorizationKey = "Authorization"

	handler = protocol.Handler{
		Initialize:             initialize,
		Initialized:            initialized,
		Shutdown:               shutdown,
		SetTrace:               setTrace,
		TextDocumentDefinition: definition,
		// TextDocumentCodeAction: codeAction,
	}

	server := server.NewServer(&handler, lsName, false)

	server.RunStdio()
}

func initialize(context *glsp.Context, params *protocol.InitializeParams) (any, error) {
	capabilities := handler.CreateServerCapabilities()
	// capabilities.CodeActionProvider = true
	capabilities.DefinitionProvider = true

	return protocol.InitializeResult{
		Capabilities: capabilities,
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name:    lsName,
			Version: &version,
		},
	}, nil
}

func initialized(context *glsp.Context, params *protocol.InitializedParams) error {
	return nil
}

func shutdown(context *glsp.Context) error {
	protocol.SetTraceValue(protocol.TraceValueOff)
	return nil
}

func setTrace(context *glsp.Context, params *protocol.SetTraceParams) error {
	protocol.SetTraceValue(params.Value)
	return nil
}

func codeAction(context *glsp.Context, params *protocol.CodeActionParams) (interface{}, error) {
	logger.Info("code action fired", params.Range)

	return nil, nil
}
func definition(context *glsp.Context, params *protocol.DefinitionParams) (interface{}, error) {
	fileName := path.Base(params.TextDocument.URI)
	dir := path.Dir(params.TextDocument.URI)
	logger.Info("code action fired", slog.String("uri", fileName), slog.Any("position", params.Position))
	t, err := time.Parse("2006_01_02", fileName[:10])
	if err != nil {
		logger.Info("failed to query ls api for file", err)
	}
	pageName := formatDate(t)
	logger.Info("code action fired", slog.String("pageName", pageName), slog.Any("position", params.Position))

	response, err := lsClient.R().SetBody(map[string]any{
		"method": "logseq.Editor.getPageBlocksTree",
		"args":   []string{pageName},
	}).SetResult(GetPageBlockTree{}).Post("/api")
	if err != nil {
		logger.Info("failed to query ls api for file", err)
	}
	logger.Info("request success")

	pageBlockTree, ok := response.Result().(*GetPageBlockTree)
	if !ok {
		logger.Info(fmt.Sprintf("%T", response.Result()), slog.Any("resp", response.Result()))
		return nil, nil
	}
	logger.Info("resp success")

	d := newDocument(*pageBlockTree, path.Join(dir, "."))
	logger.Info("doc success")

	for _, l := range d.links {
		if positionInRange(d.contents, l.Range, params.Position) {
			return &protocol.Location{URI: path.Join(l.dir, l.target+".md")}, nil
		}
	}
	return nil, nil
}

func positionInRange(content string, rng protocol.Range, pos protocol.Position) bool {
	start, end := rng.IndexesIn(content)
	i := pos.IndexIn(content)
	logger.Info("indexes", slog.Int("start", start), slog.Int("end", end), slog.Int("i", i))
	return i >= start && i <= end
}

type document struct {
	contents string
	links    []link
}
type link struct {
	target string
	dir    string
	Range  protocol.Range
}

var wikiLinkRegex = regexp.MustCompile(`\[*\[\[(.+?)]]`)

func newDocument(blockTree GetPageBlockTree, basedir string) document {
	var links []link
	var sb strings.Builder
	for line, block := range blockTree {
		for _, match := range wikiLinkRegex.FindAllStringSubmatchIndex(block.Content, -1) {
			href := block.Content[match[2]:match[3]]
			logger.Info("found match", slog.String("ref", href))

			links = append(links, newLink(href, basedir, line, match[0], match[1]))
		}
		sb.WriteString(block.Content)
	}
	return document{contents: sb.String(), links: links}
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
func newLink(href string, baseDir string, line, start, end int) link {
	if href == "" {
		return link{}
	}

	//// Go regexes work with bytes, but the LSP client expects character indexes.
	//start = strutil.ByteIndexToRuneIndex(line, start)
	//end = strutil.ByteIndexToRuneIndex(line, end)

	return link{
		target: href,
		dir:    path.Join(path.Dir(baseDir), "pages"),
		Range: protocol.Range{
			Start: protocol.Position{
				Line:      protocol.UInteger(line),
				Character: protocol.UInteger(start),
			},
			End: protocol.Position{
				Line:      protocol.UInteger(line),
				Character: protocol.UInteger(end),
			},
		},
	}
}

type GetPageBlockTree []GetPageBlockTreeElement

func UnmarshalGetPageBlockTree(data []byte) (GetPageBlockTree, error) {
	var r GetPageBlockTree
	err := json.Unmarshal(data, &r)
	return r, err
}

type GetPageBlockTreeElement struct {
	Properties           Properties  `json:"properties"`
	Unordered            bool        `json:"unordered"`
	JournalDay           *int64      `json:"journalDay,omitempty"`
	Parent               Left        `json:"parent"`
	Children             []Child     `json:"children"`
	ID                   int64       `json:"id"`
	PathRefs             []Left      `json:"pathRefs"`
	Level                int64       `json:"level"`
	UUID                 string      `json:"uuid"`
	Content              string      `json:"content"`
	Journal              bool        `json:"journal?"`
	Page                 Left        `json:"page"`
	Left                 Left        `json:"left"`
	Format               string      `json:"format"`
	Refs                 []Left      `json:"refs,omitempty"`
	PropertiesTextValues *Properties `json:"propertiesTextValues,omitempty"`
	PropertiesOrder      []string    `json:"propertiesOrder,omitempty"`
}

type Child struct {
	Properties Properties    `json:"properties"`
	Unordered  bool          `json:"unordered"`
	Parent     Left          `json:"parent"`
	Children   []interface{} `json:"children"`
	ID         int64         `json:"id"`
	PathRefs   []Left        `json:"pathRefs"`
	Level      int64         `json:"level"`
	UUID       string        `json:"uuid"`
	Content    string        `json:"content"`
	Journal    bool          `json:"journal?"`
	Page       Left          `json:"page"`
	Left       Left          `json:"left"`
	Format     string        `json:"format"`
}

type Left struct {
	ID int64 `json:"id"`
}

type Properties struct {
	Test *string `json:"test,omitempty"`
}
