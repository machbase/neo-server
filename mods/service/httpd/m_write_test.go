package httpd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	spi "github.com/machbase/neo-spi"
	"github.com/stretchr/testify/require"
)

type TestClientMock struct {
	DatabaseMock
}

type TestAppenderMock struct {
	AppenderMock
}

func TestAppendRoute(t *testing.T) {
	columnDefaultLen := 3

	dbMock := &TestClientMock{}
	dbMock.ConnectFunc = func(ctx context.Context, options ...spi.ConnectOption) (spi.Conn, error) {
		conn := &ConnMock{}
		conn.CloseFunc = func() error { return nil }
		conn.QueryRowFunc = func(ctx context.Context, sqlText string, params ...any) spi.Row {
			rm := &RowMock{}

			switch sqlText {
			case "select count(*) from M$SYS_TABLES where name = ?":
				rm.ScanFunc = func(cols ...any) error {
					if len(params) == 1 {
						if params[0] == "TAG" {
							*(cols[0].(*int)) = 1
						} else {
							*(cols[0].(*int)) = 0
						}
					}
					return nil
				}
			default:
				t.Logf("QueryRow sqlText: %s, params:%v", sqlText, params)
			}
			return rm
		}
		conn.AppenderFunc = func(ctx context.Context, tableName string, opts ...spi.AppendOption) (spi.Appender, error) {
			am := &TestAppenderMock{}
			am.AppendFunc = func(value ...any) error {
				count := 0
				for _, val := range value {
					switch v := val.(type) {
					case string:
						if v == "" {
							break
						}
						count++
					case int64:
						if v == 0 {
							break
						}
						count++
					case float64:
						if v == 0 {
							break
						}
						count++
					}
				}

				if count != columnDefaultLen {
					return errors.New("values and number of columns do not match")
				}

				return nil
			}
			am.CloseFunc = func() (int64, int64, error) {
				return 1, 1, nil
			}
			return am, nil
		}
		return conn, nil
	}

	webService, err := New(dbMock,
		OptionDebugMode(true),
		OptionHandler("/lake", HandlerLake),
	)
	if err != nil {
		t.Fatal(err)
	}

	// ========== ========== ========== ==========
	// current table column ( string, time.Time, float64 )

	wsvr := webService.(*httpd)
	router := wsvr.Router()

	var b *bytes.Buffer
	var lakereq lakeDefaultReq
	var lakersp lakeRsp
	var expectStatus int
	var w *httptest.ResponseRecorder
	var req *http.Request

	// success case - append
	b = &bytes.Buffer{}

	lakereq.Values = make([]*lakeDefaultValue, 0)
	values := &lakeDefaultValue{}
	values.Tag = "tag1"
	values.Ts = time.Now().UnixNano()
	values.Val = 11.11

	lakereq.Values = append(lakereq.Values, values)
	if err = json.NewEncoder(b).Encode(lakereq); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/lake/values", b)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	err = json.Unmarshal(w.Body.Bytes(), &lakersp)
	if err != nil {
		t.Log(w.Body.String())
		t.Fatal(err)
	}

	expectStatus = http.StatusOK
	require.Equal(t, expectStatus, w.Code, lakersp)

	// success case - append (standard)
	b = &bytes.Buffer{}

	stdReq := &lakeStandardReq{}
	stdReq.Values = make([]lakeStandardValue, 0)
	stdReq.Tag = "tag1"
	stdReq.Dateformat = "YYYY-MM-DD HH24:MI:SS mmm:uuu:nnn"
	stdReq.Values = append(stdReq.Values, lakeStandardValue{"2023-11-02 00:02:00 000:000:000", 22.969678741091588})
	stdReq.Values = append(stdReq.Values, lakeStandardValue{"2023-11-02 00:02:48 000:000:000", 18.393240581695526})
	if err = json.NewEncoder(b).Encode(stdReq); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/lake/values/standard", b)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	err = json.Unmarshal(w.Body.Bytes(), &lakersp)
	if err != nil {
		t.Fatal(err)
	}

	expectStatus = http.StatusOK
	require.Equal(t, expectStatus, w.Code, lakersp)

	// ====
	//wrong case - append
	lakereq = lakeDefaultReq{}

	b = &bytes.Buffer{}
	values = &lakeDefaultValue{}
	values.Tag = "tag1"
	values.Ts = time.Now().UnixNano()

	lakereq.Values = append(lakereq.Values, values)
	if err = json.NewEncoder(b).Encode(lakereq); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/lake/values", b)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	err = json.Unmarshal(w.Body.Bytes(), &lakersp)
	if err != nil {
		t.Fatal(err)
	}

	expectStatus = http.StatusInternalServerError
	require.Equal(t, expectStatus, w.Code, lakersp)
}
