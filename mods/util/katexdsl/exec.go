package katexdsl

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"

	"github.com/dop251/goja"
)

//go:embed katex.min.js
var code string

type Option struct {
	DisplayMode  bool              `json:"displayMode"`
	Output       string            `json:"output"` // html, mathml, htmlAndMathml
	ThrowOnError bool              `json:"throwOnError"`
	Leqno        bool              `json:"leqno,omitempty"`
	Fleqn        bool              `json:"fleqn,omitempty"`
	ErrorColor   string            `json:"errorColor,omitempty"`
	GlobalGroup  bool              `json:"globalGroup,omitempty"`
	Macros       map[string]string `json:"macros,omitempty"`
}

func Render(w io.Writer, src []byte, display bool, throwOnError bool) error {
	return RenderWithOption(w, src, Option{
		DisplayMode:  display,
		Output:       "mathml",
		ThrowOnError: throwOnError,
	})
}

func RenderOption(w io.Writer, src []byte, opt Option) error {
	return RenderWithOption(w, src, opt)
}

func RenderWithOption(w io.Writer, src []byte, opt Option) error {
	vm := goja.New()
	_, err := vm.RunString(code)
	if err != nil {
		return err
	}

	if opt.Output == "" {
		opt.Output = "mathml"
	}

	srcJSON, err := json.Marshal(string(src))
	if err != nil {
		return fmt.Errorf("marshal source: %w", err)
	}
	optJSON, err := json.Marshal(opt)
	if err != nil {
		return fmt.Errorf("marshal options: %w", err)
	}
	expr := fmt.Sprintf(`katex.renderToString(%s, %s)`, string(srcJSON), string(optJSON))

	result, err := vm.RunString(expr)
	if err != nil {
		return err
	}

	if v, ok := result.Export().(string); !ok {
		return fmt.Errorf("expected string result, got %T", result.Export())
	} else {
		_, err = io.WriteString(w, v)
	}
	return err
}
