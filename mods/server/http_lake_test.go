package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHttpLakeAppend(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		input       any
		inputParams url.Values
		expect      string
	}{
		{
			name:   "default",
			method: http.MethodPost,
			path:   "/lakes/values",
			input: &lakeDefaultReq{
				Values: []*lakeDefaultValue{{Tag: "tag1", Ts: testTimeTick.UnixNano(), Val: 11.11}},
			},
			expect: `{"success":true, "reason":"success", "data":{ "fail": 0, "success": 1 }}`,
		},
		{
			name:   "standard",
			method: http.MethodPost,
			path:   "/lakes/values/standard",
			input: &lakeStandardReq{
				Tag:        "tag1",
				Dateformat: "YYYY-MM-DD HH24:MI:SS mmm:uuu:nnn",
				Values: []lakeStandardValue{
					{"2023-11-02 00:02:00 000:000:000", 22.969678741091588},
					{"2023-11-02 00:02:48 000:000:000", 18.393240581695526},
				},
			},
			expect: `{"success":true, "reason":"success", "data":{ "fail": 0, "success": 2 }}`,
		},
		{
			name:   "append_2",
			method: http.MethodPost,
			path:   "/lakes/values",
			input: &lakeDefaultReq{
				Values: []*lakeDefaultValue{{Tag: "tag1", Ts: testTimeTick.UnixNano()}},
			},
			expect: `{"success":true, "reason":"success", "data":{ "fail": 0, "success": 1 }}`,
		},
		{
			name:   "get_calculated",
			method: http.MethodGet,
			path:   "/lakes/values/calculated",
			inputParams: url.Values{
				"tag_name":   []string{"tag1"},
				"start_time": []string{"2024-01-01 09:12:00 000"},
				"end_time":   []string{"2024-12-31 12:12:00 000"},
			},
			expect: `{
				"data": {
					"calc_mode":"AVG",
					"columns":[
						{"length":0, "name":"NAME", "type":5},
						{"length":0, "name":"TIME", "type":5},
						{"length":0, "name":"VALUE", "type":20}
					],
					"samples": [
						{
							"tag_name":"tag1",
							"data": []
						}
					]
				},
				"status":"success"
			}`,
		},
	}

	at, _, err := jwtLogin("sys", "manager")
	require.NoError(t, err)

	creTable := `create tag table tag (
			name varchar(200) primary key,
			time datetime basetime,
			value double summarized)
		WITH ROLLUP(SEC)`
	req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(creTable), nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rsp.StatusCode)
	rsp.Body.Close()

	t.Cleanup(func() {
		dropTable := `DROP TABLE TAG CASCADE`
		req, _ := http.NewRequest(http.MethodGet, httpServerAddress+"/db/query?q="+url.QueryEscape(dropTable), nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", at))
		rsp, _ := http.DefaultClient.Do(req)
		require.Equal(t, http.StatusOK, rsp.StatusCode)
		rsp.Body.Close()
	})

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := &bytes.Buffer{}
			if err := json.NewEncoder(b).Encode(tc.input); err != nil {
				t.Fatal(err)
			}
			address := ""
			if len(tc.inputParams) > 0 {
				address = httpServerAddress + tc.path + "?" + tc.inputParams.Encode()
			} else {
				address = httpServerAddress + tc.path
			}
			req, _ := http.NewRequest(tc.method, address, b)
			req.Header.Set("Content-Type", "application/json")
			rsp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			rspBody, err := io.ReadAll(rsp.Body)
			require.NoError(t, err)
			t.Cleanup(func() { rsp.Body.Close() })
			require.Equal(t, http.StatusOK, rsp.StatusCode, string(rspBody))

			require.JSONEq(t, tc.expect, string(rspBody))
		})
	}
}
