package server

import (
	"context"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/v8/jsh/service"
	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/machbase/neo-server/v8/mods/util/mdconv"
)

var (
	ginContextType = reflect.TypeOf((*gin.Context)(nil))
	contextType    = reflect.TypeOf((*context.Context)(nil)).Elem()
	webConsoleType = reflect.TypeOf((*WebConsole)(nil))
)

type rpcImplicitParamResolver func(paramType reflect.Type) (reflect.Value, bool)

var defaultJsonRpcController = &service.Controller{}

func buildRpcCallParams(handler any, rawParams []any, resolveImplicit rpcImplicitParamResolver) ([]reflect.Value, error) {
	return service.BuildRpcCallParams(handler, rawParams, service.JsonRpcImplicitParamResolver(resolveImplicit))
}

func rpcMarkdownRender(markdown string, darkMode bool) (string, error) {
	w := &strings.Builder{}
	conv := mdconv.New(mdconv.WithDarkMode(darkMode))
	if err := conv.ConvertString(markdown, w); err != nil {
		return "", err
	}
	return w.String(), nil
}

// handleHttpRpc handles HTTP POST requests for JSON-RPC
func (svr *httpd) handleHttpRpc(ctx *gin.Context) {
	var req eventbus.RPC

	// Parse JSON-RPC request
	if err := ctx.ShouldBindJSON(&req); err != nil {
		// Invalid JSON-RPC request format
		rsp := map[string]any{
			"jsonrpc": "2.0",
			"id":      nil,
			"error": map[string]any{
				"code":    -32700,
				"message": "Parse error",
			},
		}
		ctx.JSON(http.StatusOK, rsp)
		return
	}

	rsp := map[string]any{
		"jsonrpc": "2.0",
		"id":      req.ID,
	}

	ctl := svr.rpcController
	if ctl == nil {
		ctl = defaultJsonRpcController
	}
	result, rpcErr := ctl.CallJsonRpc(req.Method, req.Params, func(paramType reflect.Type) (reflect.Value, bool) {
		switch {
		case paramType == ginContextType:
			return reflect.ValueOf(ctx), true
		case paramType == contextType:
			// Pass gin.Context as context.Context to preserve requester information.
			return reflect.ValueOf(ctx), true
		default:
			return reflect.Value{}, false
		}
	})
	if rpcErr == nil {
		rsp["result"] = result
	} else {
		code := rpcErr.Code
		message := rpcErr.Message
		if code == -32603 {
			code = -32000
		}
		if rpcErr.Code == -32601 {
			message = "Method not found"
		}
		rsp["error"] = map[string]any{
			"code":    code,
			"message": message,
		}
	}

	// Always return HTTP 200 as per JSON-RPC 2.0 specification
	ctx.JSON(http.StatusOK, rsp)
}
