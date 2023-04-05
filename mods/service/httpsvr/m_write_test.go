package httpsvr

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-shell/util/mock"
	spi "github.com/machbase/neo-spi"
	"github.com/stretchr/testify/require"
)

type TestClientMock struct {
	// mock.DatabaseClientMock
	mock.DatabaseMock
}

type TestAppenderMock struct {
	mock.AppenderMock
}

func TestAppendRoute(t *testing.T) {

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
				switch val.(type) {
				case string, time.Time, float64:
					count++
				}
			}

			if count != len(value) {
				return errors.New("values and number of columns do not match")
			}

			return nil
		}
		am.CloseFunc = func() (int64, int64, error) {
			return 1, 1, nil
		}
		return am, nil
	}

	conf := &Config{
		Handlers: []HandlerConfig{
			{
				Prefix:  "/lake",
				Handler: "lake",
			},
		},
	}

	wsvr, err := New(dbMock, conf)
	if err != nil {
		t.Fatal(err)
	}

	// ========== ========== ========== ==========
	// current table column ( string, time.Time, float64 )

	router := gin.Default()
	wsvr.Route(router)

	var b *bytes.Buffer
	var lakereq lakeReq
	var lakersp lakeRsp
	var expectStatus int
	var w *httptest.ResponseRecorder
	var req *http.Request

	// success case - append
	b = &bytes.Buffer{}

	lakereq.TagName = "tag01"
	values := [][]interface{}{
		{time.Now().Format("2006-01-02 15:04:05"), rand.Float64() * 10000},
	}

	lakereq.Values = values

	expectStatus = http.StatusOK
	if err = json.NewEncoder(b).Encode(lakereq); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/lake/appender", b)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	err = json.Unmarshal(w.Body.Bytes(), &lakersp)
	if err != nil {
		t.Fatal(err)
	}

	require.Equal(t, expectStatus, w.Code, lakersp)

	// wrong case - append
	b = &bytes.Buffer{}

	lakereq.TagName = "tag01"
	values = [][]interface{}{
		// current table column ( string, time.Time, float64 )
		{time.Now().Format("2006-01-02 15:04:05"), rand.Float64() * 10000, true},
	}
	lakereq.Values = values

	expectStatus = http.StatusInternalServerError
	if err = json.NewEncoder(b).Encode(lakereq); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/lake/appender", b)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	err = json.Unmarshal(w.Body.Bytes(), &lakersp)
	if err != nil {
		t.Fatal(err)
	}

	require.Equal(t, expectStatus, w.Code, lakersp)
}
