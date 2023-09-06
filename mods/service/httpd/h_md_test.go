package httpd

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarkdown(t *testing.T) {
	w := httptest.NewRecorder()
	s, ctx, engine := NewMockServer(w)
	defer s.Shutdown()

	err := s.Login("sys", "manager")
	require.Nil(t, err)

	reader := bytes.NewBufferString(`
## markdown test
- file_root {{ file_root }}
- file_path {{ file_path }}
- file_name {{ file_name }}
- file_dir {{ file_dir }}
`)
	expect := []string{
		"<div><h2>markdown test</h2>",
		"<ul>",
		"<li>file_root /web/api/tql</li>",
		"<li>file_path /web/api/tql/sample/file.wrk</li>",
		"<li>file_name file.wrk</li>",
		"<li>file_dir /web/api/tql/sample</li>",
		"</ul>",
		"</div>",
	}
	ctx.Request, _ = http.NewRequest(http.MethodPost, "/web/api/md", reader)
	ctx.Request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	ctx.Request.Header.Set("X-Referer", "http://127.0.0.1:5654/web/api/tql/sample/file.wrk")
	engine.HandleContext(ctx)
	require.Equal(t, 200, w.Result().StatusCode)
	require.Equal(t, "application/xhtml+xml", w.Header().Get("Content-Type"))
	require.Equal(t, strings.Join(expect, "\n"), w.Body.String())
}
