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

var notBefore_UNIX int64 = 1712027952 //2024-04-02 03:19:12 +0000 UTC
var notAfter_UNIX int64 = 1712071152  //2024-04-02 03:19:12 +0000 UTC

func (mock *mgmtServerMock) ListKey(context.Context, *mgmt.ListKeyRequest) (*mgmt.ListKeyResponse, error) {
	rsp := &mgmt.ListKeyResponse{
		Success: true,
		Keys: []*mgmt.KeyInfo{
			{
				Id:        "eleven",
				NotBefore: notBefore_UNIX,
				NotAfter:  notAfter_UNIX,
			},
		},
	}
	return rsp, nil
}

func (mock *mgmtServerMock) GenKey(context.Context, *mgmt.GenKeyRequest) (*mgmt.GenKeyResponse, error) {
	rsp := &mgmt.GenKeyResponse{
		Success:     true,
		Certificate: "-----BEGIN CERTIFICATE-----\nMIICizCCAeygAwIBAgIRAI2sBSHY62va4mfSQ0Os0tQwCgYIKoZIzj0EAwQwgZIx\nCzAJBgNVBAYTAkNBMREwDwYDVQQHEwhTYW4gSm9zZTEdMBsGA1UECQwUMzAwMyBO\nIEZpcnN0IFN0ICMyMDYxDjAMBgNVBBETBTk1MTM0MRUwEwYDVQQKEwxtYWNoYmFz\nZS5jb20xEzARBgNVBAsMClImRCBDZW50ZXIxFTATBgNVBAMTDG1hY2hiYXNlLW5l\nbzAeFw0yNDA0MDIwNTEzNDdaFw0zNDAzMzEwNTEzNDdaMBExDzANBgNVBAMTBmVs\nZXZlbjCBmzAQBgcqhkjOPQIBBgUrgQQAIwOBhgAEAG8vIB/z4xMSi5dzFr2O9VwA\ncv1kszR5PXw8Eg08FpkpdzcVjPcWnDZOv+MmFrNJXT52l++q256px9IH4uQN7B3G\nAZGm63QUQ9ShJ8OMsOfmkvjkOH+WYeryCuV/OlLJ2QvnVpV7tBHfSrw0Kp3FUHEc\nHqNCLW1t1PeQ5HZVQzdjbvAPo2AwXjAOBgNVHQ8BAf8EBAMCBaAwHQYDVR0lBBYw\nFAYIKwYBBQUHAwIGCCsGAQUFBwMBMAwGA1UdEwEB/wQCMAAwHwYDVR0jBBgwFoAU\nZR2JTtPW8ZfWY9Yw4ZZWf0sDhzAwCgYIKoZIzj0EAwQDgYwAMIGIAkIBw8vVm7KC\njvyt0dzrFmBRwbfzNLBcSg2O08uPEha5UfWPIFjaGCcHGpM1UvfED+JJ+QO3TtQj\nfZlySUg2nRmnWl0CQgGaUQcEv92rBpnhQB0ztfnVeGzflsrAFcFnrfvzdwAGhSpC\n2mqHWroxAjDCFZdISIqUhLUt066xbfnern061SVoKw==\n-----END CERTIFICATE-----\n",
		Key:         "-----BEGIN EC PRIVATE KEY-----\nMIHcAgEBBEIApcmNWwSDuzkts5uZR8JkLyFxeOTf4JJvMIxnJ9Q1eSqPzRuheGY1\n5yq5XxeSWa1PlcZStJYQjdrlBlUtnXdaHm2gBwYFK4EEACOhgYkDgYYABABvLyAf\n8+MTEouXcxa9jvVcAHL9ZLM0eT18PBINPBaZKXc3FYz3Fpw2Tr/jJhazSV0+dpfv\nqtueqcfSB+LkDewdxgGRput0FEPUoSfDjLDn5pL45Dh/lmHq8grlfzpSydkL51aV\ne7QR30q8NCqdxVBxHB6jQi1tbdT3kOR2VUM3Y27wDw==\n-----END EC PRIVATE KEY-----\n",
		Token:       "eleven:b:30818702410f7f6a6ebb8fe9a95c6f439a090a14c6ecaec146a909a5a33285202112155a02cefbf5d46f323d1dd99e3806ee3db0005c9d6976bf49cc0dced0b1163acdaaa47f024201707cb9d80b17ebae3599a1896f628a9ff51cff2f7c20a5b4886040faed0dd327c8133d7a8aac7f76debb509c01fb1dc91b971be25aa0076a6a8ca1cb292d8608c7",
	}

	return rsp, nil
}

func (mock *mgmtServerMock) DelKey(context.Context, *mgmt.DelKeyRequest) (*mgmt.DelKeyResponse, error) {
	rsp := &mgmt.DelKeyResponse{
		Success: true,
	}
	return rsp, nil
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
	req, err = http.NewRequest("GET", "/web/api/keys/eleven", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	router.ServeHTTP(w, req)

	rsp := struct {
		Success bool       `json:"success"`
		Reason  string     `json:"reason"`
		List    [][]string `json:"list"`
		Elapse  string     `json:"elapse"`
	}{}
	payload := w.Body.Bytes()
	err = json.Unmarshal(payload, &rsp)
	if err != nil {
		t.Log("rsp", string(payload))
		t.Fatal(err)
	}

	expectStatus = http.StatusOK
	require.Equal(t, expectStatus, w.Code, rsp)

	expectList := rsp.List[0]
	actualList := []string{"0", "eleven", "2024-04-02 03:19:12 +0000 UTC", "2024-04-02 15:19:12 +0000 UTC"}
	require.Equal(t, expectList, actualList, rsp)

	// ========================
	// POST key-gen
	b = &bytes.Buffer{}

	param := map[string]interface{}{}
	param["name"] = "eleven"
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

	resp := struct {
		Success     bool   `json:"success"`
		Reason      string `json:"reason"`
		Elapse      string `json:"elapse"`
		Certificate string `json:"certificate"`
		PrivateKey  string `json:"privateKey"`
		Token       string `json:"token"`
	}{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil {
		t.Log(w.Body.String())
		t.Fatal(err)
	}

	expectStatus = http.StatusOK
	require.Equal(t, expectStatus, w.Code, resp)

	// ========================
	// DELETE key-delete
	w = httptest.NewRecorder()
	req, err = http.NewRequest("GET", "/web/api/keys/eleven", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.AccessToken()))
	router.ServeHTTP(w, req)

	rsp = struct {
		Success bool       `json:"success"`
		Reason  string     `json:"reason"`
		List    [][]string `json:"list"`
		Elapse  string     `json:"elapse"`
	}{}
	err = json.Unmarshal(w.Body.Bytes(), &rsp)
	if err != nil {
		t.Fatal(err)
	}

	expectStatus = http.StatusOK
	require.Equal(t, expectStatus, w.Code, rsp)
}
