package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestImageFileUpload(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip test on windows")
	}
	logging.Configure(&logging.Config{
		Console:                     true,
		Filename:                    "-",
		Append:                      false,
		DefaultPrefixWidth:          10,
		DefaultEnableSourceLocation: false,
		DefaultLevel:                "TRACE",
	})

	dbMock := &TestImageDBMock{}

	svr, err := NewHttp(dbMock, WithHttpDebugMode(false))
	if err != nil {
		t.Fatal(err)
	}
	router := svr.Router()

	fd, _ := os.Open("test/image.png")

	req, err := buildMultipartFormDataRequest("http://localhost:8080/db/write/EXAMPLE",
		[]string{"NAME", "TIME", "VALUE", "EXTDATA"}, []any{"test", time.Now(), 3.14, fd})
	if err != nil {
		t.Fatal(err)
		return
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	rspBody := w.Body.String()
	require.Equal(t, http.StatusOK, w.Code, rspBody)
	result := map[string]any{}
	if err := json.Unmarshal([]byte(rspBody), &result); err != nil {
		t.Fatal(err)
	}
	require.True(t, result["success"].(bool), rspBody)
	require.Equal(t, "image.png", result["data"].(map[string]any)["files"].(map[string]any)["EXTDATA"].(map[string]any)["FN"].(string), rspBody)
	require.Equal(t, "image/png", result["data"].(map[string]any)["files"].(map[string]any)["EXTDATA"].(map[string]any)["CT"].(string), rspBody)
	require.Equal(t, "/tmp/store", result["data"].(map[string]any)["files"].(map[string]any)["EXTDATA"].(map[string]any)["SD"].(string), rspBody)
	require.Greater(t, result["data"].(map[string]any)["files"].(map[string]any)["EXTDATA"].(map[string]any)["SZ"].(float64), 0.0, rspBody)
	var id uuid.UUID
	err = id.Parse(result["data"].(map[string]any)["files"].(map[string]any)["EXTDATA"].(map[string]any)["ID"].(string))
	require.NoError(t, err, rspBody)
	require.Equal(t, uint8(6), id.Version(), rspBody)
	timestamp, err := uuid.TimestampFromV6(id)
	require.NoError(t, err, rspBody)
	ts, err := timestamp.Time()
	require.NoError(t, err, rspBody)
	require.LessOrEqual(t, ts.UnixNano(), time.Now().UnixNano(), rspBody)
	require.GreaterOrEqual(t, ts.UnixNano(), time.Now().Add(-5*time.Second).UnixNano(), rspBody)
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

func buildMultipartFormDataRequest(url string, names []string, values []any) (*http.Request, error) {
	var ret *http.Request
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for i := range names {
		key := names[i]
		r := values[i]
		h := make(textproto.MIMEHeader)
		var src io.Reader

		if fd, ok := r.(*os.File); ok {
			filename := filepath.Base(fd.Name())
			h.Set("Content-Disposition",
				fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
					escapeQuotes(key), escapeQuotes(filename)))
			h.Set("X-Store-Dir", "/tmp/store")
			if contentType := mime.TypeByExtension(filepath.Ext(filename)); contentType != "" {
				h.Set("Content-Type", contentType)
			} else {
				h.Set("Content-Type", "application/octet-stream")
			}
			defer fd.Close()
			src = fd
		} else {
			h.Set("Content-Disposition",
				fmt.Sprintf(`form-data; name="%s"`, escapeQuotes(key)))
			switch val := r.(type) {
			case string:
				src = bytes.NewBuffer([]byte(val))
			case time.Time:
				src = bytes.NewBuffer([]byte(fmt.Sprintf("%d", val.UnixNano())))
			case float64:
				src = bytes.NewBuffer([]byte(fmt.Sprintf("%f", val)))
			default:
				return nil, fmt.Errorf("unsupported type %T", val)
			}
		}
		if dst, err := w.CreatePart(h); err != nil {
			return nil, err
		} else {
			if _, err := io.Copy(dst, src); err != nil {
				return nil, err
			}
		}
	}
	// Don't forget to close the multipart writer.
	// If you don't close it, your request will be missing the terminating boundary.
	w.Close()

	if req, err := http.NewRequest("POST", url, &b); err != nil {
		return nil, err
	} else {
		ret = req
	}
	// Don't forget to set the content type, this will contain the boundary.
	ret.Header.Set("Content-Type", w.FormDataContentType())
	return ret, nil
}

type TestImageDBMock struct {
	DatabaseMock
}

