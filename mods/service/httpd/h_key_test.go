package httpd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/machbase/neo-server/api/mgmt"
	"github.com/stretchr/testify/require"
)

type mgmtServerMock struct {
	mgmt.UnimplementedManagementServer
}

func (mock *mgmtServerMock) ListKey(context.Context, *mgmt.ListKeyRequest) (*mgmt.ListKeyResponse, error) {
	return &mgmt.ListKeyResponse{Success: true}, nil
}

func (mock *mgmtServerMock) GenKey(context.Context, *mgmt.GenKeyRequest) (*mgmt.GenKeyResponse, error) {
	return &mgmt.GenKeyResponse{Success: true}, nil
}

func (mock *mgmtServerMock) ServerKey(context.Context, *mgmt.ServerKeyRequest) (*mgmt.ServerKeyResponse, error) {
	return &mgmt.ServerKeyResponse{Success: true}, nil
}

func (mock *mgmtServerMock) DelKey(context.Context, *mgmt.DelKeyRequest) (*mgmt.DelKeyResponse, error) {
	return &mgmt.DelKeyResponse{Success: true}, nil
}

func TestKey(t *testing.T) {
	webService, err := New(&DatabaseMock{},
		OptionDebugMode(true),
	)
	if err != nil {
		t.Fatal(err)
	}

	wsvr := webService.(*httpd)
	wsvr.mgmtImpl = &mgmtServerMock{}

	router := wsvr.Router()

	var b *bytes.Buffer
	var w *httptest.ResponseRecorder
	var req *http.Request
	var expectStatus int

	// accessToken
	w = httptest.NewRecorder()
	s, _, _ := NewMockServer(w)
	err = s.Login("sys", "manager")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Shutdown()

	// ========================
	//GET key-list
	w = httptest.NewRecorder()
	req, err = http.NewRequest("GET", "/web/api/keys", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	router.ServeHTTP(w, req)

	listRsp := struct {
		Success bool      `json:"success"`
		Reason  string    `json:"reason"`
		Data    []KeyInfo `json:"data"`
		Elapse  string    `json:"elapse"`
	}{}

	payload := w.Body.Bytes()
	err = json.Unmarshal(payload, &listRsp)
	if err != nil {
		t.Log("rsp", string(payload))
		t.Fatal(err)
	}

	expectStatus = http.StatusOK
	require.Equal(t, expectStatus, w.Code, listRsp)

	// ========================
	// POST key-gen
	b = &bytes.Buffer{}

	param := map[string]interface{}{}
	param["name"] = "twelve"
	param["notValidAfter"] = time.Now().Add(10 * time.Hour).Unix()
	if err := json.NewEncoder(b).Encode(param); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req, err = http.NewRequest("POST", "/web/api/keys", b)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	router.ServeHTTP(w, req)

	genRsp := struct {
		Success     bool   `json:"success"`
		Reason      string `json:"reason"`
		Elapse      string `json:"elapse"`
		ServerKey   string `json:"serverKey"`
		PrivateKey  string `json:"privateKey"`
		Certificate string `json:"certificate"`
		Token       string `json:"token"`
	}{}
	err = json.Unmarshal(w.Body.Bytes(), &genRsp)
	if err != nil {
		t.Log(w.Body.String())
		t.Fatal(err)
	}

	expectStatus = http.StatusOK
	require.Equal(t, expectStatus, w.Code, genRsp)

	// ========================
	// DELETE key-delete
	w = httptest.NewRecorder()
	req, err = http.NewRequest("DELETE", "/web/api/keys/eleven", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	router.ServeHTTP(w, req)

	deleteRsp := struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}
	err = json.Unmarshal(w.Body.Bytes(), &deleteRsp)
	if err != nil {
		t.Fatal(err)
	}

	expectStatus = http.StatusOK
	require.Equal(t, expectStatus, w.Code, deleteRsp)
}
