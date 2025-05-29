package templ_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

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
			Name: "first_last_empty",
			Args: [][]any{},
			Templates: []string{`{{ if .IsFirst }}-head-{{"\n"}}{{end}}
				{{- if not .IsEmpty -}}
				<li>{{.Num}}: {{ .Value 0 }} {{ .Value 1 }}
				{{- else }}
				{{- end }}
				{{- if .IsLast }}-tail-{{end}}`},
			Expects: []string{
				"-head-\n",
				"-tail-",
			},
		},
		{
			Name: "columns",
			Args: [][]any{
				{"a", 1.23, true},
				{"b", 4.56, false},
				{"c", 7.89, true},
			},
			Columns:   []string{"col1", "col2", "col3"},
			Templates: []string{`{{- .Num}}: {{ .V.col1 | toUpper }} {{ .V.col2 }} {{ .V.col3 }}{{ "\n" -}}`},
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
			Templates: []string{`{{- .Num}}: {{ index .Values 0 | toLower }} {{ index .Values 1  }} {{ index .Values 2 }}{{ "\n" -}}`},
			Expects: []string{
				"1: a 1.23 true\n",
				"2: b 4.56 false\n",
				"3: c 7.89 true\n",
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
	} else if len(testCase.Args) > 0 {
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

func TestFormat(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		format   string
		expected string
	}{
		{
			name:     "double_full",
			value:    3.141592,
			format:   "%f",
			expected: "3.141592",
		},
		{
			name:     "double_2",
			value:    3.141592,
			format:   "%.2f",
			expected: "3.14",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			tmpl := templ.NewEncoder(templ.HTML)
			tmpl.SetOutputStream(out)
			tmpl.SetTemplate(fmt.Sprintf(`{{ .Value 0 | format "%s" }}`, tt.format))
			tmpl.Open()
			tmpl.AddRow([]any{tt.value})
			tmpl.Close()
			require.Equal(t, tt.expected, out.String())
		})
	}
}

func TestTimeformat(t *testing.T) {
	tests := []struct {
		name     string
		ts       time.Time
		format   string
		location string
		expected string
	}{
		{
			name:     "timeformat_tz_GMT",
			ts:       time.Unix(1633072800, 0), // 2021-10-01 07:20:00 UTC
			format:   "2006-01-02 15:04:05",
			location: "GMT",
			expected: "2021-10-01 07:20:00",
		},
		{
			name:     "timeformat_tz_local",
			ts:       time.Unix(1633072800, 0), // 2021-10-01 07:20:00 UTC
			format:   "2006-01-02 15:04:05",
			location: "Asia/Seoul",
			expected: "2021-10-01 16:20:00",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			tmpl := templ.NewEncoder(templ.HTML)
			tmpl.SetOutputStream(out)
			tmpl.SetTemplate(fmt.Sprintf(`{{ .Value 0 | timeformat "%s" "%s" }}`, tt.format, tt.location))
			tmpl.Open()
			tmpl.AddRow([]any{tt.ts})
			tmpl.Close()
			require.Equal(t, tt.expected, out.String())
		})
	}
}

func TestParams(t *testing.T) {
	tests := []struct {
		name     string
		values   []any
		params   map[string][]string
		template string
		expected string
	}{
		{
			name:     "params_value",
			values:   []any{3.141592},
			params:   map[string][]string{"f": {"%.2f"}},
			template: `{{ param "f" }}`,
			expected: "%.2f",
		},
		{
			name:     "format_from_param",
			values:   []any{3.141592},
			params:   map[string][]string{"f": {"%.2f"}},
			template: `{{ .Value 0 | format (param "f") }}`,
			expected: "3.14",
		},
		{
			name:     "format_from_paramDefault",
			values:   []any{3.141592},
			params:   map[string][]string{"f": {"%.2f"}},
			template: `{{ .Value 0 | format (paramDefault "x" "%.4f") }}`,
			expected: "3.1416",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			tmpl := templ.NewEncoder(templ.HTML)
			tmpl.SetOutputStream(out)
			tmpl.ExportParams(tt.params)
			tmpl.SetTemplate(tt.template)
			tmpl.Open()
			tmpl.AddRow(tt.values)
			tmpl.Close()
			require.Equal(t, tt.expected, out.String())
		})
	}
}

func TestUnsafeHTML(t *testing.T) {
	out := &bytes.Buffer{}
	tmpl := templ.NewEncoder(templ.HTML)
	tmpl.SetOutputStream(out)
	tmpl.SetTemplate(`Hello, {{ .Value 0 }}!` + "\n" +
		`<div {{ .ValueHTMLAttr 1 }} href="{{ .ValueURL 2 }}">Hello, {{ .ValueHTML 0}}!</div>`)
	tmpl.Open()
	tmpl.AddRow([]any{"<b>World</b>", `color="red"`, `http://example.com?q=123#tag`})
	tmpl.Close()
	require.Equal(t, "Hello, &lt;b&gt;World&lt;/b&gt;!"+"\n"+
		`<div color="red" href="http://example.com?q=123#tag">Hello, <b>World</b>!</div>`, out.String())
}

func TestUnsafeCSS(t *testing.T) {
	out := &bytes.Buffer{}
	tmpl := templ.NewEncoder(templ.HTML)
	tmpl.SetOutputStream(out)
	tmpl.SetTemplate(`body {{ .Value 0 }}` + "\n" +
		`body {{ .ValueCSS 0 }}`)
	tmpl.Open()
	tmpl.AddRow([]any{"{ color: red; margin: 2px; }"})
	tmpl.Close()
	require.Equal(t, "body { color: red; margin: 2px; }\nbody { color: red; margin: 2px; }", out.String())
}

func TestUnsafeJS(t *testing.T) {
	out := &bytes.Buffer{}
	tmpl := templ.NewEncoder(templ.HTML)
	tmpl.SetOutputStream(out)
	tmpl.SetTemplate(`<script>{{ .Value 0 }}` + "\n" + `{{ .ValueJS 0 }}</script>`)
	tmpl.Open()
	tmpl.AddRow([]any{`function hello() { return "Hello, World!"; }`})
	tmpl.Close()
	require.Equal(t, `<script>"function hello() { return \"Hello, World!\"; }"`+"\n"+
		`function hello() { return "Hello, World!"; }</script>`, out.String())
}
