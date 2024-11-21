package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	bridgerpc "github.com/machbase/neo-server/v8/api/bridge"
	"github.com/machbase/neo-server/v8/api/mgmt"
	"github.com/machbase/neo-server/v8/api/schedule"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/require"
)

type schedServerMock struct {
	schedule.ManagementServer
}

func (mock *schedServerMock) GetSchedule(ctx context.Context, req *schedule.GetScheduleRequest) (*schedule.GetScheduleResponse, error) {
	if req.Name == "eleven" {
		return &schedule.GetScheduleResponse{Success: true, Schedule: &schedule.Schedule{
			Name:      "eleven",
			AutoStart: true,
		}}, nil
	}
	return &schedule.GetScheduleResponse{Success: false}, nil
}

func (mock *schedServerMock) ListSchedule(context.Context, *schedule.ListScheduleRequest) (*schedule.ListScheduleResponse, error) {
	return &schedule.ListScheduleResponse{Success: true}, nil
}

func (mock *schedServerMock) AddSchedule(ctx context.Context, req *schedule.AddScheduleRequest) (*schedule.AddScheduleResponse, error) {
	_, err := parseSchedule(req.Schedule)
	if err != nil {
		return &schedule.AddScheduleResponse{Success: false}, err
	}
	return &schedule.AddScheduleResponse{Success: true}, nil
}

func (mock *schedServerMock) StartSchedule(context.Context, *schedule.StartScheduleRequest) (*schedule.StartScheduleResponse, error) {
	return &schedule.StartScheduleResponse{Success: true}, nil
}

func (mock *schedServerMock) StopSchedule(context.Context, *schedule.StopScheduleRequest) (*schedule.StopScheduleResponse, error) {
	return &schedule.StopScheduleResponse{Success: true}, nil
}

func (mock *schedServerMock) UpdateSchedule(ctx context.Context, req *schedule.UpdateScheduleRequest) (*schedule.UpdateScheduleResponse, error) {
	_, err := parseSchedule(req.Schedule)
	if err != nil {
		return nil, err
	}
	return &schedule.UpdateScheduleResponse{Success: true}, nil
}

func (mock *schedServerMock) DelSchedule(context.Context, *schedule.DelScheduleRequest) (*schedule.DelScheduleResponse, error) {
	return &schedule.DelScheduleResponse{Success: true}, nil
}

