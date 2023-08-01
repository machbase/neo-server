package maps

import (
	"github.com/machbase/neo-server/mods/tql/context"
)

func Take(ctx *context.Context, key any, value any, limit int) *context.Param {
	if ctx.Nrow > limit {
		return context.ExecutionCircuitBreak
	}
	return &context.Param{K: key, V: value}
}

func Drop(ctx *context.Context, key any, value any, limit int) *context.Param {
	if ctx.Nrow <= limit {
		return nil
	}
	return &context.Param{K: key, V: value}
}
