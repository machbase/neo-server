package httpd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
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

	req, err := buildMultipartFormDataRequest("http://localhost:8080/db/write/EXAMPLE", map[string]any{
		"NAME":    "test",
		"TIME":    time.Now(),
		"VALUE":   3.14,
		"EXTDATA": fd,
	})
	if err != nil {
		t.Fatal(err)
		return
	}
	req.Header.Set("X-Store-Dir", "/tmp/store")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	rspBody := w.Body.String()
	require.Equal(t, http.StatusOK, w.Code, rspBody)
	t.Log("result>>", rspBody)
}

func buildMultipartFormDataRequest(url string, values map[string]any) (*http.Request, error) {
	var ret *http.Request
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for key, r := range values {
		switch val := r.(type) {
		case *os.File:
			defer val.Close()
			if fw, err := w.CreateFormFile(key, val.Name()); err != nil {
				return nil, err
			} else {
				if _, err := io.Copy(fw, val); err != nil {
					return nil, err
				}
			}
		case string:
			if fw, err := w.CreateFormField(key); err != nil {
				return nil, err
			} else {
				if _, err := fw.Write([]byte(val)); err != nil {
					return nil, err
				}
			}
		case float64:
			if fw, err := w.CreateFormField(key); err != nil {
				return nil, err
			} else {
				if _, err := fw.Write([]byte(fmt.Sprintf("%f", val))); err != nil {
					return nil, err
				}
			}
		case time.Time:
			if fw, err := w.CreateFormField(key); err != nil {
				return nil, err
			} else {
				if _, err := fw.Write([]byte(fmt.Sprintf("%d", val.UnixNano()))); err != nil {
					return nil, err
				}
			}
		default:
			return nil, fmt.Errorf("unsupported type %T", val)
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
					*(cols[0].(*int)) = api.TagTableType
					return nil
				},
			}
		}
	} else if sqlText == `SELECT j.ID as TABLE_ID, j.TYPE as TABLE_TYPE, j.FLAG as TABLE_FLAG, j.COLCOUNT as TABLE_COLCOUNT from M$SYS_USERS u, M$SYS_TABLES j where u.NAME = ? and j.USER_ID = u.USER_ID and j.DATABASE_ID = ? and j.NAME = ?` {
		if len(params) == 3 && params[0] == "SYS" && params[1] == -1 && params[2] == "EXAMPLE" {
			return &RowMock{
				ScanFunc: func(cols ...any) error {
					*(cols[0].(*int)) = 4907             // table id
					*(cols[1].(*int)) = api.TagTableType // table type
					*(cols[2].(*int)) = 0                // table flag
					*(cols[3].(*int)) = 4                // column count
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
	if sqlText == `INSERT INTO EXAMPLE(NAME,TIME,VALUE,EXTDATA) VALUES(?,?,?,?)` {
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
		if len(params) == 2 && params[0] == 4907 && params[1] == -1 {
			return newColumnsMock(), nil
		}
	} else if sqlText == `select name, type, id from M$SYS_INDEXES where table_id = ? AND database_id = ?` {
		if len(params) == 2 && params[0] == 4907 && params[1] == -1 {
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
	typ    int
	length int
	id     uint64
	flag   int
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
			{"NAME", api.VarcharColumnType, 200, 1, 0},
			{"TIME", api.DatetimeColumnType, 8, 2, 1},
			{"VALUE", api.Float64ColumnType, 8, 3, 2},
			{"EXTDATA", api.JsonColumnType, 32767, 4, 3},
			{"_RID", api.Int64ColumnType, 8, 5, 65534},
		},
	}
	ret.NextFunc = func() bool {
		return ret.cursor < len(ret.rows)
	}
	ret.ScanFunc = func(cols ...any) error {
		*(cols[0].(*string)) = ret.rows[ret.cursor].name // name
		*(cols[1].(*int)) = ret.rows[ret.cursor].typ     // type
		*(cols[2].(*int)) = ret.rows[ret.cursor].length  // length
		*(cols[3].(*uint64)) = ret.rows[ret.cursor].id   // id
		*(cols[4].(*int)) = ret.rows[ret.cursor].flag    // flag
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
