package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/machbase/neo-server/v8/jsh/service"
	base "github.com/machbase/neo-server/v8/mods/lsp"
	lspjsh "github.com/machbase/neo-server/v8/mods/lsp/jsh"
	lsptql "github.com/machbase/neo-server/v8/mods/lsp/tql"
)

type lspDocumentRequest struct {
	Language string        `json:"language"`
	URI      string        `json:"uri"`
	Text     string        `json:"text"`
	Position base.Position `json:"position"`
}

type lspMetadataRequest struct {
	Language string `json:"language"`
}

func (req *lspDocumentRequest) document() base.Document {
	return base.Document{
		URI:      req.URI,
		Language: base.Language(strings.ToLower(req.Language)),
		Text:     req.Text,
	}
}

func rpcLspDiagnostics(ctx context.Context, req lspDocumentRequest) (map[string]any, error) {
	svc, err := lspLanguageService(req.Language)
	if err != nil {
		return nil, lspRpcInvalidParams(err)
	}
	diagnostics, err := lspDiagnostics(ctx, svc, req)
	if err != nil {
		return nil, err
	}
	return map[string]any{"diagnostics": diagnostics}, nil
}

func rpcLspCompletion(ctx context.Context, req lspDocumentRequest) (map[string]any, error) {
	svc, err := lspLanguageService(req.Language)
	if err != nil {
		return nil, lspRpcInvalidParams(err)
	}
	items, err := lspCompletion(ctx, svc, req)
	if err != nil {
		return nil, err
	}
	return map[string]any{"items": items}, nil
}

func rpcLspHover(ctx context.Context, req lspDocumentRequest) (map[string]any, error) {
	svc, err := lspLanguageService(req.Language)
	if err != nil {
		return nil, lspRpcInvalidParams(err)
	}
	hover, err := lspHover(ctx, svc, req)
	if err != nil {
		return nil, err
	}
	return map[string]any{"hover": hover}, nil
}

func rpcLspSignatureHelp(ctx context.Context, req lspDocumentRequest) (map[string]any, error) {
	svc, err := lspLanguageService(req.Language)
	if err != nil {
		return nil, lspRpcInvalidParams(err)
	}
	help, err := lspSignatureHelp(ctx, svc, req)
	if err != nil {
		return nil, err
	}
	return map[string]any{"signatureHelp": help}, nil
}

func rpcLspMetadata(req lspMetadataRequest) (map[string]any, error) {
	metadata, err := lspMetadata(req.Language)
	if err != nil {
		return nil, lspRpcInvalidParams(err)
	}
	return map[string]any{"metadata": metadata}, nil
}

func lspDiagnostics(ctx context.Context, svc base.LanguageService, req lspDocumentRequest) ([]base.Diagnostic, error) {
	return svc.Diagnostics(ctx, req.document())
}

func lspCompletion(ctx context.Context, svc base.LanguageService, req lspDocumentRequest) ([]base.CompletionItem, error) {
	return svc.Completion(ctx, req.document(), req.Position)
}

func lspHover(ctx context.Context, svc base.LanguageService, req lspDocumentRequest) (*base.Hover, error) {
	return svc.Hover(ctx, req.document(), req.Position)
}

func lspSignatureHelp(ctx context.Context, svc base.LanguageService, req lspDocumentRequest) (*base.SignatureHelp, error) {
	return svc.SignatureHelp(ctx, req.document(), req.Position)
}

func lspRpcInvalidParams(err error) error {
	return &service.JsonRpcError{Code: -32602, Message: err.Error()}
}

func lspLanguageService(language string) (base.LanguageService, error) {
	switch base.Language(strings.ToLower(language)) {
	case base.LanguageTQL:
		return lsptql.NewService(), nil
	case base.LanguageJSH:
		return lspjsh.NewService(), nil
	case base.LanguageSQL:
		return nil, fmt.Errorf("%s language service is not implemented yet", language)
	default:
		return nil, fmt.Errorf("unsupported language %q", language)
	}
}

func lspMetadata(language string) (base.Metadata, error) {
	switch base.Language(strings.ToLower(language)) {
	case base.LanguageTQL:
		return lsptql.BuildMetadata(), nil
	case base.LanguageJSH:
		return lspjsh.BuildMetadata(), nil
	case base.LanguageSQL:
		return base.Metadata{}, fmt.Errorf("%s metadata is not implemented yet", language)
	default:
		return base.Metadata{}, fmt.Errorf("unsupported language %q", language)
	}
}
