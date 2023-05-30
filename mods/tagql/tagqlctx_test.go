package tagql

import (
	"context"
	"testing"

	"github.com/d5/tengo/v2/require"
)

func TestNewContextChain(t *testing.T) {
	exprs := []string{
		"PUSHKEY('tt')",
		"FFT()",
	}
	chain, err := newExecutionChain(context.TODO(), exprs, nil)
	require.Nil(t, err)
	require.NotNil(t, chain)
	require.Equal(t, 2, len(chain.nodes))
	require.NotNil(t, chain.nodes[0].Next)
	require.NotNil(t, chain.nodes[1])
	require.Nil(t, chain.nodes[1].Next)
	require.Equal(t, "PUSHKEY(CTX,K,V,'tt')", chain.nodes[0].Expr.String())
	require.Equal(t, "FFT(CTX,K,V)", chain.nodes[1].Expr.String())
	require.True(t, chain.nodes[1] == chain.nodes[0].Next)
	chain.Stop()
}
