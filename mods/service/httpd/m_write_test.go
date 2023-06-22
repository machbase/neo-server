package httpd

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/util/mock"
	spi "github.com/machbase/neo-spi"
	"github.com/stretchr/testify/require"
)

type TestClientMock struct {
	mock.DatabaseMock
}

type TestAppenderMock struct {
	mock.AppenderMock
}

func TestAppendRoute(t *testing.T) {
	columnDefaultLen := 3

	dbMock := &TestClientMock{}

	dbMock.QueryRowFunc = func(sqlText string, params ...any) spi.Row {
		rm := &mock.RowMock{}

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

	dbMock.AppenderFunc = func(tableName string, opts ...spi.AppendOption) (spi.Appender, error) {
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

	webService, err := New(dbMock,
		OptionDebugMode(),
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
	var lakereq lakeReq
	var lakersp lakeRsp
	var expectStatus int
	var w *httptest.ResponseRecorder
	var req *http.Request

	// success case - append
	b = &bytes.Buffer{}

	lakereq.Values = make([]*Values, 0)
	values := &Values{}
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
		t.Fatal(err)
	}

	expectStatus = http.StatusOK
	require.Equal(t, expectStatus, w.Code, lakersp)

	// ====
	//wrong case - append
	lakereq = lakeReq{}

	b = &bytes.Buffer{}
	values = &Values{}
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
