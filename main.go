package main

import (
	"errors"
	"fmt"
	"github.com/WhiskeyJack96/logseqlsp/document"
	"github.com/WhiskeyJack96/logseqlsp/files"
	"github.com/WhiskeyJack96/logseqlsp/logseq"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
	"golang.org/x/exp/slog"
	"os"
	"strings"
)

const lsName = "logSeq"

var version string = "0.0.1"

type graphInfo struct {
	name string
	path string

	pagesPath string

	journalsPath string

	client  logseq.Client
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
	client, err := logseq.NewClient()
	if err != nil {
		panic(err)
	}

	graph, err := client.CurrentGraph()
	if err != nil {
		panic(err)
	}

	info := graphInfo{
		name:         graph.Name,
		path:         graph.Path,
		pagesPath:    "pages",
		journalsPath: "journals",
		client:       client,
		logger:       logger,
	}

	info.handler = protocol.Handler{
		//CancelRequest:                      nil,
		//Progress:                           nil,
		Initialize:  info.initialize,
		Initialized: info.initialized,
		Shutdown:    info.shutdown,
		//Exit:                               nil,
		//LogTrace:                           nil,
		SetTrace: info.setTrace,
		//WindowWorkDoneProgressCancel:       nil,
		//WorkspaceDidChangeWorkspaceFolders: nil,
		//WorkspaceDidChangeConfiguration:    nil,
		//WorkspaceDidChangeWatchedFiles:     nil,
		//WorkspaceSymbol:                    nil,
		//WorkspaceExecuteCommand:            nil,
		//WorkspaceWillCreateFiles:           nil,
		//WorkspaceDidCreateFiles:            nil,
		//WorkspaceWillRenameFiles:           nil,
		//WorkspaceDidRenameFiles:            nil,
		//WorkspaceWillDeleteFiles:           nil,
		//WorkspaceDidDeleteFiles:            nil,
		TextDocumentDidOpen:           nil,
		TextDocumentDidChange:         nil,
		TextDocumentWillSave:          nil,
		TextDocumentWillSaveWaitUntil: nil,
		TextDocumentDidSave:           nil,
		TextDocumentDidClose:          nil,
		TextDocumentCompletion:        nil,
		CompletionItemResolve:         nil,
		TextDocumentHover:             info.hover,
		TextDocumentSignatureHelp:     nil,
		TextDocumentDeclaration:       nil,
		TextDocumentDefinition:        info.definition,
		TextDocumentTypeDefinition:    nil,
		TextDocumentImplementation:    nil,
		TextDocumentReferences:        nil,
		TextDocumentDocumentHighlight: info.highlight,
		TextDocumentDocumentSymbol:    nil,
		TextDocumentCodeAction:        info.codeAction,
		CodeActionResolve:             nil,
		TextDocumentCodeLens:          nil,
		CodeLensResolve:               nil,
		TextDocumentDocumentLink:      info.links,
		DocumentLinkResolve:           nil,
		TextDocumentColor:             nil,
		TextDocumentColorPresentation: nil,
		TextDocumentFormatting:        nil,
		TextDocumentRangeFormatting:   nil,
		TextDocumentOnTypeFormatting:  nil,
		TextDocumentRename:            nil,
		TextDocumentPrepareRename:     nil,
		TextDocumentFoldingRange:      nil,
		TextDocumentSelectionRange:    nil,
		//TextDocumentPrepareCallHierarchy:    nil,
		//CallHierarchyIncomingCalls:          nil,
		//CallHierarchyOutgoingCalls:          nil,
		TextDocumentLinkedEditingRange: nil,
		TextDocumentMoniker:            nil,
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
	capabilities.DocumentHighlightProvider = true
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
	d, err := gi.readURI(params.TextDocument)
	if err != nil {
		return nil, err
	}
	for _, l := range d.Links {
		if positionInRange(d.Contents, l.Range, params.Position) {
			s, err := gi.linkToURI(l)
			if err != nil {
				return nil, err
			}

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
	}
	return nil, nil
}

func (gi *graphInfo) links(ctx *glsp.Context, params *protocol.DocumentLinkParams) ([]protocol.DocumentLink, error) {
	d, err := gi.readURI(params.TextDocument)
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
	d, err := gi.readURI(params.TextDocument)
	if err != nil {
		return nil, err
	}
	for _, l := range d.Links {
		if positionInRange(d.Contents, l.Range, params.Position) {
			switch l.Type {
			case document.Wiki, document.Tag, document.Prop:
				hoverDoc, err := gi.linkToDocument(l)
				if err != nil {
					gi.logger.Info("could not find file", slog.String("uri", params.TextDocument.URI))
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
				return &protocol.Hover{Contents: response.Content, Range: &l.Range}, nil
			}
			return nil, nil
		}
	}
	return nil, nil
}

func (gi *graphInfo) highlight(context *glsp.Context, params *protocol.DocumentHighlightParams) ([]protocol.DocumentHighlight, error) {
	d, err := gi.readURI(params.TextDocument)
	if err != nil {
		return nil, err
	}
	var highlights []protocol.DocumentHighlight
	var primaryLink string
	for _, link := range d.Links {
		if positionInRange(d.Contents, link.Range, params.Position) {
			kindText := protocol.DocumentHighlightKindText
			primaryLink = link.Target
			highlights = append(highlights, protocol.DocumentHighlight{
				Range: link.Range,
				Kind:  &kindText,
			})
		}
	}
	for _, link := range d.Links {
		if link.Target == primaryLink && !positionInRange(d.Contents, link.Range, params.Position) {
			kindText := protocol.DocumentHighlightKindText
			primaryLink = link.Target
			highlights = append(highlights, protocol.DocumentHighlight{
				Range: link.Range,
				Kind:  &kindText,
			})
		}
	}
	return highlights, nil
}

func (gi *graphInfo) readURI(td protocol.TextDocumentIdentifier) (document.Document, error) {
	readCloser, err := files.URIToReader(td.URI)
	if err != nil {
		return document.Document{}, err
	}
	defer readCloser.Close()
	d, err := document.New(gi.logger, readCloser)
	if err != nil {
		return document.Document{}, err
	}
	return d, nil
}

func (gi *graphInfo) linkToURI(l document.Link) (*protocol.DocumentUri, error) {
	switch l.Type {
	case document.Wiki, document.Tag, document.Prop:
		if l.Target == "" {
			return nil, nil
		}
		page, err := gi.client.GetPageByName(l.Target)
		if err != nil {
			return nil, err
		}
		uri := page.ToURI(gi.path, gi.journalsPath, gi.pagesPath)
		return &uri, nil
	case document.BlockEmbed:
		block, err := gi.client.GetBlock(l.Target)
		if err != nil {
			gi.logger.Error("error in linkToUri", fmt.Errorf("error type mismatch getBlock: %v", block), slog.Any("link", l))
			return nil, fmt.Errorf("error calling getBlock: %w", err)
		}
		page, err := gi.client.GetPageById(block.Page.ID)
		if err != nil {
			gi.logger.Error("error in linkToUri", fmt.Errorf("error calling getPage: %w", err))
			return nil, fmt.Errorf("error calling getPage: %w", err)
		}
		uri := page.ToURI(gi.path, gi.journalsPath, gi.pagesPath)
		return &uri, nil
	default:
		gi.logger.Error("error in linkToUri", fmt.Errorf("unsupported link type: %s", l.Type))
		return nil, fmt.Errorf("unsupported link type: %s", l.Type)
	}
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
	doc, err := document.New(gi.logger, file)
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
