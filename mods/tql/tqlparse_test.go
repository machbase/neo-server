package tql

import (
	"testing"

	"github.com/d5/tengo/v2/require"
	"github.com/machbase/neo-server/mods/expression"
)

func TestTQL(t *testing.T) {
	funcMap := map[string]expression.Function{
		"INPUT":  nil,
		"range":  nil,
		"OUTPUT": nil,
		"CSV":    nil,
		"DOTASK": nil,
	}
	// single line
	input := `INPUT('value')`
	toks, err := expression.ParseTokens(input, funcMap)
	require.Nil(t, err)
	require.NotNil(t, toks)

	input = `INPUT('value'`
	_, err = expression.ParseTokens(input, funcMap)
	require.NotNil(t, err)
	require.Equal(t, "unbalanced parenthesis", err.Error())

	// multi lines
	input = `
INPUT( 'value',
range(10)
)
DOTASK()
DOTASK(
)
OUTPUT(
	CSV()
)`
	toks, err = expression.ParseTokens(input, funcMap)
	require.Nil(t, err)
	require.NotNil(t, toks)
	t.Logf("%v", toks)
}
