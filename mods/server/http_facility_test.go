package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/scheduler"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/require"
)

func request(t *testing.T, jwt *LoginRsp, method, requestPath string, body io.Reader) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, httpServerAddress+requestPath, body)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwt.AccessToken))

	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	payload, err := io.ReadAll(rsp.Body)
	require.NoError(t, err)
	rsp.Body.Close()
	return rsp, payload
}

func TestTimer(t *testing.T) {
	// Login
	jwt := HttpTestLogin(t, "sys", "manager")
	timerName := fmt.Sprintf("timer-%d", time.Now().UnixNano())
	invalidTimerName := fmt.Sprintf("%s-invalid", timerName)

	listRsp := struct {
		Success bool                  `json:"success"`
		Reason  string                `json:"reason"`
		Data    []*scheduler.Schedule `json:"data"`
		Elapse  string                `json:"elapse"`
	}{}

	// ========================
	//GET /api/timers
	rsp, payload := request(t, jwt, http.MethodGet, "/web/api/timers", nil)
	err := json.Unmarshal(payload, &listRsp)
	if err != nil {
		t.Log("rsp", string(payload))
		t.Fatal(err)
	}
	require.Equal(t, http.StatusOK, rsp.StatusCode, listRsp)

	// ========================
	// POST /api/timers  Success, correct schedule
	addReq := struct {
		Name      string `json:"name"`
		AutoStart bool   `json:"autoStart"`
		Schedule  string `json:"schedule"`
		Path      string `json:"path"`
	}{
		Name:      timerName,
		AutoStart: false,
		Schedule:  "0 30 * * * *",
		Path:      "csv_map.tql",
	}

	b := &bytes.Buffer{}
	if err = json.NewEncoder(b).Encode(addReq); err != nil {
		t.Fatal(err)
	}
	rsp, payload = request(t, jwt, http.MethodPost, "/web/api/timers", b)

	addRsp := struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}

	err = json.Unmarshal(payload, &addRsp)
	if err != nil {
		t.Fatal(err)
	}

	require.Equal(t, http.StatusOK, rsp.StatusCode, addRsp)

	// ========================
	// POST /api/timers  Failed, incorrect schedule
	addReq = struct {
		Name      string `json:"name"`
		AutoStart bool   `json:"autoStart"`
		Schedule  string `json:"schedule"`
		Path      string `json:"path"`
	}{
		Name:      invalidTimerName,
		AutoStart: false,
		Schedule:  "* * a b c d ",
		Path:      "csv_map.tql",
	}

	b = &bytes.Buffer{}
	if err = json.NewEncoder(b).Encode(addReq); err != nil {
		t.Fatal(err)
	}
	rsp, payload = request(t, jwt, http.MethodPost, "/web/api/timers", b)

	addRsp = struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}

	err = json.Unmarshal(payload, &addRsp)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, http.StatusInternalServerError, rsp.StatusCode, addRsp)

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

	rsp, payload = request(t, jwt, http.MethodPost, "/web/api/timers/"+timerName+"/state", b)
	stateRsp := struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}
	err = json.Unmarshal(payload, &stateRsp)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, http.StatusOK, rsp.StatusCode, stateRsp)

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

	rsp, payload = request(t, jwt, http.MethodPost, "/web/api/timers/"+timerName+"/state", b)
	stateRsp = struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}
	err = json.Unmarshal(payload, &stateRsp)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, http.StatusOK, rsp.StatusCode, stateRsp)

	// ========================
	// PUT /api/timers/:name Update
	updateReq := struct {
		AutoStart bool   `json:"autoStart"`
		Schedule  string `json:"schedule"`
		Path      string `json:"path"`
	}{
		AutoStart: true,
		Schedule:  "0 30 * * * *",
		Path:      "csv_map.tql",
	}

	b = &bytes.Buffer{}
	if err = json.NewEncoder(b).Encode(updateReq); err != nil {
		t.Fatal(err)
	}

	rsp, payload = request(t, jwt, http.MethodPut, "/web/api/timers/"+timerName, b)
	updateRsp := struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}
	err = json.Unmarshal(payload, &updateRsp)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, http.StatusOK, rsp.StatusCode, updateRsp)

	// ========================
	// DELETE /api/timers/:name
	rsp, payload = request(t, jwt, http.MethodDelete, "/web/api/timers/"+timerName, nil)
	deleteRsp := struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}
	err = json.Unmarshal(payload, &deleteRsp)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, http.StatusOK, rsp.StatusCode, deleteRsp)
}

