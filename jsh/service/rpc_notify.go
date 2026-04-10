package service

import "context"

type JsonRpcNotificationWriter interface {
	NotifyJsonRpc(session string, payload map[string]any) error
}

type jsonRpcNotifyWriterKey struct{}
type jsonRpcNotifySessionKey struct{}

func WithJsonRpcNotificationWriter(ctx context.Context, writer JsonRpcNotificationWriter) context.Context {
	if writer == nil {
		return ctx
	}
	return context.WithValue(ctx, jsonRpcNotifyWriterKey{}, writer)
}

func WithJsonRpcSession(ctx context.Context, session string) context.Context {
	if session == "" {
		return ctx
	}
	return context.WithValue(ctx, jsonRpcNotifySessionKey{}, session)
}

func emitJsonRpcNotification(ctx context.Context, method string, params map[string]any) bool {
	if ctx == nil || method == "" {
		return false
	}
	writer, ok := ctx.Value(jsonRpcNotifyWriterKey{}).(JsonRpcNotificationWriter)
	if !ok || writer == nil {
		return false
	}
	session, _ := ctx.Value(jsonRpcNotifySessionKey{}).(string)
	if err := writer.NotifyJsonRpc(session, map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}); err != nil {
		return false
	}
	return true
}
