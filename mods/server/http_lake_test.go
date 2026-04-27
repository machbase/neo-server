package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/v8/mods/logging"
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

func newTestLakeServer() *httpd {
	gin.SetMode(gin.TestMode)
	return &httpd{log: logging.GetLog("httpd-fake")}
}

func newTestLakeContext(method, target string) (*gin.Context, *httptest.ResponseRecorder) {
	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)
	ctx.Request = httptest.NewRequest(method, target, nil)
	ctx.Set(HTTP_TRACKID, "test-track")
	return ctx, writer
}

func TestLakeHelperBuilders(t *testing.T) {
	require.Equal(t, "SELECT * FROM tag", SqlTidy("\n SELECT * \n FROM tag \n"))
	require.Equal(t, "LIMIT 10", makeLimit("", "10"))
	require.Equal(t, "LIMIT 5, 10", makeLimit("5", "10"))
	require.Equal(t, " AND a AND b ", makeAndCondition("a,b", ",", true))
	require.Equal(t, "TO_DATE('2023-05-16 09:10:20')", makeToDate("2023-05-16T09:10:20"))
	require.Equal(t, " AND NAME IN('alpha','beta')", makeInCondition("NAME", []string{"alpha", "beta"}, true, true))
	require.Equal(t, "'factory.sensor.%'", makeLikeTag("factory.sensor.temp"))
	require.Equal(t, `, "value" AS "value_alias", "level"`, makeValueColumn([]string{" value ", " level "}, []string{"value_alias", ""}))
	require.Equal(t, "TO_TIMESTAMP(TIME/1000000) AS TS", makeTimeColumn("TIME", "ms", "TS"))
	require.Equal(t, "/*+ SCAN_BACKWARD(TAG) */ ", makeScanHint("1", "TAG"))
	require.Equal(t, "SUM(VALUE)", makeCalculator("VALUE", "COUNT"))
	require.Equal(t, "TIME ROLLUP 1 HOUR TIME, AVG(VALUE) VALUE", makeRollupHint("TIME", "day", "AVG", "VALUE"))
}

func TestLakeHelperChecks(t *testing.T) {
	svr := newTestLakeServer()
	ctx, _ := newTestLakeContext(http.MethodGet, "/lakes/values")

	require.Equal(t, "limit param is not number", svr.checkSelectTagLimit(ctx, "abc", 10))
	require.Contains(t, svr.checkSelectTagLimit(ctx, "11", 10), "limit over")
	require.Equal(t, "limit param is not number", svr.checkSelectValueLimit(ctx, "abc", 10))
	require.Contains(t, svr.checkSelectValueLimit(ctx, "11", 10), "limit over")

	timeType, err := svr.checkTimeFormat(ctx, "", true)
	require.NoError(t, err)
	require.Empty(t, timeType)

	timeType, err = svr.checkTimeFormat(ctx, "1710000000", false)
	require.NoError(t, err)
	require.Equal(t, "timestamp", timeType)

	timeType, err = svr.checkTimeFormat(ctx, "2023-05-16.09:10:20.123", false)
	require.NoError(t, err)
	require.Equal(t, "date", timeType)

	_, err = svr.checkTimeFormat(ctx, "123456789", false)
	require.Error(t, err)

	err = svr.checkTimePeriod(ctx, "1710000000", "timestamp", "2023-05-16.09:10:20.123", "date")
	require.Error(t, err)

	require.Equal(t, "1710000000000000000", svr.makeNanoTimeStamp(ctx, "1710000000"))
	require.Equal(t, "FROM_TIMESTAMP(1710000000000000000)", svr.makeFromTimestamp(ctx, "1710000000"))
	require.Empty(t, svr.makeFromTimestamp(ctx, "not-a-timestamp"))
}

