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

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestHttp(t *testing.T) {
	tableName := strings.ToUpper("test_tc")
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
	q = url.QueryEscape(fmt.Sprintf("create tag table %s (name varchar(200) primary key, time datetime basetime, value double summarized, jsondata json, ival int, sval short)", tableName))
	rsp, err = client.Get(baseURL + "/db/query?q=" + q)
	require.Nil(t, err)
	require.Equal(t, 200, rsp.StatusCode)

	//// insert
	row1 := fmt.Sprintf(`["test_1",%d,1.12,null,101,102]`, time.Now().Unix())
	row2 := fmt.Sprintf(`["test_1",%d,2.23,null,201,202]`, time.Now().Unix()+1)
	rsp, err = client.Post(baseURL+"/db/write/"+tableName+"?timeformat=s", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"data":{"columns:":["name","time","value","jsondata","ival","sval"],"rows":[%s,%s]}}`, row1, row2)))
	require.Nil(t, err)
	content, _ = io.ReadAll(rsp.Body)
	rsp.Body.Close()
	require.Equal(t, 200, rsp.StatusCode)
	require.True(t, gjson.Get(string(content), "success").Bool())
	time.Sleep(1 * time.Second)

	//// query
	rsp, err = client.Get(baseURL + "/db/query?q=" + url.QueryEscape(fmt.Sprintf("select * from %s where name='test_1'", tableName)))
	require.Nil(t, err)
	content, _ = io.ReadAll(rsp.Body)
	rsp.Body.Close()
	require.Equal(t, 200, rsp.StatusCode)
	require.Equal(t, int64(2), gjson.Get(string(content), "data.rows.#").Int())
	require.Equal(t, "test_1", gjson.Get(string(content), "data.rows.0.0").String())
	require.Equal(t, "test_1", gjson.Get(string(content), "data.rows.1.0").String())
	require.Equal(t, int64(101), gjson.Get(string(content), "data.rows.0.4").Int())
	require.Equal(t, int64(102), gjson.Get(string(content), "data.rows.0.5").Int())
	require.Equal(t, int64(201), gjson.Get(string(content), "data.rows.1.4").Int())
	require.Equal(t, int64(202), gjson.Get(string(content), "data.rows.1.5").Int())

	//// lineprotocol
	// lineStr := `sample.tag name="gauge",value=3.003 1670380345000000`
	rsp, err = client.Post(baseURL+"/metrics/write?db="+tableName, "application/octet-stream", bytes.NewBufferString(lineProtocolData))
	require.Nil(t, err)
	if rsp.StatusCode != http.StatusNoContent {
		t.Log(rsp)
	}
	require.Equal(t, http.StatusNoContent, rsp.StatusCode)

	//// drop table
	q = url.QueryEscape(fmt.Sprintf("drop table %s", tableName))
	rsp, err = client.Get(baseURL + "/db/query?q=" + q)
	require.Nil(t, err)
	require.Equal(t, 200, rsp.StatusCode)

}
