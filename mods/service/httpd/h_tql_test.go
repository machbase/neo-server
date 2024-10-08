package httpd

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTQL_CSV(t *testing.T) {
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
	require.Equal(t, "text/csv; charset=utf-8", w.Header().Get("Content-Type"))
	require.Equal(t, strings.Join([]string{"0", "1", "\n"}, "\n"), w.Body.String())
}

func TestTQL_JSON(t *testing.T) {
	w := httptest.NewRecorder()
	s, ctx, engine := NewMockServer(w)
	err := s.Login("sys", "manager")
	require.Nil(t, err)
	defer s.Shutdown()

	reader := bytes.NewBufferString(`
		FAKE(linspace(0,1,2))
		JSON()
	`)
	ctx.Request, _ = http.NewRequest(http.MethodPost, "/web/api/tql", reader)
	ctx.Request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	engine.HandleContext(ctx)
	require.Equal(t, 200, w.Result().StatusCode)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	expectReg := regexp.MustCompile(`^{"data":{"columns":\["x"\],"types":\["double"\],"rows":\[\[0\],\[1\]\]},"success":true,"reason":"success","elapse":"[0-9.]+[nµm]?s"}`)
	if !expectReg.MatchString(w.Body.String()) {
		t.Log("FAIL", w.Body.String())
		t.Fail()
	}
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
		//APPEND(table('example'))
		JSON()
	`)
	ctx.Request, _ = http.NewRequest(http.MethodPost, "/web/api/tql", reader)
	ctx.Request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	engine.HandleContext(ctx)
	require.Equal(t, 200, w.Result().StatusCode)
	//expectReg := regexp.MustCompile(`^{"success":false,"reason":"f\(MAPKEY\) invalid number of args; expect:1, actual:2","elapse":"[0-9.]+[nµm]?s","data":{"message":"append 0 row \(success 0, fail 0\)"}}`)
	expectReg := regexp.MustCompile(`^{"data":{"columns":\["x"\],"types":\["double"\],"rows":\[\]},"success":true,"reason":"success","elapse":"[0-9.]+[nµm]?s"}`)
	if !expectReg.MatchString(w.Body.String()) {
		t.Log("FAIL received:", w.Body.String())
		t.Fail()
	}
}

func TestTQLParam_CSV(t *testing.T) {
	w := httptest.NewRecorder()
	s, ctx, engine := NewMockServer(w)
	err := s.Login("sys", "manager")
	require.Nil(t, err)
	defer s.Shutdown()

	script := url.QueryEscape(`
		CSV(payload())
		CSV()
	`)
	payload := bytes.NewBufferString("a,1\nb,2\n")

	ctx.Request, _ = http.NewRequest(http.MethodPost, "/web/api/tql?"+TQL_SCRIPT_PARAM+"="+script, payload)
	ctx.Request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	ctx.Request.Header.Set("Content-Type", "text/csv")
	engine.HandleContext(ctx)
	require.Equal(t, 200, w.Result().StatusCode)
	require.Equal(t, "text/csv; charset=utf-8", w.Header().Get("Content-Type"))
	require.Equal(t, strings.Join([]string{"a,1", "b,2", "\n"}, "\n"), w.Body.String())
}
