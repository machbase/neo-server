package httpd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRefsFiles(t *testing.T) {
	w := httptest.NewRecorder()
	s, _, engine := NewMockServer(w)
	defer s.Shutdown()

	err := s.Login("sys", "manager")
	require.Nil(t, err)

	req, err := http.NewRequest(http.MethodGet, "/web/api/refs/", nil)
	if err != nil {
		t.Log("ERR", err.Error())
	}
	require.Nil(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	engine.ServeHTTP(w, req)
	require.Equal(t, 200, w.Result().StatusCode)
	require.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

	var rsp RefsResponse
	err = json.Unmarshal(w.Body.Bytes(), &rsp)
	require.Nil(t, err)

	require.Equal(t, 1, len(rsp.Data.Refs))
	require.Equal(t, rsp.Data.Refs[0].Label, "References")
	require.Equal(t, 6, len(rsp.Data.Refs[0].Items))

}