func parseSchedule(schedule string) (cron.Schedule, error) {
	scheduleParser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	if s, err := scheduleParser.Parse(schedule); err != nil {
		return nil, fmt.Errorf("invalid schedule, %s", err.Error())
	} else {
		return s, err
	}
}

func TestKey(t *testing.T) {
	// Login
	jwt := HttpTestLogin(t, "sys", "manager")

	keyExists := func(keys []KeyInfo, name string) bool {
		for _, key := range keys {
			if key.Id == name {
				return true
			}
		}
		return false
	}

	listRsp := struct {
		Success bool      `json:"success"`
		Reason  string    `json:"reason"`
		Data    []KeyInfo `json:"data"`
		Elapse  string    `json:"elapse"`
	}{}

	// ========================
	//GET key-list
	rsp, payload := request(t, jwt, http.MethodGet, "/web/api/keys", nil)
	err := json.Unmarshal(payload, &listRsp)
	if err != nil {
		t.Log("rsp", string(payload))
		t.Fatal(err)
	}

	require.Equal(t, http.StatusOK, rsp.StatusCode, listRsp)
	require.False(t, keyExists(listRsp.Data, "twelve"))

	// ========================
	// POST key-gen
	b := &bytes.Buffer{}

	param := map[string]interface{}{}
	param["name"] = "twelve"
	param["notValidAfter"] = time.Now().Add(10 * time.Hour).Unix()
	if err := json.NewEncoder(b).Encode(param); err != nil {
		t.Fatal(err)
	}

	rsp, payload = request(t, jwt, http.MethodPost, "/web/api/keys", b)
	genRsp := struct {
		Success     bool   `json:"success"`
		Reason      string `json:"reason"`
		Elapse      string `json:"elapse"`
		ServerKey   string `json:"serverKey"`
		PrivateKey  string `json:"privateKey"`
		Certificate string `json:"certificate"`
		Token       string `json:"token"`
	}{}
	err = json.Unmarshal(payload, &listRsp)
	if err != nil {
		t.Log("rsp", string(payload))
		t.Fatal(err)
	}
	err = json.Unmarshal(payload, &genRsp)
	if err != nil {
		t.Log(string(payload))
		t.Fatal(err)
	}

	require.Equal(t, http.StatusOK, rsp.StatusCode, genRsp)
	require.True(t, genRsp.Success)
	require.Equal(t, "success", genRsp.Reason)
	require.NotEmpty(t, genRsp.ServerKey)
	require.NotEmpty(t, genRsp.PrivateKey)
	require.NotEmpty(t, genRsp.Certificate)
	require.NotEmpty(t, genRsp.Token)

	// ========================
	// GET key-list after creation
	rsp, payload = request(t, jwt, http.MethodGet, "/web/api/keys", nil)
	listRsp = struct {
		Success bool      `json:"success"`
		Reason  string    `json:"reason"`
		Data    []KeyInfo `json:"data"`
		Elapse  string    `json:"elapse"`
	}{}
	err = json.Unmarshal(payload, &listRsp)
	if err != nil {
		t.Log("rsp", string(payload))
		t.Fatal(err)
	}
	require.Equal(t, http.StatusOK, rsp.StatusCode, listRsp)
	require.True(t, keyExists(listRsp.Data, "twelve"))

	// ========================
	// DELETE key-delete
	deleteRsp := struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}
	rsp, payload = request(t, jwt, http.MethodDelete, "/web/api/keys/twelve", nil)
	err = json.Unmarshal(payload, &deleteRsp)
	if err != nil {
		t.Fatal(err)
	}

	require.Equal(t, http.StatusOK, rsp.StatusCode, deleteRsp)
	require.True(t, deleteRsp.Success)
	require.Equal(t, "success", deleteRsp.Reason)

	// ========================
	// GET key-list after deletion
	rsp, payload = request(t, jwt, http.MethodGet, "/web/api/keys", nil)
	listRsp = struct {
		Success bool      `json:"success"`
		Reason  string    `json:"reason"`
		Data    []KeyInfo `json:"data"`
		Elapse  string    `json:"elapse"`
	}{}
	err = json.Unmarshal(payload, &listRsp)
	if err != nil {
		t.Log("rsp", string(payload))
		t.Fatal(err)
	}
	require.Equal(t, http.StatusOK, rsp.StatusCode, listRsp)
	require.False(t, keyExists(listRsp.Data, "twelve"))
}

