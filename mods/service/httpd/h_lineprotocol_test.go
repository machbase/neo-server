package httpd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime/debug"
	"testing"

	"github.com/machbase/neo-server/api"
	"github.com/stretchr/testify/require"
)

const (
	LINEPROTOCOLDATA = `cpu,cpu=cpu-total,host=desktop usage_irq=0,usage_softirq=0.004171359446037821,usage_guest=0,usage_user=0.3253660367906774,usage_system=0.0792558294748905,usage_idle=99.59120677410203,usage_guest_nice=0,usage_nice=0,usage_iowait=0,usage_steal=0 1670975120000000000
mem,host=desktop committed_as=8780218368i,dirty=327680i,huge_pages_free=0i,shared=67067904i,sreclaimable=414224384i,total=67377881088i,buffered=810778624i,vmalloc_total=35184372087808i,active=3356581888i,available_percent=95.04513097460023,free=56726638592i,slab=617472000i,available=64039395328i,vmalloc_used=54685696i,cached=7298387968i,inactive=6323064832i,low_total=0i,page_tables=32129024i,high_free=0i,commit_limit=35836420096i,high_total=0i,swap_total=2147479552i,write_back_tmp=0i,write_back=0i,used=2542075904i,swap_cached=0i,vmalloc_chunk=0i,mapped=652132352i,huge_page_size=2097152i,huge_pages_total=0i,low_free=0i,sunreclaim=203247616i,swap_free=2147479552i,used_percent=3.7728641253646424 1670975120000000000
disk,device=nvme0n1p3,fstype=ext4,host=desktop,mode=rw,path=/ total=1967315451904i,free=1823398948864i,used=43906785280i,used_percent=2.3513442109214915,inodes_total=122068992i,inodes_free=121125115i,inodes_used=943877i 1670975120000000000
system,host=desktop n_users=2i,load1=0.08,load5=0.1,load15=0.09,n_cpus=24i 1670975120000000000
system,host=desktop uptime=513536i 1670975120000000000
system,host=desktop uptime_format="5 days, 22:38" 1670975120000000000
processes,host=desktop zombies=0i,unknown=0i,dead=0i,paging=0i,total_threads=1084i,blocked=0i,stopped=0i,running=0i,sleeping=282i,total=426i,idle=144i 1670975120000000000`

	H_LINE_DESC_QUERYROW_SQL = `SELECT
			j.ID as TABLE_ID,
			j.TYPE as TABLE_TYPE,
			j.FLAG as TABLE_FLAG,
			j.COLCOUNT as TABLE_COLCOUNT
		from
			M$SYS_USERS u,
			M$SYS_TABLES j
		where
			u.NAME = ?
		and j.USER_ID = u.USER_ID
		and j.DATABASE_ID = ?
		and j.NAME = ?`

	H_LINE_DESC_QUERY_SQL = "select name, type, length, id, flag from M$SYS_COLUMNS where table_id = ? AND database_id = ? order by id"

	H_LINE_DESC_INDEXES_SQL = "select name, type, id from M$SYS_INDEXES where table_id = ? AND database_id = ?"
)

