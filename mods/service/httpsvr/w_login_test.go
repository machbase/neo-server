package httpsvr_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	. "github.com/machbase/neo-server/mods/service/httpsvr"
	"github.com/machbase/neo-server/mods/util/mock"
	"github.com/stretchr/testify/require"
)

type TestServerMock struct {
	mock.DatabaseServerMock
	mock.DatabaseAuthMock
}

func TestLoginRoute(t *testing.T) {

	dbMock := &TestServerMock{}
	dbMock.UserAuthFunc = func(user, password string) (bool, error) {
		return user == "sys" && password == "manager", nil
	}

	conf := &Config{
		Handlers: []HandlerConfig{
			{
				Prefix:  "/web",
				Handler: "web",
			},
		},
	}
	wsvr, err := New(dbMock, conf)
	if err != nil {
		t.Fatal(err)
	}

	router := gin.Default()
	wsvr.Route(router)

	var b *bytes.Buffer
	var loginReq *LoginReq
	var loginRsp *LoginRsp
	var w *httptest.ResponseRecorder
	var expectStatus int
	var req *http.Request

	// wrong password case - login
	b = &bytes.Buffer{}
	loginReq = &LoginReq{
		LoginName: "sys",
		Password:  "wrong",
	}
	expectStatus = http.StatusNotFound
	if err = json.NewEncoder(b).Encode(loginReq); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/web/api/login", b)
	req.Header.Set("Content-type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, expectStatus, w.Code, w.Body.String())

	// success case - login
	b = &bytes.Buffer{}
	loginReq = &LoginReq{
		LoginName: "sys",
		Password:  "manager",
	}
	expectStatus = http.StatusOK
	if err = json.NewEncoder(b).Encode(loginReq); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/web/api/login", b)
	req.Header.Set("Content-type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, expectStatus, w.Code, w.Body.String())

	dec := json.NewDecoder(w.Body)
	loginRsp = &LoginRsp{}
	err = dec.Decode(loginRsp)
	require.Nil(t, err, "login response decode")

	// success case - re-login
	b = &bytes.Buffer{}
	reloginReq := &ReLoginReq{
		RefreshToken: loginRsp.RefreshToken,
	}
	expectStatus = http.StatusOK
	if err = json.NewEncoder(b).Encode(reloginReq); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/web/api/relogin", b)
	req.Header.Set("Content-type", "application/json")
	req.Header.Set("Authorization", "Bearer "+loginRsp.AccessToken)
	router.ServeHTTP(w, req)
	require.Equal(t, expectStatus, w.Code, w.Body.String())

	dec = json.NewDecoder(w.Body)
	reRsp := &ReLoginRsp{}
	err = dec.Decode(reRsp)
	require.Nil(t, err, w.Body.String())
	require.True(t, reRsp.Success, w.Body.String())

	// success case - logout
	b = &bytes.Buffer{}
	logoutReq := &LogoutReq{
		RefreshToken: reRsp.RefreshToken,
	}
	expectStatus = http.StatusOK
	if err = json.NewEncoder(b).Encode(logoutReq); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/web/api/logout", b)
	req.Header.Set("Content-type", "application/json")
	req.Header.Set("Authorization", "Bearer "+reRsp.AccessToken)
	router.ServeHTTP(w, req)
	require.Equal(t, expectStatus, w.Code, w.Body.String())

	dec = json.NewDecoder(w.Body)
	logoutRsp := &LogoutRsp{}
	err = dec.Decode(logoutRsp)
	require.Nil(t, err, w.Body.String())
	require.True(t, logoutRsp.Success, w.Body.String())
}
