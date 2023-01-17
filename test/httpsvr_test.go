package test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/machbase/cemlib/logging"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestHttp(t *testing.T) {
	tableName := strings.ToUpper("sample")
	baseURL := "http://127.0.0.1:5654"

	client := http.Client{}

	//// check table exists
	q := url.QueryEscape(fmt.Sprintf("select count(*) from M$SYS_TABLES where name = '%s'", tableName))
	rsp, err := client.Get(baseURL + "/db/query?q=" + q)
	if err != nil {
		panic(err)
	}
	require.Equal(t, 200, rsp.StatusCode)

	content, err := io.ReadAll(rsp.Body)
	require.Nil(t, err)

	str := string(content)
	vSuccess := gjson.Get(str, "success")
	require.True(t, vSuccess.Bool())

	vCount := gjson.Get(str, "data.rows.0.0")

	//// drop table
	if vCount.Int() == 1 {
		q = url.QueryEscape(fmt.Sprintf("drop table " + tableName))
		rsp, err := client.Get(baseURL + "/db/query?q=" + q)
		require.Nil(t, err)
		require.Equal(t, 200, rsp.StatusCode)
	}

	//// create table
	q = url.QueryEscape(fmt.Sprintf("create tag table %s (name varchar(200) primary key, time datetime basetime, value double summarized, jsondata json)", tableName))
	rsp, err = client.Get(baseURL + "/db/query?q=" + q)
	require.Nil(t, err)
	require.Equal(t, 200, rsp.StatusCode)

	//// insert
	// TODO

	//// lineprotocol
	// linestr := `sample.tag name="gauge",value=3.003 1670380345000000`
	rsp, err = client.Post(baseURL+"/metrics/write?db="+tableName, "application/octet-stream", bytes.NewBufferString(lineProtocolData))
	require.Nil(t, err)
	require.Equal(t, http.StatusNoContent, rsp.StatusCode)

	//// logvault
	lvw := logging.NewLogVaultWriter(baseURL+"/logvault/push", map[string]string{"job": "test-log", "host": "localhost"})
	lvw.Start()
	lvw.Write(time.Now(), "some log messages")
	lvw.Stop()
	time.Sleep(1 * time.Second)
}