func TestLineprotocol(t *testing.T) {
	columnDefaultLen := 4
	dbMock := &TestClientMock{}
	dbMock.ConnectFunc = func(ctx context.Context, options ...api.ConnectOption) (api.Conn, error) {
		conn := &ConnMock{}
		conn.CloseFunc = func() error { return nil }
		conn.QueryRowFunc = func(ctx context.Context, sqlText string, params ...any) api.Row {
			rm := &RowMock{}

			switch sqlText {
			case H_LINE_DESC_QUERYROW_SQL:
				rm.ScanFunc = func(cols ...any) error {
					if len(params) == 3 {
						*(cols[0].(*int)) = 0
						*(cols[1].(*int)) = int(api.TagTableType)
						*(cols[2].(*int)) = 0
						*(cols[3].(*int)) = 0
					}
					return nil
				}

				rm.ErrFunc = func() error {
					return nil
				}
			default:
				t.Logf("QueryRow sqlText: %s, params:%v", sqlText, params)
			}
			return rm
		}
		conn.QueryFunc = func(ctx context.Context, sqlText string, params ...any) (api.Rows, error) {
			rm := &RowsMock{}
			tCnt := columnDefaultLen
			cnt := 0

			switch sqlText {
			case H_LINE_DESC_QUERY_SQL:
				rm.NextFunc = func() bool {
					if tCnt != cnt {
						cnt++
						return true
					} else {
						return false
					}
				}

				rm.ScanFunc = func(cols ...any) error {
					if len(cols) != 5 {
						t.Logf("ERROR ColumnCount, expect: 5, actual: %d", len(cols))
						debug.PrintStack()
						return errors.New("invalid column count")
					}
					// name, type, length, id, flag
					switch cnt - 1 {
					case 0:
						*(cols[0].(*string)) = "NAME"
						*(cols[1].(*int)) = int(api.VarcharColumnType)
						*(cols[2].(*int)) = 0
						*(cols[3].(*uint64)) = 0
					case 1:
						*(cols[0].(*string)) = "TYPE"
						*(cols[1].(*int)) = int(api.Int32ColumnType)
						*(cols[2].(*int)) = 0
						*(cols[3].(*uint64)) = 0
					case 2:
						*(cols[0].(*string)) = "LENGTH"
						*(cols[1].(*int)) = int(api.Int32ColumnType)
						*(cols[2].(*int)) = 0
						*(cols[3].(*uint64)) = 0
					case 3:
						*(cols[0].(*string)) = "ID"
						*(cols[1].(*int)) = int(api.Int32ColumnType)
						*(cols[2].(*int)) = 0
						*(cols[3].(*uint64)) = 0
					case 4:
						*(cols[0].(*string)) = "FLAG"
						*(cols[1].(*int)) = int(api.Int32ColumnType)
						*(cols[2].(*int)) = 0
						*(cols[3].(*uint64)) = 0
					}
					return nil
				}
			case H_LINE_DESC_INDEXES_SQL:
				rm.NextFunc = func() bool { return false }
			default:
				t.Logf("QueryRow sqlText: %s, params:%v", sqlText, params)
			}

			rm.CloseFunc = func() error {
				return nil
			}
			return rm, nil
		}
		conn.ExecFunc = func(ctx context.Context, sqlText string, params ...any) api.Result {
			var failed bool
			var expected int
			if sqlText == "INSERT INTO tag(NAME,TYPE,LENGTH) VALUES(?,?,?)" {
				expected = 3
				failed = len(params) != expected
			} else {
				fmt.Println("========>", sqlText)
				if len(params) != columnDefaultLen {
					expected = columnDefaultLen
					failed = true
				}
			}
			if failed {
				t.Logf("ERROR column len different, expect: %d, actual: %d\nSQL:%s", expected, len(params), sqlText)
				t.Fail()
				debug.PrintStack()
				return nil
			}
			rm := &ResultMock{}
			rm.ErrFunc = func() error {
				return nil
			}
			return rm
		}
		return conn, nil
	}

	webService, err := New(dbMock,
		OptionDebugMode(true),
	)
	if err != nil {
		t.Fatal(err)
	}

	// create router
	wsvr := webService.(*httpd)
	router := wsvr.Router()

	var w *httptest.ResponseRecorder
	var req *http.Request
	var expectStatus int

	// success case - line protocol
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/metrics/write?db=tag", bytes.NewBufferString(LINEPROTOCOLDATA))
	req.Header.Set("Content-Type", "application/octet-stream")
	router.ServeHTTP(w, req)

	expectStatus = http.StatusNoContent
	if expectStatus != w.Code {
		content, _ := io.ReadAll(w.Result().Body)
		t.Logf("respose code %d expected, got=%d %q\nrequest: %v", expectStatus, w.Code, string(content), LINEPROTOCOLDATA)
		t.FailNow()
	}

	//wrong case - wrong request
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/metrics/read?db=tag", bytes.NewBufferString(LINEPROTOCOLDATA))
	req.Header.Set("Content-Type", "application/octet-stream")
	router.ServeHTTP(w, req)

	expectStatus = http.StatusNotImplemented
	require.Equal(t, expectStatus, w.Code)

	//wrong case - gzip wrong request
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/metrics/write?db=tag", bytes.NewBufferString(LINEPROTOCOLDATA))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Encoding", "gzip")
	router.ServeHTTP(w, req)

	expectStatus = http.StatusBadRequest
	require.Equal(t, expectStatus, w.Code)

	//wrong case - time format wrong request
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/metrics/write?db=tag&precision=ms", bytes.NewBufferString(LINEPROTOCOLDATA))
	req.Header.Set("Content-Type", "application/octet-stream")
	router.ServeHTTP(w, req)

	expectStatus = http.StatusBadRequest
	require.Equal(t, expectStatus, w.Code)
}
