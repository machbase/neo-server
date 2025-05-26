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
			Templates: []string{`<li>{{.Num}}: {{ .Value 0 }} {{ .Value 1 }}`},
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
			Templates: []string{`{{.Num}},{{ .Value 0 }},{{ .Value 1 }}`},
			Format:    templ.TEXT,
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
			Templates: []string{`<script>
function test() {
	return {{ .Value 0 }}+{{ .Value 1 }};
}
</script>`},
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
			Templates: []string{`{{ if .IsFirst }}-head-{{end}}
<li>{{.Num}}: {{ .Value 0 }} {{ .Value 1 }}
{{ if .IsLast }}-tail-{{end}}`},
			Expects: []string{
				"-head-\n",
				"<li>1: Hello World!\n\n",
				"<li>2: 3.14 true\n",
				"-tail-",
			},
		},
		{
			Name: "columns",
			Args: [][]any{
				{"A", 1.23, true},
				{"B", 4.56, false},
				{"C", 7.89, true},
			},
			Columns:   []string{"col1", "col2", "col3"},
			Templates: []string{`{{- .Num}}: {{ .V.col1 }} {{ .V.col2 }} {{ .V.col3 }}{{ "\n" -}}`},
			Expects: []string{
				"1: A 1.23 true\n",
				"2: B 4.56 false\n",
				"3: C 7.89 true\n",
			},
		},
		{
			Name: "values",
			Args: [][]any{
				{"A", 1.23, true},
				{"B", 4.56, false},
				{"C", 7.89, true},
			},
			Columns:   []string{"col1", "col2", "col3"},
			Templates: []string{`{{- .Num}}: {{ index .Values 0 }} {{ index .Values 1  }} {{ index .Values 2 }}{{ "\n" -}}`},
			Expects: []string{
				"1: A 1.23 true\n",
				"2: B 4.56 false\n",
				"3: C 7.89 true\n",
			},
		},
		{
			Name: "template_files",
			Args: [][]any{
				{"A", 1.23, true},
				{"B", 4.56, false},
				{"C", 7.89, true},
			},
			Columns: []string{"col1", "col2", "col3"},
			Templates: []string{
				`{{.Num}}: {{ template "item" .V }}{{"\n"}}`,
				`{{ define "item" -}} {{ .col1 }} {{ .col2  }} {{ .col3 }} {{- end}}`,
			},
			Expects: []string{
				"1: A 1.23 true\n",
				"2: B 4.56 false\n",
				"3: C 7.89 true\n",
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
	Name      string
	Args      [][]any
	Columns   []string
	Templates []string
	Format    templ.Format
	Expects   []string
}

func runTestCase(t *testing.T, testCase TestCase) {
	t.Helper()
	var enc *templ.Exporter
	if testCase.Format == templ.TEXT {
		enc = templ.NewEncoder(templ.TEXT)
		require.Equal(t, "text/plain", enc.ContentType())
	} else {
		enc = templ.NewEncoder(templ.HTML)
		require.Equal(t, "application/xhtml+xml", enc.ContentType())
	}

	w := &bytes.Buffer{}
	enc.SetOutputStream(w)
	enc.SetTemplate(testCase.Templates...)
	if len(testCase.Columns) > 0 {
		enc.SetColumns(testCase.Columns...)
	} else {
		cols := make([]string, len(testCase.Args[0]))
		for i := 0; i < len(testCase.Args[0]); i++ {
			cols = append(cols, fmt.Sprintf("column%d", i))
		}
		enc.SetColumns(cols...)
	}
	err := enc.Open()
	require.Nil(t, err)

	for _, row := range testCase.Args {
		err = enc.AddRow(row)
		if err != nil {
			t.Fatalf("Error adding row: %v", err)
		}
	}
	enc.Close()

	expects := testCase.Expects
	require.Equal(t, strings.Join(expects, ""), w.String())
	fmt.Println()
}
