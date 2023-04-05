package httpsvr

import (
	"bytes"
	"encoding/json"
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
	mock.DatabaseClientMock
}

type TestAppenderMock struct {
	mock.AppenderMock
}

func TestAppendRoute(t *testing.T) {

	dbMock := &TestClientMock{}

	dbMock.QueryRowFunc = func(sqlText string, params ...any) spi.Row {
		rm := &mock.RowMock{}

		rm.ScanFunc = func(cols ...any) error {
			t.Logf("cols : %v", cols)
			return nil
		}

		rm.SuccessFunc = func() bool {
			return true
		}
		return rm
	}

	dbMock.AppenderFunc = func(tableName string, opts ...spi.AppendOption) (spi.Appender, error) {
		am := &TestAppenderMock{}
		am.AppendFunc = func(value ...any) error {
			t.Logf("append value : %+v", value)
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

	router := gin.Default()
	wsvr.Route(router)

	var b *bytes.Buffer
	var lakereq lakeReq
	// var lakersp *lakeRsp
	var w *httptest.ResponseRecorder
	var expectStatus int
	var req *http.Request

	// wrong data case - append
	b = &bytes.Buffer{}

	lakereq.TagName = "tag01"
	values := [][]interface{}{
		{time.Now().Format("2006-01-02 15:04:05"), rand.Float64() * 10000},
		{time.Now().Format("2006-01-02 15:04:05"), rand.Float64() * 10000},
	}
	lakereq.Values = values

	// expectStatus = http.StatusInternalServerError
	expectStatus = http.StatusOK
	if err = json.NewEncoder(b).Encode(lakereq); err != nil {
		t.Fatal(err)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/lake/appender", b)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, expectStatus, w.Code, w.Body.String())

}

// func TestMachbaseWrite(t *testing.T) {
// 	neoAddr := "./neo/mach-grpc.sock"
// 	db := machrpc.NewClient()
// 	db.Connect(neoAddr)
// 	svr, err := New(db, &Config{})
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	router := gin.Default()

// 	router.POST("/write", svr.LakeWrite)

// 	t.Fatal(router.Run())
// }

// func TestRequest(t *testing.T) {
// 	client := &http.Client{}
// 	url := "http://127.0.0.1:8080/write"
// 	values := [][]interface{}{
// 		{time.Now().Format("2006-01-02 15:04:05"), rand.Float64() * 10000},
// 		{time.Now().Format("2006-01-02 15:04:05"), rand.Float64() * 10000},
// 		{time.Now().Format("2006-01-02 15:04:05"), rand.Float64() * 10000},
// 		{time.Now().Format("2006-01-02 15:04:05"), rand.Float64() * 10000},
// 	}

// 	lakeReq := make(map[string]interface{})
// 	lakeReq["tagName"] = "tag01"
// 	lakeReq["values"] = values

// 	data, err := json.Marshal(lakeReq)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
// 	req.Header.Set("Content-Type", "application/json")
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	resp, err := client.Do(req)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer resp.Body.Close()

// 	body, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	t.Logf("resp : %s\n", string(body))
// }
