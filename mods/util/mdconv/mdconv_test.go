package mdconv_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/mods/util/mdconv"
	"github.com/stretchr/testify/require"
)

func TestMdCon(t *testing.T) {
	src := `# Test	
	Content`
	expect := `<h1>Test</h1>
<pre><code>Content
</code></pre>
`
	w := &bytes.Buffer{}
	conv := mdconv.New(mdconv.WithDarkMode(true))
	err := conv.ConvertString(src, w)
	require.Nil(t, err)
	require.Equal(t, expect, w.String())
}

func TestMdWithImage(t *testing.T) {
	code := []string{
		`# Image includes`,
		`![sample](./sample_image.png)`,
	}
	expect := []string{
		`<h1>Image includes</h1>`,
		`<p><img src="./sample_image.png" alt="sample" /></p>`,
	}

	w := &bytes.Buffer{}
	conv := mdconv.New(mdconv.WithDarkMode(true))
	err := conv.ConvertString(strings.Join(code, "\n"), w)
	require.Nil(t, err)
	require.Equal(t, strings.Join(expect, "\n"), strings.TrimSpace(w.String()))
}

func TestMdWithMermaid(t *testing.T) {
	code := []string{
		`# Mermaid test`,
		"```mermaid",
		`graph TD;`,
		`A-->B;`,
		"```",
	}
	expect := []string{
		`<h1>Mermaid test</h1>`,
		`<pre class="mermaid">graph TD;`,
		`A--&gt;B;`,
		`</pre>`,
	}

	w := &bytes.Buffer{}
	conv := mdconv.New(mdconv.WithDarkMode(true))
	err := conv.ConvertString(strings.Join(code, "\n"), w)
	require.Nil(t, err)
	require.Equal(t, strings.Join(expect, "\n"), strings.TrimSpace(w.String()))
}

func TestMdWithJshCodeFence(t *testing.T) {
	code := []string{
		`# JSH Code Example`,
		"```jsh",
		`const result = db.query('select * from table');`,
		"```",
	}

	w := &bytes.Buffer{}
	conv := mdconv.New(mdconv.WithDarkMode(true))
	err := conv.ConvertString(strings.Join(code, "\n"), w)
	require.Nil(t, err)

	result := w.String()
	// Verify that jsh code fence is converted to javascript
	// The HTML should not contain ">jsh<" which would appear if jsh was used as the language
	require.NotContains(t, result, ">jsh<")
	// Should have syntax highlighting applied (const keyword in different color)
	require.Contains(t, result, "<span style=")
}

func TestMdWithJshRunCodeFence(t *testing.T) {
	code := []string{
		`# JSH-RUN Code Example`,
		"```jsh-run",
		`print('Hello from JSH');`,
		"```",
	}

	w := &bytes.Buffer{}
	conv := mdconv.New(mdconv.WithDarkMode(true))
	err := conv.ConvertString(strings.Join(code, "\n"), w)
	require.Nil(t, err)

	result := w.String()
	// Verify that jsh-run code fence is converted to javascript
	// The HTML should not contain ">jsh-run<" which would appear if jsh-run was used as the language
	require.NotContains(t, result, ">jsh-run<")
	// Should have syntax highlighting applied
	require.Contains(t, result, "<span style=")
}
