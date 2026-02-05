package server

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/machbase/neo-server/v8/mods/util/mdconv"
)

func init() {
	RegisterJsonRpcHandler("markdownRender", rpcMarkdownRender)
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

	// Lookup handler
	wsRpcHandlersMutex.RLock()
	handler, ok := wsRpcHandlers[req.Method]
	wsRpcHandlersMutex.RUnlock()

	rsp := map[string]any{
		"jsonrpc": "2.0",
		"id":      req.ID,
	}

	if ok {
		// Reflection for the handler method signature
		// Convert req.Params to the expected types of handler function
		var params []reflect.Value
		handlerType := reflect.TypeOf(handler)
		implicitParams := 0

		for i := 0; i < handlerType.NumIn(); i++ {
			paramType := handlerType.In(i)
			var paramValue reflect.Value

			// Support implicit parameters
			if paramType.String() == "*gin.Context" {
				implicitParams++
				paramValue = reflect.ValueOf(ctx)
			} else if paramType.String() == "context.Context" {
				implicitParams++
				// passing gin.Context as context.Context
				// it is used in shutdown server rpc to identify requester info
				paramValue = reflect.ValueOf(ctx)
			} else if i-implicitParams < len(req.Params) {
				paramValue = reflect.ValueOf(req.Params[i-implicitParams])
			} else {
				paramValue = reflect.Zero(paramType)
			}
			params = append(params, paramValue)
		}

		// Call the handler
		resultValues := reflect.ValueOf(handler).Call(params)
		var result interface{}
		var err error

		if len(resultValues) > 0 {
			result = resultValues[0].Interface()
		}
		if len(resultValues) == 1 && result != nil {
			if errVal, ok := result.(error); ok {
				result = nil
				err = errVal
			}
		}
		if len(resultValues) > 1 {
			if !resultValues[1].IsNil() {
				err = resultValues[1].Interface().(error)
			}
		}

		// Send response
		if err == nil {
			rsp["result"] = result
		} else {
			rsp["error"] = map[string]any{
				"code":    -32000,
				"message": err.Error(),
			}
		}
	} else {
		rsp["error"] = map[string]any{
			"code":    -32601,
			"message": "Method not found",
		}
	}

	// Always return HTTP 200 as per JSON-RPC 2.0 specification
	ctx.JSON(http.StatusOK, rsp)
}
