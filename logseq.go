package main

import (
	"os"
	"path"

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
	logger.Info("code action fired", slog.String("uri", fileName), slog.Any("position", params.Position))

	response, err := lsClient.R().SetBody(map[string]any{
		"method": "logseq.Editor.getPageBlocksTree",
		"args":   []string{fileName},
	}).Post("/api")
	if err != nil {
		logger.Info("failed to query ls api for file", err)
	}
	logger.Info("resp", slog.Any("resp", string(response.Body())))
	return nil, nil
}