func TestTimer(t *testing.T) {
	wsvr, err := NewHttp(&DatabaseMock{},
		WithHttpDebugMode(true),
	)
	if err != nil {
		t.Fatal(err)
	}

	wsvr.schedMgmtImpl = &schedServerMock{}

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
	//GET /api/timers
	w = httptest.NewRecorder()
	req, err = http.NewRequest("GET", "/web/api/timers", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	router.ServeHTTP(w, req)

	listRsp := struct {
		Success bool                 `json:"success"`
		Reason  string               `json:"reason"`
		Data    []*schedule.Schedule `json:"data"`
		Elapse  string               `json:"elapse"`
	}{}

	payload := w.Body.Bytes()
	err = json.Unmarshal(payload, &listRsp)
	if err != nil {
		t.Fatal(err)
	}

	expectStatus = http.StatusOK
	require.Equal(t, expectStatus, w.Code, listRsp)

	// ========================
	// POST /api/timers  Success, correct schedule
	addReq := struct {
		Name      string `json:"name"`
		AutoStart bool   `json:"autoStart"`
		Schedule  string `json:"schedule"`
		TqlPath   string `json:"tqlPath"`
	}{
		Name:      "twelve",
		AutoStart: false,
		Schedule:  "0 30 * * * *",
		TqlPath:   "timer.tql",
	}

	b = &bytes.Buffer{}
	if err = json.NewEncoder(b).Encode(addReq); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req, err = http.NewRequest("POST", "/web/api/timers", b)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	router.ServeHTTP(w, req)

	rsp := struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}

	payload = w.Body.Bytes()
	err = json.Unmarshal(payload, &rsp)
	if err != nil {
		t.Fatal(err)
	}

	expectStatus = http.StatusOK
	require.Equal(t, expectStatus, w.Code, rsp)

	// ========================
	// POST /api/timers  Failed, incorrect schedule
	addReq = struct {
		Name      string `json:"name"`
		AutoStart bool   `json:"autoStart"`
		Schedule  string `json:"schedule"`
		TqlPath   string `json:"tqlPath"`
	}{
		Name:      "twelve",
		AutoStart: false,
		Schedule:  "* * a b c d ",
		TqlPath:   "timer.tql",
	}

	b = &bytes.Buffer{}
	if err = json.NewEncoder(b).Encode(addReq); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req, err = http.NewRequest("POST", "/web/api/timers", b)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	router.ServeHTTP(w, req)

	rsp = struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}

	payload = w.Body.Bytes()
	err = json.Unmarshal(payload, &rsp)
	if err != nil {
		t.Fatal(err)
	}

	expectStatus = http.StatusInternalServerError
	require.Equal(t, expectStatus, w.Code, rsp)

	// ========================
	// POST /api/timers/:name/state  START
	doReq := struct {
		State string `json:"state"`
	}{
		State: "start",
	}

	b = &bytes.Buffer{}
	if err = json.NewEncoder(b).Encode(doReq); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req, err = http.NewRequest("POST", "/web/api/timers/twelve/state", b)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	router.ServeHTTP(w, req)

	rsp = struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}

	payload = w.Body.Bytes()
	err = json.Unmarshal(payload, &rsp)
	if err != nil {
		t.Fatal(err)
	}

	expectStatus = http.StatusOK
	require.Equal(t, expectStatus, w.Code, rsp)

	// ========================
	// POST /api/timers/:name/state  Stop
	doReq = struct {
		State string `json:"state"`
	}{
		State: "stop",
	}

	b = &bytes.Buffer{}
	if err = json.NewEncoder(b).Encode(doReq); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req, err = http.NewRequest("POST", "/web/api/timers/eleven/state", b)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	router.ServeHTTP(w, req)

	rsp = struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}

	payload = w.Body.Bytes()
	err = json.Unmarshal(payload, &rsp)
	if err != nil {
		t.Fatal(err)
	}

	expectStatus = http.StatusOK
	require.Equal(t, expectStatus, w.Code, rsp)

	// ========================
	// PUT /api/timers/:name Update
	updateReq := struct {
		AutoStart bool   `json:"autoStart"`
		Schedule  string `json:"schedule"`
		Path      string `json:"path"`
	}{
		AutoStart: true,
		Schedule:  "0 30 * * * *",
		Path:      "example.tql",
	}

	b = &bytes.Buffer{}
	if err = json.NewEncoder(b).Encode(updateReq); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req, err = http.NewRequest("PUT", "/web/api/timers/eleven", b)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	router.ServeHTTP(w, req)

	rsp = struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}

	payload = w.Body.Bytes()
	err = json.Unmarshal(payload, &rsp)
	if err != nil {
		t.Fatal(err)
	}

	expectStatus = http.StatusOK
	require.Equal(t, expectStatus, w.Code, rsp)

	// ========================
	// DELETE /api/timers/:name
	w = httptest.NewRecorder()
	req, err = http.NewRequest("DELETE", "/web/api/timers/eleven", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	router.ServeHTTP(w, req)

	rsp = struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}

	payload = w.Body.Bytes()
	err = json.Unmarshal(payload, &rsp)
	if err != nil {
		t.Fatal(err)
	}

	expectStatus = http.StatusOK
	require.Equal(t, expectStatus, w.Code, rsp)
}

func parseSchedule(schedule string) (cron.Schedule, error) {
	scheduleParser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	if s, err := scheduleParser.Parse(schedule); err != nil {
		return nil, fmt.Errorf("invalid schedule, %s", err.Error())
	} else {
		return s, err
	}
}

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
	wsvr, err := NewHttp(&DatabaseMock{},
		WithHttpDebugMode(true),
	)
	if err != nil {
		t.Fatal(err)
	}

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
	wsvr, err := NewHttp(&DatabaseMock{},
		WithHttpDebugMode(true),
	)
	if err != nil {
		t.Fatal(err)
	}

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
