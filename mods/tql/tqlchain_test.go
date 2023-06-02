package tql

import (
	"context"
	"testing"

	"github.com/d5/tengo/v2/require"
	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/tql/fmap"
)

func TestNewContextChain(t *testing.T) {
	strExprs := []string{
		"PUSHKEY('tt')",
		"FFT()",
	}
	exprs := make([]*expression.Expression, len(strExprs))
	for i, str := range strExprs {
		exprs[i], _ = fmap.Parse(str)
	}
	chain, err := newExecutionChain(context.TODO(), nil, nil, nil, exprs, nil)
	require.Nil(t, err)
	require.NotNil(t, chain)
	require.Equal(t, 2, len(chain.nodes))
	require.NotNil(t, chain.nodes[0].Next)
	require.NotNil(t, chain.nodes[1])
	require.Nil(t, chain.nodes[1].Next)
	require.Equal(t, "PUSHKEY(CTX,K,V,'tt')", chain.nodes[0].Expr.String())
	require.Equal(t, "FFT(CTX,K,V)", chain.nodes[1].Expr.String())
	require.True(t, chain.nodes[1] == chain.nodes[0].Next)
	chain.stop()
}
