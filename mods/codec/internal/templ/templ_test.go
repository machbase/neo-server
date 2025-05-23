package templ_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/mods/codec/internal/templ"
	"github.com/stretchr/testify/require"
)

func TestTemplEncoder(t *testing.T) {
	tests := []TestCase{
		{
			Name: "hello_world_html",
			Args: [][]any{
				{"Hello", "World!"},
				{3.14, true},
			},
			Template: `<li>{{.ROWNUM}}: {{ (index .Values 0) }} {{ (index .Values 1) }}`,
			Expects: []string{
				"<li>1: Hello World!",
				"<li>2: 3.14 true",
			},
		},
		{
			Name: "hello_world_text",
			Args: [][]any{
				{"Hello", "World!"},
				{3.14, true},
			},
			Template: `{{.ROWNUM}},{{ (index .Values 0) }},{{ (index .Values 1) }}`,
			Format:   templ.TEXT,
			Expects: []string{
				"1,Hello,World!",
				"2,3.14,true",
			},
		},
		{
			Name: "script",
			Args: [][]any{
				{"Hello", []float64{1, 2.3, 3.14}},
			},
			Template: `<script>
function test() {
	return {{ (index .Values 0) }}+{{ (index .Values 1) }};
}
</script>`,
			Expects: []string{
				"<script>\n",
				"function test() {\n",
				"\treturn \"Hello\"+[1,2.3,3.14];\n",
				"}\n",
				"</script>",
			},
		},
		{
			Name: "first_last",
			Args: [][]any{
				{"Hello", "World!"},
				{3.14, true},
			},
			Template: `{{ if .IsFirst }}-head-{{end}}
<li>{{.ROWNUM}}: {{ (index .Values 0) }} {{ (index .Values 1) }}
{{ if .IsLast }}-tail-{{end}}`,
			Expects: []string{
				"-head-\n",
				"<li>1: Hello World!\n\n",
				"<li>2: 3.14 true\n",
				"-tail-",
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.Name, func(t *testing.T) {
			runTestCase(t, testCase)
		})
	}
}

type TestCase struct {
	Name     string
	Args     [][]any
	Template string
	Format   templ.Format
	Expects  []string
}

func runTestCase(t *testing.T, testCase TestCase) {
	t.Helper()
	var enc *templ.Exporter
	if testCase.Format == templ.TEXT {
		enc = templ.NewEncoder(templ.TEXT)
	} else {
		enc = templ.NewEncoder(templ.HTML)
	}
	require.Equal(t, "application/xhtml+xml", enc.ContentType())

	w := &bytes.Buffer{}
	enc.SetOutputStream(w)
	enc.SetTemplate(testCase.Template)
	err := enc.Open()
	require.Nil(t, err)

	for _, row := range testCase.Args {
		err = enc.AddRow(row)
		require.Nil(t, err)
	}
	enc.Close()

	expects := testCase.Expects
	require.Equal(t, strings.Join(expects, ""), w.String())
	fmt.Println()
}