func (db *TestImageDBMock) Connect(ctx context.Context, options ...api.ConnectOption) (api.Conn, error) {
	return &ConnMock{
		QueryRowFunc: db.QueryRow,
		QueryFunc:    db.Query,
		ExecFunc:     db.Exec,
		CloseFunc:    func() error { return nil },
	}, nil
}

func (db *TestImageDBMock) QueryRow(ctx context.Context, sqlText string, params ...any) api.Row {
	sqlText = makeSingleLine(sqlText)
	if sqlText == `select type from M$SYS_TABLES T, M$SYS_USERS U where U.NAME = ? and U.USER_ID = T.USER_ID AND T.NAME = ?` {
		if len(params) == 2 && params[0] == "SYS" && params[1] == "EXAMPLE" {
			return &RowMock{
				ScanFunc: func(cols ...any) error {
					*(cols[0].(*api.TableType)) = api.TableTypeTag
					return nil
				},
				ErrFunc: func() error { return nil },
			}
		}
	} else if sqlText == `SELECT j.ID as TABLE_ID, j.TYPE as TABLE_TYPE, j.FLAG as TABLE_FLAG, j.COLCOUNT as TABLE_COLCOUNT from M$SYS_USERS u, M$SYS_TABLES j where u.NAME = ? and j.USER_ID = u.USER_ID and j.DATABASE_ID = ? and j.NAME = ?` {
		if len(params) == 3 && params[0] == "SYS" && params[1] == -1 && params[2] == "EXAMPLE" {
			return &RowMock{
				ScanFunc: func(cols ...any) error {
					*(cols[0].(*int64)) = int64(4907)              // table id
					*(cols[1].(*api.TableType)) = api.TableTypeTag // table type
					*(cols[2].(*api.TableFlag)) = 0                // table flag
					*(cols[3].(*int)) = 4                          // column count
					return nil
				},
				ErrFunc: func() error { return nil },
			}
		}
	}

	fmt.Println("MOCK-QueryRow", sqlText, params)
	return nil
}

func (db *TestImageDBMock) Exec(ctx context.Context, sqlText string, params ...any) api.Result {
	sqlText = makeSingleLine(sqlText)
	// because NAME, TIME, VALUE, EXTDATA columns can come in any order
	if strings.HasPrefix(sqlText, `INSERT INTO EXAMPLE`) && strings.HasSuffix(sqlText, `VALUES(?,?,?,?)`) {
		fmt.Println("MOCK-Exec", params)
		return &ResultMock{
			ErrFunc:          func() error { return nil },
			MessageFunc:      func() string { return "success" },
			RowsAffectedFunc: func() int64 { return 1 },
		}
	}
	fmt.Println("MOCK-Exec", sqlText, params)
	return nil
}

func (db *TestImageDBMock) Query(ctx context.Context, sqlText string, params ...any) (api.Rows, error) {
	sqlText = makeSingleLine(sqlText)
	if sqlText == `select name, type, length, id, flag from M$SYS_COLUMNS where table_id = ? AND database_id = ? order by id` {
		if len(params) == 2 && params[0] == int64(4907) && params[1] == -1 {
			return newColumnsMock(), nil
		}
	} else if sqlText == `select name, type, id from M$SYS_INDEXES where table_id = ? AND database_id = ?` {
		if len(params) == 2 && params[0] == int64(4907) && params[1] == -1 {
			return &RowsMock{
				NextFunc:  func() bool { return false },
				CloseFunc: func() error { return nil },
			}, nil
		}
	}
	fmt.Println("MOCK-Query", sqlText, params)
	return nil, nil
}

type columnInfo struct {
	name   string
	typ    api.ColumnType
	length int
	id     uint64
	flag   api.ColumnFlag
}

type columnsMock struct {
	RowsMock
	cursor int
	rows   []columnInfo
}

func newColumnsMock() *columnsMock {
	ret := &columnsMock{
		cursor: 0,
		rows: []columnInfo{
			{"NAME", api.ColumnTypeVarchar, 200, 1, 0},
			{"TIME", api.ColumnTypeDatetime, 8, 2, 1},
			{"VALUE", api.ColumnTypeDouble, 8, 3, 2},
			{"EXTDATA", api.ColumnTypeJSON, 32767, 4, 3},
			{"_RID", api.ColumnTypeLong, 8, 5, 65534},
		},
	}
	ret.NextFunc = func() bool {
		return ret.cursor < len(ret.rows)
	}
	ret.ScanFunc = func(cols ...any) error {
		*(cols[0].(*string)) = ret.rows[ret.cursor].name         // name
		*(cols[1].(*api.ColumnType)) = ret.rows[ret.cursor].typ  // type
		*(cols[2].(*int)) = ret.rows[ret.cursor].length          // length
		*(cols[3].(*uint64)) = ret.rows[ret.cursor].id           // id
		*(cols[4].(*api.ColumnFlag)) = ret.rows[ret.cursor].flag // flag
		ret.cursor++
		return nil
	}
	ret.CloseFunc = func() error { return nil }
	return ret
}

