package httpd

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
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid/v5"
	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/logging"
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

	webService, err := New(dbMock, OptionDebugMode(false))
	if err != nil {
		t.Fatal(err)
	}
	var router *gin.Engine
	if svr, ok := webService.(*httpd); ok {
		router = svr.Router()
	}

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
