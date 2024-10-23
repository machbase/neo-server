package mdconv_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/machbase/neo-server/mods/util/mdconv"
	"github.com/stretchr/testify/require"
)

func TestMdCon(t *testing.T) {
	src := `# Test	
	Content`
	expect := `<h1>Test</h1>
<pre><code>Content</code></pre>
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
