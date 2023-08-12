package mdconv_test

import (
	"bytes"
	"testing"

	"github.com/d5/tengo/v2/require"
	"github.com/machbase/neo-server/mods/util/mdconv"
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