func TestShell(t *testing.T) {
	jwt := HttpTestLogin(t, "sys", "manager")

	execPath, err := os.Executable()
	require.NoError(t, err)

	shellId := strings.ToUpper(fmt.Sprintf("test-shell-%d", time.Now().UnixNano()))
	shellReq := &model.ShellDefinition{
		Id:      shellId,
		Type:    model.SHELL_TERM,
		Icon:    "console",
		Label:   "TEST SHELL",
		Command: fmt.Sprintf(`"%s" shell`, execPath),
		Attributes: &model.ShellAttributes{
			Removable: true,
			Cloneable: true,
			Editable:  true,
		},
	}

	// ========================
	// GET shell before creation
	rsp, payload := request(t, jwt, http.MethodGet, "/web/api/shell/"+shellId, nil)
	getRsp := struct {
		Success bool                   `json:"success"`
		Reason  string                 `json:"reason"`
		Data    *model.ShellDefinition `json:"data"`
		Elapse  string                 `json:"elapse"`
	}{}
	err = json.Unmarshal(payload, &getRsp)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, rsp.StatusCode, string(payload))
	require.False(t, getRsp.Success)
	require.Equal(t, "not found", getRsp.Reason)

	// ========================
	// POST shell create
	b := &bytes.Buffer{}
	err = json.NewEncoder(b).Encode(shellReq)
	require.NoError(t, err)

	rsp, payload = request(t, jwt, http.MethodPost, "/web/api/shell/"+shellId, b)
	postRsp := struct {
		Success bool                   `json:"success"`
		Reason  string                 `json:"reason"`
		Data    *model.ShellDefinition `json:"data"`
		Elapse  string                 `json:"elapse"`
	}{}
	err = json.Unmarshal(payload, &postRsp)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode, string(payload))
	require.True(t, postRsp.Success)
	require.Equal(t, "success", postRsp.Reason)
	require.NotNil(t, postRsp.Data)
	require.Equal(t, shellId, postRsp.Data.Id)
	require.Equal(t, shellReq.Command, postRsp.Data.Command)

	// ========================
	// GET shell after creation
	rsp, payload = request(t, jwt, http.MethodGet, "/web/api/shell/"+shellId, nil)
	getRsp = struct {
		Success bool                   `json:"success"`
		Reason  string                 `json:"reason"`
		Data    *model.ShellDefinition `json:"data"`
		Elapse  string                 `json:"elapse"`
	}{}
	err = json.Unmarshal(payload, &getRsp)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode, string(payload))
	require.True(t, getRsp.Success)
	require.NotNil(t, getRsp.Data)
	require.Equal(t, shellId, getRsp.Data.Id)
	require.Equal(t, shellReq.Label, getRsp.Data.Label)

	// ========================
	// GET shell copy
	rsp, payload = request(t, jwt, http.MethodGet, "/web/api/shell/"+shellId+"/copy", nil)
	copyRsp := struct {
		Success bool                   `json:"success"`
		Reason  string                 `json:"reason"`
		Data    *model.ShellDefinition `json:"data"`
		Elapse  string                 `json:"elapse"`
	}{}
	err = json.Unmarshal(payload, &copyRsp)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode, string(payload))
	require.True(t, copyRsp.Success)
	require.NotNil(t, copyRsp.Data)
	require.NotEmpty(t, copyRsp.Data.Id)
	require.NotEqual(t, shellId, copyRsp.Data.Id)
	require.Equal(t, "CUSTOM SHELL", copyRsp.Data.Label)
	require.Equal(t, shellReq.Command, copyRsp.Data.Command)

	// ========================
	// DELETE original shell
	rsp, payload = request(t, jwt, http.MethodDelete, "/web/api/shell/"+shellId, nil)
	deleteRsp := struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}
	err = json.Unmarshal(payload, &deleteRsp)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode, string(payload))
	require.True(t, deleteRsp.Success)
	require.Equal(t, "success", deleteRsp.Reason)

	// ========================
	// DELETE copied shell
	rsp, payload = request(t, jwt, http.MethodDelete, "/web/api/shell/"+copyRsp.Data.Id, nil)
	deleteRsp = struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}
	err = json.Unmarshal(payload, &deleteRsp)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode, string(payload))
	require.True(t, deleteRsp.Success)

	// ========================
	// GET original shell after deletion
	rsp, payload = request(t, jwt, http.MethodGet, "/web/api/shell/"+shellId, nil)
	getRsp = struct {
		Success bool                   `json:"success"`
		Reason  string                 `json:"reason"`
		Data    *model.ShellDefinition `json:"data"`
		Elapse  string                 `json:"elapse"`
	}{}
	err = json.Unmarshal(payload, &getRsp)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, rsp.StatusCode, string(payload))
	require.False(t, getRsp.Success)
	require.Equal(t, "not found", getRsp.Reason)
}

