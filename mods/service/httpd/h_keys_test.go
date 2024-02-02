package httpd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetKeys(t *testing.T) {
	dbMock := &TestServerMock{}
	dbMock.UserAuthFunc = func(user, password string) (bool, error) {
		return user == "sys" && password == "manager", nil
	}
	wservice, err := New(dbMock,
		OptionDebugMode(true),
		OptionHandler("/web", HandlerWeb),
	)
	if err != nil {
		t.Fatal(err)
	}
	wsvr := wservice.(*httpd)
	router := wsvr.Router()

	var b *bytes.Buffer
	var loginReq *LoginReq
	var loginRsp *LoginRsp
	var w *httptest.ResponseRecorder
	var req *http.Request

	loginReq = &LoginReq{
		LoginName: "sys",
		Password:  "manager",
	}
	b = &bytes.Buffer{}
	if err = json.NewEncoder(b).Encode(loginReq); err != nil {
		t.Fatal(err)
	}
	if req, err = http.NewRequest("POST", "/web/api/login", b); err != nil {
		t.Fatal(err)
	} else {
		req.Header.Set("Content-Type", "application/json")
	}
	// login
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	dec := json.NewDecoder(w.Body)
	loginRsp = &LoginRsp{}
	err = dec.Decode(loginRsp)
	require.Nil(t, err, "login response decode")
	require.Equal(t, 200, w.Code, w.Result().Status)

	// get keys
	w = httptest.NewRecorder()
	req, err = http.NewRequest("GET", "/web/api/keys", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+loginRsp.AccessToken)
	router.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, w.Body.String())

	// get key id
	w = httptest.NewRecorder()
	req, err = http.NewRequest("GET", "/web/api/keys/12345", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+loginRsp.AccessToken)
	router.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, w.Body.String())

	// delete key id
	w = httptest.NewRecorder()
	req, err = http.NewRequest("DELETE", "/web/api/keys/12345", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+loginRsp.AccessToken)
	router.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, w.Body.String())

	// gen new key
	w = httptest.NewRecorder()
	req, err = http.NewRequest("DELETE", "/web/api/keys/12345", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+loginRsp.AccessToken)
	router.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, w.Body.String())
}
