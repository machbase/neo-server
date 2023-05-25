package tagql

import (
	"context"
	"testing"

	"github.com/d5/tengo/v2/require"
	"github.com/machbase/neo-server/mods/expression"
)

func TestNewContextChain(t *testing.T) {
	exprMerge, err := expression.NewWithFunctions("MERGE(K, V, 'tt')", mapFunctions)
	require.Nil(t, err)
	exprFFT, err := expression.NewWithFunctions("FFT(K, V)", mapFunctions)
	require.Nil(t, err)
	exprs := []*expression.Expression{
		exprMerge,
		exprFFT,
	}
	var R chan any = nil

	chain := NewContextChain(context.TODO(), exprs, R)
	require.NotNil(t, chain)
	require.Equal(t, 2, len(chain))
	require.NotNil(t, chain[0].Next)
	require.NotNil(t, chain[1])
	require.Nil(t, chain[1].Next)
	require.True(t, exprMerge == chain[0].Expr)
	require.True(t, exprFFT == chain[1].Expr)
	require.True(t, chain[1] == chain[0].Next)
}