func TestBridge(t *testing.T) {
	// accessToken
	jwt := HttpTestLogin(t, "sys", "manager")

	// ========================
	//GET bridge-list
	rsp, payload := request(t, jwt, http.MethodGet, "/web/api/bridges", nil)

	listRsp := struct {
		Success bool                 `json:"success"`
		Reason  string               `json:"reason"`
		Data    []*bridge.BridgeInfo `json:"data"`
		Elapse  string               `json:"elapse"`
	}{}

	err := json.Unmarshal(payload, &listRsp)
	if err != nil {
		t.Log("rsp", string(payload))
		t.Fatal(err)
	}

	require.Equal(t, http.StatusOK, rsp.StatusCode, listRsp)

	// ========================
	// POST bridge-add
	b := &bytes.Buffer{}
	bridgeReq := map[string]string{
		"name": "test-br",
		"type": "sqlite",
		"path": "file::memory:?cache=shared",
	}
	if err = json.NewEncoder(b).Encode(bridgeReq); err != nil {
		t.Fatal(err)
	}

	rsp, payload = request(t, jwt, http.MethodPost, "/web/api/bridges", b)

	postRsp := struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}
	err = json.Unmarshal(payload, &postRsp)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, http.StatusOK, rsp.StatusCode, postRsp)

	defer func() {
		// ========================
		// DELETE bridge-delete
		rsp, payload = request(t, jwt, http.MethodDelete, "/web/api/bridges/test-br", nil)

		deleteRsp := struct {
			Success bool   `json:"success"`
			Reason  string `json:"reason"`
			Elapse  string `json:"elapse"`
		}{}
		err = json.Unmarshal(payload, &deleteRsp)
		if err != nil {
			t.Fatal(err)
		}
		require.Equal(t, http.StatusOK, rsp.StatusCode, deleteRsp)
	}()

	// ========================
	// POST bridge-add duplicate
	b = &bytes.Buffer{}
	bridgeReq = map[string]string{
		"name": "test-br",
		"type": "sqlite",
		"path": "file::memory:?cache=shared",
	}
	if err = json.NewEncoder(b).Encode(bridgeReq); err != nil {
		t.Fatal(err)
	}

	rsp, payload = request(t, jwt, http.MethodPost, "/web/api/bridges", b)
	require.Equal(t, http.StatusBadRequest, rsp.StatusCode)
	json.Unmarshal(payload, &postRsp)
	require.False(t, postRsp.Success)
	require.Equal(t, "'test-br' is duplicate bridge name.", postRsp.Reason)

	// ========================
	// DELETE bridge-delete
	rsp, payload = request(t, jwt, http.MethodDelete, "/web/api/bridges/non-existing-br", nil)

	deleteRsp := struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}
	err = json.Unmarshal(payload, &deleteRsp)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, http.StatusInternalServerError, rsp.StatusCode, deleteRsp)
	require.False(t, deleteRsp.Success)
	if runtime.GOOS != "windows" {
		require.Contains(t, deleteRsp.Reason, "no such file")
	}

	// ========================
	// POST bridge-state test
	b = &bytes.Buffer{}
	stateReq := map[string]string{
		"state": "test",
	}
	if err = json.NewEncoder(b).Encode(stateReq); err != nil {
		t.Fatal(err)
	}
	rsp, payload = request(t, jwt, http.MethodPost, "/web/api/bridges/test-br/state", b)
	stateRsp := struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}
	err = json.Unmarshal(payload, &stateRsp)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, http.StatusOK, rsp.StatusCode, stateRsp)

	// ========================
	// POST bridge-state invalid
	b = &bytes.Buffer{}
	stateReq = map[string]string{
		"state": "invalid",
	}
	if err = json.NewEncoder(b).Encode(stateReq); err != nil {
		t.Fatal(err)
	}

	rsp, payload = request(t, jwt, http.MethodPost, "/web/api/bridges/test-br/state", b)
	stateRsp = struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}
	err = json.Unmarshal(payload, &stateRsp)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, http.StatusBadRequest, rsp.StatusCode, stateRsp)
}

