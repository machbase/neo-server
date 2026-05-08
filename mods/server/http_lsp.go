package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	base "github.com/machbase/neo-server/v8/mods/lsp"
	lspjsh "github.com/machbase/neo-server/v8/mods/lsp/jsh"
	lsptql "github.com/machbase/neo-server/v8/mods/lsp/tql"

	"github.com/gin-gonic/gin"
)

type lspResponse struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
	Data    any    `json:"data,omitempty"`
}

type lspDocumentRequest struct {
	Language string        `json:"language"`
	URI      string        `json:"uri"`
	Text     string        `json:"text"`
	Position base.Position `json:"position"`
}

func (svr *httpd) handleLspDiagnostics(ctx *gin.Context) {
	rsp := &lspResponse{Success: false, Reason: "not specified"}
	tick := time.Now()
	req, svc, ok := svr.bindLspRequest(ctx, rsp, tick)
	if !ok {
		return
	}
	diagnostics, err := svc.Diagnostics(ctx.Request.Context(), req.document())
	if err != nil {
		svr.writeLspError(ctx, rsp, tick, http.StatusInternalServerError, err)
		return
	}
	svr.writeLspSuccess(ctx, rsp, tick, map[string]any{"diagnostics": diagnostics})
}

func (svr *httpd) handleLspCompletion(ctx *gin.Context) {
	rsp := &lspResponse{Success: false, Reason: "not specified"}
	tick := time.Now()
	req, svc, ok := svr.bindLspRequest(ctx, rsp, tick)
	if !ok {
		return
	}
	items, err := svc.Completion(ctx.Request.Context(), req.document(), req.Position)
	if err != nil {
		svr.writeLspError(ctx, rsp, tick, http.StatusInternalServerError, err)
		return
	}
	svr.writeLspSuccess(ctx, rsp, tick, map[string]any{"items": items})
}

func (svr *httpd) handleLspHover(ctx *gin.Context) {
	rsp := &lspResponse{Success: false, Reason: "not specified"}
	tick := time.Now()
	req, svc, ok := svr.bindLspRequest(ctx, rsp, tick)
	if !ok {
		return
	}
	hover, err := svc.Hover(ctx.Request.Context(), req.document(), req.Position)
	if err != nil {
		svr.writeLspError(ctx, rsp, tick, http.StatusInternalServerError, err)
		return
	}
	svr.writeLspSuccess(ctx, rsp, tick, map[string]any{"hover": hover})
}

func (svr *httpd) bindLspRequest(ctx *gin.Context, rsp *lspResponse, tick time.Time) (*lspDocumentRequest, base.LanguageService, bool) {
	req := &lspDocumentRequest{}
	if err := ctx.ShouldBindJSON(req); err != nil {
		svr.writeLspError(ctx, rsp, tick, http.StatusBadRequest, err)
		return nil, nil, false
	}
	svc, err := lspLanguageService(req.Language)
	if err != nil {
		svr.writeLspError(ctx, rsp, tick, http.StatusBadRequest, err)
		return nil, nil, false
	}
	return req, svc, true
}

func (req *lspDocumentRequest) document() base.Document {
	return base.Document{
		URI:      req.URI,
		Language: base.Language(strings.ToLower(req.Language)),
		Text:     req.Text,
	}
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

func (svr *httpd) writeLspSuccess(ctx *gin.Context, rsp *lspResponse, tick time.Time, data any) {
	rsp.Success = true
	rsp.Reason = "success"
	rsp.Data = data
	rsp.Elapse = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) writeLspError(ctx *gin.Context, rsp *lspResponse, tick time.Time, status int, err error) {
	rsp.Success = false
	rsp.Reason = err.Error()
	rsp.Elapse = time.Since(tick).String()
	ctx.JSON(status, rsp)
}