func TestLakeMakeReturnFormat(t *testing.T) {
	dbData := &MachbaseResult{
		Columns: []MachbaseColumn{{Name: "NAME"}, {Name: "TIME"}, {Name: "VALUE"}},
		Data: [][]interface{}{
			{"tag1", int64(1), 1.25},
			{"tag1", int64(2), 2.5},
		},
	}

	tagResult := MakeReturnFormat(dbData, "AVG", "0", "tag", []string{"tag1"})
	require.Equal(t, "AVG", tagResult.CalcMode)
	require.Len(t, tagResult.Columns, 2)
	require.Equal(t, "TIME", tagResult.Columns[0].Name)

	tagSamples, ok := tagResult.Samples.([]ReturnData)
	require.True(t, ok)
	require.Len(t, tagSamples, 1)
	require.Equal(t, "tag1", tagSamples[0].TagName)
	tagRows, ok := tagSamples[0].Data.([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, tagRows, 2)
	require.Equal(t, int64(1), tagRows[0]["TIME"])
	require.Equal(t, 1.25, tagRows[0]["VALUE"])

	logResult := MakeReturnFormat(&MachbaseResult{
		Columns: []MachbaseColumn{{Name: "TIME"}, {Name: "VALUE"}},
		Data:    [][]interface{}{{int64(1), 10.5}, {int64(2), 11.5}},
	}, "AVG", "1", "log", nil)

	logSamples, ok := logResult.Samples.([]ReturnDataPivot)
	require.True(t, ok)
	require.Len(t, logSamples, 1)
	logData, ok := logSamples[0].Data.(map[string][]interface{})
	require.True(t, ok)
	require.Equal(t, []interface{}{int64(1), int64(2)}, logData["TIME"])
	require.Equal(t, []interface{}{10.5, 11.5}, logData["VALUE"])

	emptyResult := MakeReturnFormat(&MachbaseResult{Columns: []MachbaseColumn{{Name: "NAME"}}}, "AVG", "0", "tag", []string{"tag1"})
	emptySamples, ok := emptyResult.Samples.([]ReturnData)
	require.True(t, ok)
	require.Empty(t, emptySamples)
}

func TestLakeHandlersRejectInvalidInput(t *testing.T) {
	svr := newTestLakeServer()

	t.Run("unsupported-values-type", func(t *testing.T) {
		ctx, writer := newTestLakeContext(http.MethodGet, "/lakes/values/unknown")
		ctx.Params = gin.Params{{Key: "type", Value: "unknown"}}

		svr.handleLakeGetValues(ctx)

		require.Equal(t, http.StatusBadRequest, writer.Code)
		require.Contains(t, writer.Body.String(), "This type is not available")
	})

	t.Run("invalid-tag-limit", func(t *testing.T) {
		ctx, writer := newTestLakeContext(http.MethodGet, "/lakes/tags?limit=abc")

		svr.handleLakeGetTagList(ctx)

		require.Equal(t, http.StatusPreconditionFailed, writer.Code)
		require.Contains(t, writer.Body.String(), `"status":"fail"`)
	})

	t.Run("current-data-requires-tag-name", func(t *testing.T) {
		ctx, writer := newTestLakeContext(http.MethodGet, "/lakes/values/current")

		svr.GetCurrentData(ctx)

		require.Equal(t, http.StatusUnprocessableEntity, writer.Code)
		require.Contains(t, writer.Body.String(), "tag_name")
	})

	t.Run("raw-data-rejects-invalid-return-type", func(t *testing.T) {
		ctx, writer := newTestLakeContext(http.MethodGet, "/lakes/values/raw?tag_name=tag1&start_time=1710000000&end_time=1710000001&value_return_form=2")

		svr.GetRawData(ctx)

		require.Equal(t, http.StatusPreconditionFailed, writer.Code)
		require.Contains(t, writer.Body.String(), "value_return_form")
	})

	t.Run("raw-data-rejects-mismatched-alias-count", func(t *testing.T) {
		ctx, writer := newTestLakeContext(http.MethodGet, "/lakes/values/raw?tag_name=tag1&start_time=1710000000&end_time=1710000001&columns=value,level&aliases=only_one")

		svr.GetRawData(ctx)

		require.Equal(t, http.StatusUnprocessableEntity, writer.Code)
		require.Contains(t, writer.Body.String(), "columns' and 'aliases'")
	})

	t.Run("calculate-data-rejects-invalid-calc-mode", func(t *testing.T) {
		ctx, writer := newTestLakeContext(http.MethodGet, "/lakes/values/calculated?tag_name=tag1&start_time=1710000000&end_time=1710000001&calc_mode=median")

		svr.GetCalculateData(ctx)

		require.Equal(t, http.StatusUnprocessableEntity, writer.Code)
		require.Contains(t, writer.Body.String(), "calc_mode")
	})

	t.Run("calculate-data-rejects-invalid-interval-type", func(t *testing.T) {
		ctx, writer := newTestLakeContext(http.MethodGet, "/lakes/values/calculated?tag_name=tag1&start_time=1710000000&end_time=1710000001&interval_type=week")

		svr.GetCalculateData(ctx)

		require.Equal(t, http.StatusUnprocessableEntity, writer.Code)
		require.Contains(t, writer.Body.String(), "interval_type")
	})

	t.Run("group-data-requires-tag-name", func(t *testing.T) {
		ctx, writer := newTestLakeContext(http.MethodGet, "/lakes/values/group")

		svr.GetGroupData(ctx)

		require.Equal(t, http.StatusUnprocessableEntity, writer.Code)
		require.Contains(t, writer.Body.String(), "tag name is empty")
	})

	t.Run("group-data-rejects-invalid-calc-mode", func(t *testing.T) {
		ctx, writer := newTestLakeContext(http.MethodGet, "/lakes/values/group?tag_name=tag1&calc_mode=median")

		svr.GetGroupData(ctx)

		require.Equal(t, http.StatusUnprocessableEntity, writer.Code)
		require.Contains(t, writer.Body.String(), "invalid calculate mode")
	})

	t.Run("last-data-rejects-invalid-calc-mode", func(t *testing.T) {
		ctx, writer := newTestLakeContext(http.MethodGet, "/lakes/values/last?tag_name=tag1&calc_mode=median")

		svr.GetLastData(ctx)

		require.Equal(t, http.StatusUnprocessableEntity, writer.Code)
		require.Contains(t, writer.Body.String(), "invalid calculate mode")
	})

	t.Run("stat-data-rejects-invalid-return-type", func(t *testing.T) {
		ctx, writer := newTestLakeContext(http.MethodGet, "/lakes/values/stat?tag_name=tag1&value_return_form=2")

		svr.GetStatData(ctx)

		require.Equal(t, http.StatusPreconditionFailed, writer.Code)
		require.Contains(t, writer.Body.String(), "value_return_form")
	})

	t.Run("pivot-data-rejects-invalid-interpolation", func(t *testing.T) {
		ctx, writer := newTestLakeContext(http.MethodGet, "/lakes/values/pivoted?tag_name=tag1&start_time=1710000000&end_time=1710000001&interpolation=4")

		svr.GetPivotData(ctx)

		require.Equal(t, http.StatusUnprocessableEntity, writer.Code)
		require.Contains(t, writer.Body.String(), "interpolation")
	})

	t.Run("pivot-data-rejects-invalid-direction", func(t *testing.T) {
		ctx, writer := newTestLakeContext(http.MethodGet, "/lakes/values/pivoted?tag_name=tag1&start_time=1710000000&end_time=1710000001&direction=3")

		svr.GetPivotData(ctx)

		require.Equal(t, http.StatusUnprocessableEntity, writer.Code)
		require.Contains(t, writer.Body.String(), "direction")
	})
}
