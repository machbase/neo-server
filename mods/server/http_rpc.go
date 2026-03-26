package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/machbase/neo-server/v8/mods/util/mdconv"
)

var (
	ginContextType = reflect.TypeOf((*gin.Context)(nil))
	contextType    = reflect.TypeOf((*context.Context)(nil)).Elem()
	webConsoleType = reflect.TypeOf((*WebConsole)(nil))
)

type rpcImplicitParamResolver func(paramType reflect.Type) (reflect.Value, bool)

func buildRpcCallParams(handler any, rawParams []any, resolveImplicit rpcImplicitParamResolver) ([]reflect.Value, error) {
	handlerType := reflect.TypeOf(handler)
	params := make([]reflect.Value, 0, handlerType.NumIn())
	explicitIndex := 0

	for i := 0; i < handlerType.NumIn(); i++ {
		paramType := handlerType.In(i)
		if resolveImplicit != nil {
			if implicitValue, ok := resolveImplicit(paramType); ok {
				params = append(params, implicitValue)
				continue
			}
		}

		if explicitIndex >= len(rawParams) {
			params = append(params, reflect.Zero(paramType))
			continue
		}

		paramValue, err := convertRpcParam(rawParams[explicitIndex], paramType)
		if err != nil {
			return nil, fmt.Errorf("param %d: %w", explicitIndex, err)
		}
		params = append(params, paramValue)
		explicitIndex++
	}

	return params, nil
}

func convertRpcParam(raw any, targetType reflect.Type) (reflect.Value, error) {
	if raw == nil {
		return reflect.Zero(targetType), nil
	}

	rawValue := reflect.ValueOf(raw)
	if rawValue.IsValid() && rawValue.Type().AssignableTo(targetType) {
		return rawValue, nil
	}

	encoded, err := json.Marshal(raw)
	if err != nil {
		return reflect.Value{}, fmt.Errorf("marshal param: %w", err)
	}

	if targetType.Kind() == reflect.Pointer {
		targetValue := reflect.New(targetType.Elem())
		if err := json.Unmarshal(encoded, targetValue.Interface()); err != nil {
			return reflect.Value{}, fmt.Errorf("unmarshal to %s: %w", targetType, err)
		}
		return targetValue, nil
	}

	targetValue := reflect.New(targetType)
	if err := json.Unmarshal(encoded, targetValue.Interface()); err != nil {
		return reflect.Value{}, fmt.Errorf("unmarshal to %s: %w", targetType, err)
	}
	return targetValue.Elem(), nil
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
		params, bindErr := buildRpcCallParams(handler, req.Params, func(paramType reflect.Type) (reflect.Value, bool) {
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
		if bindErr != nil {
			rsp["error"] = map[string]any{
				"code":    -32602,
				"message": bindErr.Error(),
			}
			ctx.JSON(http.StatusOK, rsp)
			return
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

func RegisterJsonRpcHandlers(s *Server) {
	RegisterJsonRpcHandler("markdownRender", rpcMarkdownRender)
	if s == nil {
		return
	}
	RegisterJsonRpcHandler("getServerInfo", s.getServerInfo)
	RegisterJsonRpcHandler("getServicePorts", s.getServicePorts)
	RegisterJsonRpcHandler("listShells", s.listShells)
	RegisterJsonRpcHandler("addShell", s.addShell)
	RegisterJsonRpcHandler("deleteShell", s.deleteShell)
	RegisterJsonRpcHandler("listBridges", s.listBridges)
	RegisterJsonRpcHandler("getBridge", s.getBridge)
	RegisterJsonRpcHandler("addBridge", s.addBridge)
	RegisterJsonRpcHandler("deleteBridge", s.deleteBridge)
	RegisterJsonRpcHandler("testBridge", s.testBridge)
	RegisterJsonRpcHandler("statsBridge", s.statsBridge)
	RegisterJsonRpcHandler("execBridge", s.execBridge)
	RegisterJsonRpcHandler("queryBridge", s.queryBridge)
	RegisterJsonRpcHandler("fetchResultBridge", s.fetchResultBridge)
	RegisterJsonRpcHandler("closeResultBridge", s.closeResultBridge)
	RegisterJsonRpcHandler("listSSHKeys", s.listSSHKeys)
	RegisterJsonRpcHandler("addSSHKey", s.addSSHKey)
	RegisterJsonRpcHandler("deleteSSHKey", s.deleteSSHKey)
	RegisterJsonRpcHandler("listKeys", s.listKeys)
	RegisterJsonRpcHandler("genKey", s.genKey)
	RegisterJsonRpcHandler("deleteKey", s.deleteKey)
	RegisterJsonRpcHandler("getServerCertificate", s.getServerCertificate)
	RegisterJsonRpcHandler("listSchedules", s.listSchedules)
	RegisterJsonRpcHandler("addTimerSchedule", s.addTimerSchedule)
	RegisterJsonRpcHandler("addSubscriberSchedule", s.addSubscriberSchedule)
	RegisterJsonRpcHandler("deleteSchedule", s.deleteSchedule)
	RegisterJsonRpcHandler("startSchedule", s.startSchedule)
	RegisterJsonRpcHandler("stopSchedule", s.stopSchedule)
	RegisterJsonRpcHandler("shutdownServer", s.Shutdown)
	RegisterJsonRpcHandler("setHttpDebug", s.setHttpDebug)
	RegisterJsonRpcHandler("listSessions", s.listSessions)
	RegisterJsonRpcHandler("killSession", s.killSession)
	RegisterJsonRpcHandler("statSession", s.statSession)
	RegisterJsonRpcHandler("getSessionLimit", s.getSessionLimit)
	RegisterJsonRpcHandler("setSessionLimit", s.setSessionLimit)
	RegisterJsonRpcHandler("splitSqlStatements", s.splitSqlStatements)
}
