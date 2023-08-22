package httpd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/machbase/neo-server/mods/util/mock"
	spi "github.com/machbase/neo-spi"
	"github.com/stretchr/testify/require"
)

const (
	H_LINE_DESC_QUERYROW_SQL string = `SELECT
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

	H_LINE_DESC_QUERY_SQL string = "select name, type, length, id from M$SYS_COLUMNS where table_id = ? order by id"
)

func TestLineprotocol(t *testing.T) {
	dbMock := &TestClientMock{}

	dbMock.QueryRowFunc = func(sqlText string, params ...any) spi.Row {
		rm := &mock.RowMock{}

		switch sqlText {
		case H_LINE_DESC_QUERYROW_SQL:
			rm.ScanFunc = func(cols ...any) error {
				if len(params) == 3 {
					*(cols[0].(*int)) = 0
					*(cols[1].(*int)) = 6
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

	dbMock.QueryFunc = func(sqlText string, params ...any) (spi.Rows, error) {
		rm := &mock.RowsMock{}
		tCnt := 5
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
				if len(cols) != 4 {
					t.Logf("ColumnCount: %d", len(cols))
				}
				switch cnt - 1 {
				case 0:
					*(cols[0].(*string)) = "NAME"
					*(cols[1].(*int)) = 5
					*(cols[2].(*int)) = 0
					*(cols[3].(*uint64)) = 0
				case 1:
					*(cols[0].(*string)) = "TIME"
					*(cols[1].(*int)) = 6
					*(cols[2].(*int)) = 0
					*(cols[3].(*uint64)) = 0
				case 2:
					*(cols[0].(*string)) = "VALUE"
					*(cols[1].(*int)) = 20
					*(cols[2].(*int)) = 0
					*(cols[3].(*uint64)) = 0
				case 3:
					*(cols[0].(*string)) = "SERVER"
					*(cols[1].(*int)) = 5
					*(cols[2].(*int)) = 0
					*(cols[3].(*uint64)) = 0
				case 4:
					*(cols[0].(*string)) = "LOCAL"
					*(cols[1].(*int)) = 5
					*(cols[2].(*int)) = 0
					*(cols[3].(*uint64)) = 0
				}
				return nil
			}
		default:
			t.Logf("QueryRow sqlText: %s, params:%v", sqlText, params)
		}

		rm.CloseFunc = func() error {
			return nil
		}
		return rm, nil
	}

	webService, err := New(dbMock,
		OptionDebugMode(true),
		OptionHandler("/metrics", HandlerInflux),
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

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/metrics/write?db=tag", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "application/octet-stream")
	router.ServeHTTP(w, req)

	// test := make(map[string]interface{})
	// err = json.Unmarshal(w.Body.Bytes(), &test)
	// if err != nil {
	// 	t.Fatal(err)
	// }

	// fmt.Println(test)

	expectStatus = http.StatusNoContent
	require.Equal(t, expectStatus, w.Code)

	return
}