func TestSubscriber(t *testing.T) {
	jwt := HttpTestLogin(t, "sys", "manager")

	// ========================
	// POST add mqtt bridge for subscriber test
	b := &bytes.Buffer{}
	bridgeReq := map[string]string{
		"name": "existing-bridge",
		"type": "mqtt",
		"path": "broker=" + mqttServerAddress + " id=client-id",
	}
	if err := json.NewEncoder(b).Encode(bridgeReq); err != nil {
		t.Fatal(err)
	}
	rsp, payload := request(t, jwt, http.MethodPost, "/web/api/bridges", b)
	require.Equal(t, http.StatusOK, rsp.StatusCode)

	defer func() {
		// ========================
		// DELETE mqtt bridge
		rsp, _ = request(t, jwt, http.MethodDelete, "/web/api/bridges/existing-bridge", nil)
		require.Equal(t, http.StatusOK, rsp.StatusCode)
	}()

	// ========================
	// POST /api/subscribers  - add subscriber
	addReq := struct {
		Name      string `json:"name"`
		AutoStart bool   `json:"autoStart"`
		Bridge    string `json:"bridge"`
		Topic     string `json:"topic"`
		Task      string `json:"task"`
		QoS       int    `json:"QoS"`
	}{
		Name:      "test-sub",
		AutoStart: false,
		Bridge:    "existing-bridge",
		Topic:     "test/topic",
		Task:      "sub.tql",
		QoS:       0,
	}

	b = &bytes.Buffer{}
	if err := json.NewEncoder(b).Encode(addReq); err != nil {
		t.Fatal(err)
	}

	rsp, payload = request(t, jwt, http.MethodPost, "/web/api/subscribers", b)
	addRsp := struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}
	err := json.Unmarshal(payload, &addRsp)
	if err != nil {
		t.Log(string(payload))
		t.Fatal(err)
	}
	require.Equal(t, http.StatusOK, rsp.StatusCode, addRsp)

	// ========================
	// GET /api/subscribers/:name
	rsp, payload = request(t, jwt, http.MethodGet, "/web/api/subscribers/test-sub", nil)
	require.Equal(t, http.StatusOK, rsp.StatusCode)

	// ========================
	// GET /api/subscribers
	rsp, payload = request(t, jwt, http.MethodGet, "/web/api/subscribers", nil)
	listRsp := struct {
		Success bool                  `json:"success"`
		Reason  string                `json:"reason"`
		Data    []*scheduler.Schedule `json:"data"`
		Elapse  string                `json:"elapse"`
	}{}
	if err := json.Unmarshal(payload, &listRsp); err != nil {
		t.Log("rsp", string(payload))
		t.Fatal(err)
	}
	require.Equal(t, "TEST-SUB", listRsp.Data[0].Name)
	require.Equal(t, "existing-bridge", listRsp.Data[0].Bridge)
	require.Equal(t, http.StatusOK, rsp.StatusCode, listRsp)

	// ========================
	// POST /api/subscribers/:name/state START
	b = &bytes.Buffer{}
	stateReq := struct {
		State string `json:"state"`
	}{State: "start"}
	if err = json.NewEncoder(b).Encode(stateReq); err != nil {
		t.Fatal(err)
	}

	rsp, payload = request(t, jwt, http.MethodPost, "/web/api/subscribers/test-sub/state", b)
	stateRsp := struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}
	err = json.Unmarshal(payload, &stateRsp)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, http.StatusOK, rsp.StatusCode, stateRsp)

	// ========================
	// POST /api/subscribers/:name/state STOP
	b = &bytes.Buffer{}
	stateReq = struct {
		State string `json:"state"`
	}{State: "stop"}
	if err = json.NewEncoder(b).Encode(stateReq); err != nil {
		t.Fatal(err)
	}
	rsp, payload = request(t, jwt, http.MethodPost, "/web/api/subscribers/test-sub/state", b)
	stateRsp = struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}
	err = json.Unmarshal(payload, &stateRsp)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, http.StatusOK, rsp.StatusCode, stateRsp)

	// ========================
	// POST /api/subscribers/:name/state invalid
	b = &bytes.Buffer{}
	stateReq = struct {
		State string `json:"state"`
	}{State: "invalid"}
	if err = json.NewEncoder(b).Encode(stateReq); err != nil {
		t.Fatal(err)
	}

	rsp, payload = request(t, jwt, http.MethodPost, "/web/api/subscribers/test-sub/state", b)
	stateRsp = struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}
	err = json.Unmarshal(payload, &stateRsp)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, http.StatusBadRequest, rsp.StatusCode, stateRsp)

	// ========================
	// DELETE /api/subscribers/:name
	rsp, payload = request(t, jwt, http.MethodDelete, "/web/api/subscribers/test-sub", nil)
	deleteRsp := struct {
		Success bool   `json:"success"`
		Reason  string `json:"reason"`
		Elapse  string `json:"elapse"`
	}{}
	err = json.Unmarshal(payload, &deleteRsp)
	if err != nil {
		t.Fatal(err)
	}
	require.Equal(t, http.StatusOK, rsp.StatusCode, deleteRsp)
	require.True(t, deleteRsp.Success)
	require.Equal(t, "success", deleteRsp.Reason)

	// ========================
	// GET /api/subscribers/:name after deletion
	rsp, payload = request(t, jwt, http.MethodGet, "/web/api/subscribers/test-sub", nil)
	require.Equal(t, http.StatusInternalServerError, rsp.StatusCode)
	require.Contains(t, string(payload), "no such file or directory")
}

