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

func TestTQL(t *testing.T) {
	w := httptest.NewRecorder()
	s, ctx, engine := NewMockServer(w)
	err := s.Login("sys", "manager")
	require.Nil(t, err)
	defer s.Shutdown()

	reader := bytes.NewBufferString(`
		FAKE(linspace(0,1,2))
		CSV()
	`)
	ctx.Request, _ = http.NewRequest(http.MethodPost, "/web/api/tql", reader)
	ctx.Request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	engine.HandleContext(ctx)
	require.Equal(t, 200, w.Result().StatusCode)
	require.Equal(t, strings.Join([]string{"0", "1", ""}, "\n"), w.Body.String())
}

func TestTQLWrongSyntax(t *testing.T) {
	w := httptest.NewRecorder()
	s, ctx, engine := NewMockServer(w)
	err := s.Login("sys", "manager")
	require.Nil(t, err)
	defer s.Shutdown()

	reader := bytes.NewBufferString(`
		FAKE(linspace(0,1,2))
		MAPKEY(-1,-1) // intended syntax error
		CSV()
	`)
	ctx.Request, _ = http.NewRequest(http.MethodPost, "/web/api/tql", reader)
	ctx.Request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	engine.HandleContext(ctx)
	require.Equal(t, 200, w.Result().StatusCode)
	expect := `{"success":false,"reason":"f(MAPKEY) invalid number of args;`
	require.Equal(t, expect, w.Body.String()[0:len(expect)])
}
