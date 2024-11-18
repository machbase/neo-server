package httpd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	bridgerpc "github.com/machbase/neo-server/v8/api/bridge"
)

type bridgeServerMock struct {
	bridgerpc.UnimplementedManagementServer
}

func (mock bridgeServerMock) ListBridge(ctx context.Context, req *bridgerpc.ListBridgeRequest) (*bridgerpc.ListBridgeResponse, error) {
	return &bridgerpc.ListBridgeResponse{Success: true}, nil
}
func (mock bridgeServerMock) AddBridge(ctx context.Context, req *bridgerpc.AddBridgeRequest) (*bridgerpc.AddBridgeResponse, error) {
	return &bridgerpc.AddBridgeResponse{Success: true}, nil
}
func (mock bridgeServerMock) DelBridge(ctx context.Context, req *bridgerpc.DelBridgeRequest) (*bridgerpc.DelBridgeResponse, error) {
	return &bridgerpc.DelBridgeResponse{Success: true}, nil
}
func (mock bridgeServerMock) TestBridge(ctx context.Context, req *bridgerpc.TestBridgeRequest) (*bridgerpc.TestBridgeResponse, error) {
	return &bridgerpc.TestBridgeResponse{Success: true}, nil
}
func (mock bridgeServerMock) GetBridge(ctx context.Context, req *bridgerpc.GetBridgeRequest) (*bridgerpc.GetBridgeResponse, error) {
	return &bridgerpc.GetBridgeResponse{Success: true}, nil
}

func TestBridge(t *testing.T) {
	webService, err := New(&DatabaseMock{},
		OptionDebugMode(true),
	)
	if err != nil {
		t.Fatal(err)
	}

	wsvr := webService.(*httpd)
	wsvr.bridgeMgmtImpl = bridgeServerMock{}

	router := wsvr.Router()

	// var b *bytes.Buffer
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
	req, err = http.NewRequest("GET", "/web/api/bridges", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	router.ServeHTTP(w, req)

	listRsp := struct {
		Success bool                `json:"success"`
		Reason  string              `json:"reason"`
		Data    []*bridgerpc.Bridge `json:"data"`
		Elapse  string              `json:"elapse"`
	}{}

	payload := w.Body.Bytes()
	err = json.Unmarshal(payload, &listRsp)
	if err != nil {
		t.Log("rsp", string(payload))
		t.Fatal(err)
	}

	expectStatus = http.StatusOK
	require.Equal(t, expectStatus, w.Code, listRsp)

}