func TestSshKey(t *testing.T) {
	jwt := HttpTestLogin(t, "sys", "manager")

	// ========================
	// GET /api/sshkeys
	rsp, _ := request(t, jwt, http.MethodGet, "/web/api/sshkeys", nil)
	require.Equal(t, http.StatusOK, rsp.StatusCode)

	// ========================
	// POST /api/sshkeys - add key
	b := &bytes.Buffer{}
	sshKeyReq := map[string]string{
		"key": "ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKMYQpY26gDO9JAK7gFRtM3JR2JiDCLGKsTqVzcQmNvJ your_email@example.com",
	}
	if err := json.NewEncoder(b).Encode(sshKeyReq); err != nil {
		t.Fatal(err)
	}
	rsp, _ = request(t, jwt, http.MethodPost, "/web/api/sshkeys", b)
	require.Equal(t, http.StatusOK, rsp.StatusCode)

	// ========================
	// POST /api/sshkeys - invalid key format
	b = &bytes.Buffer{}
	sshKeyReq = map[string]string{
		"key": "invalidkey",
	}
	if err := json.NewEncoder(b).Encode(sshKeyReq); err != nil {
		t.Fatal(err)
	}
	rsp, _ = request(t, jwt, http.MethodPost, "/web/api/sshkeys", b)
	require.Equal(t, http.StatusBadRequest, rsp.StatusCode)

	// ========================
	// GET /api/sshkeys
	rsp, payload := request(t, jwt, http.MethodGet, "/web/api/sshkeys", nil)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	keysRsp := struct {
		Success bool     `json:"success"`
		Reason  string   `json:"reason"`
		Data    []SshKey `json:"data"`
		Elapse  string   `json:"elapse"`
	}{}
	err := json.Unmarshal(payload, &keysRsp)
	require.NoError(t, err)
	require.True(t, keysRsp.Success)
	require.Len(t, keysRsp.Data, 1)

	// ========================
	// DELETE /api/sshkeys/:fingerprint
	rsp, _ = request(t, jwt, http.MethodDelete, "/web/api/sshkeys/"+keysRsp.Data[0].Fingerprint, nil)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
}
