package script_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/machbase/neo-server/v8/mods/script"
	"github.com/stretchr/testify/require"
)

func TestTengoStript(t *testing.T) {
	loader := script.NewLoader()

	s := []byte(`
		fmt := import("fmt")
		output := greeting(input)
	`)

	engine, err := loader.Parse(s)
	if err != nil {
		t.Fatal(err)
	}

	engine.SetVar("input", "world")
	engine.SetFunc("greeting", func(args ...any) (any, error) {
		if len(args) != 1 {
			return nil, errors.New("missing argument")
		}
		who := fmt.Sprintf("%v", args[0])

		return "hello " + who + "?", nil
	})

	if err = engine.Run(); err != nil {
		t.Fatal(err)
	}

	var output string
	engine.GetVar("output", &output)

	require.Equal(t, "hello world?", output)
}
