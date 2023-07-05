package tql

import (
	"context"
	"strings"
	"testing"

	"github.com/d5/tengo/v2/require"
	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/stream"
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

func TestFFTChain(t *testing.T) {
	strExprs := []string{
		"INPUT( FAKE( oscillator( range(time(1685714509*1000000000,'1s'), '1s', '100us'), freq(10, 1.0), freq(50, 2.0))))",
		"PUSHKEY('samples')",
		"GROUPBYKEY()",
		"FFT(minHz(0), maxHz(60))",
		"POPKEY()",
		"OUTPUT(CSV())",
	}
	reader := strings.NewReader(strings.Join(strExprs, "\n"))
	output, _ := stream.NewOutputStream("-")

	tq, err := Parse(reader, nil, nil, output, false)
	require.Nil(t, err)
	require.NotNil(t, tq)

	tq.Execute(context.TODO(), nil)
}
