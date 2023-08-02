package tql

import (
	"context"
	"strings"
	"testing"

	"github.com/d5/tengo/v2/require"
	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/tql/fx"
)

func TestNewContextChain(t *testing.T) {
	task := fx.NewTaskContext(context.TODO())
	strExprs := []string{
		"PUSHKEY('tt')",
		"FFT()",
	}
	exprs := make([]*expression.Expression, len(strExprs))
	for i, str := range strExprs {
		exprs[i], _ = ParseMap(task, str)
		require.NotNil(t, exprs[i], str)
	}
	chain, err := newExecutionChain(task, nil, nil, nil, exprs)
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
		"FAKE( oscillator( range(timeAdd(1685714509*1000000000,'1s'), '1s', '100us'), freq(10, 1.0), freq(50, 2.0)))",
		"PUSHKEY('samples')",
		"GROUPBYKEY()",
		"FFT(minHz(0), maxHz(60))",
		"POPKEY()",
		"CSV()",
	}
	reader := strings.NewReader(strings.Join(strExprs, "\n"))
	output, _ := stream.NewOutputStream("-")

	task := fx.NewTaskContext(context.TODO())
	task.SetOutputStream(output)
	tq, err := Parse(task, reader)
	require.Nil(t, err)
	require.NotNil(t, tq)

	tq.Execute(task, nil)
}