func makeSingleLine(sqlText string) string {
	lines := strings.Split(sqlText, "\n")
	for i, line := range lines {
		str := strings.TrimSpace(line)
		if str == "" {
			continue
		}
		lines[i] = str
	}
	return strings.Join(lines, " ")
}

var (
	LINEPROTOCOLDATA = `cpu,cpu=cpu-total,host=desktop usage_irq=0,usage_softirq=0.004171359446037821,usage_guest=0,usage_user=0.3253660367906774,usage_system=0.0792558294748905,usage_idle=99.59120677410203,usage_guest_nice=0,usage_nice=0,usage_iowait=0,usage_steal=0 1670975120000000000
mem,host=desktop committed_as=8780218368i,dirty=327680i,huge_pages_free=0i,shared=67067904i,sreclaimable=414224384i,total=67377881088i,buffered=810778624i,vmalloc_total=35184372087808i,active=3356581888i,available_percent=95.04513097460023,free=56726638592i,slab=617472000i,available=64039395328i,vmalloc_used=54685696i,cached=7298387968i,inactive=6323064832i,low_total=0i,page_tables=32129024i,high_free=0i,commit_limit=35836420096i,high_total=0i,swap_total=2147479552i,write_back_tmp=0i,write_back=0i,used=2542075904i,swap_cached=0i,vmalloc_chunk=0i,mapped=652132352i,huge_page_size=2097152i,huge_pages_total=0i,low_free=0i,sunreclaim=203247616i,swap_free=2147479552i,used_percent=3.7728641253646424 1670975120000000000
disk,device=nvme0n1p3,fstype=ext4,host=desktop,mode=rw,path=/ total=1967315451904i,free=1823398948864i,used=43906785280i,used_percent=2.3513442109214915,inodes_total=122068992i,inodes_free=121125115i,inodes_used=943877i 1670975120000000000
system,host=desktop n_users=2i,load1=0.08,load5=0.1,load15=0.09,n_cpus=24i 1670975120000000000
system,host=desktop uptime=513536i 1670975120000000000
system,host=desktop uptime_format="5 days, 22:38" 1670975120000000000
processes,host=desktop zombies=0i,unknown=0i,dead=0i,paging=0i,total_threads=1084i,blocked=0i,stopped=0i,running=0i,sleeping=282i,total=426i,idle=144i 1670975120000000000`

	H_LINE_DESC_QUERYROW_SQL = api.SqlTidy(
		`SELECT
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
		and j.NAME = ?`)

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
						*(cols[0].(*int64)) = 0
						*(cols[1].(*api.TableType)) = api.TableTypeTag
						*(cols[2].(*api.TableFlag)) = 0
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
						*(cols[1].(*api.ColumnType)) = api.ColumnTypeVarchar
						*(cols[2].(*int)) = 0
						*(cols[3].(*uint64)) = 0
					case 1:
						*(cols[0].(*string)) = "TYPE"
						*(cols[1].(*api.ColumnType)) = api.ColumnTypeInteger
						*(cols[2].(*int)) = 0
						*(cols[3].(*uint64)) = 0
					case 2:
						*(cols[0].(*string)) = "LENGTH"
						*(cols[1].(*api.ColumnType)) = api.ColumnTypeInteger
						*(cols[2].(*int)) = 0
						*(cols[3].(*uint64)) = 0
					case 3:
						*(cols[0].(*string)) = "ID"
						*(cols[1].(*api.ColumnType)) = api.ColumnTypeInteger
						*(cols[2].(*int)) = 0
						*(cols[3].(*uint64)) = 0
					case 4:
						*(cols[0].(*string)) = "FLAG"
						*(cols[1].(*api.ColumnType)) = api.ColumnTypeInteger
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

	wsvr, err := NewHttp(dbMock,
		WithHttpDebugMode(true),
	)
	if err != nil {
		t.Fatal(err)
	}

	// create router
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
		t.Logf("response code %d expected, got=%d %q\nrequest: %v", expectStatus, w.Code, string(content), LINEPROTOCOLDATA)
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
