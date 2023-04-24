package httpd

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/do"
	spi "github.com/machbase/neo-spi"
)

type lakeReq struct {
	TagName string          `json:"tagName"`
	Values  [][]interface{} `json:"values"`
}

type lakeRsp struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason,omitempty"`
	Data    string `json:"data,omitempty"`
}

var once sync.Once
var appender spi.Appender

const tableName = "TAG"

func (svr *httpd) handleAppender(ctx *gin.Context) {
	rsp := lakeRsp{Success: false}

	req := lakeReq{}
	err := ctx.Bind(&req)
	if err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusPreconditionFailed, rsp)
		return
	}

	if req.TagName == "" {
		rsp.Reason = "tag name is empty"
		ctx.JSON(http.StatusPreconditionFailed, rsp)
		return
	}

	if req.Values == nil || len(req.Values) == 0 {
		rsp.Reason = "values is nil"
		ctx.JSON(http.StatusPreconditionFailed, rsp)
		return
	}

	// log.Printf("req : %+v\n", req)

	once.Do(func() {
		exists, err := do.ExistsTable(svr.db, tableName)
		if err != nil {
			rsp.Reason = err.Error()
			ctx.JSON(http.StatusPreconditionFailed, rsp)
			return
		}

		if !exists {
			rsp.Reason = fmt.Sprintf("%s table is not exist", tableName)
			ctx.JSON(http.StatusPreconditionFailed, rsp)
			return
		}

		appender, err = svr.db.Appender(tableName)
		if err != nil {
			rsp.Reason = err.Error()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}

		// close 시점 언제?
		defer appender.Close()
	})

	if appender == nil {
		log.Println("appender is nil")
		appender, err = svr.db.Appender(tableName)
		if err != nil {
			rsp.Reason = err.Error()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	}

	dataSet := make([][]interface{}, len(req.Values))
	for idx, value := range req.Values {
		temp := []interface{}{req.TagName}
		// 임시
		t, _ := time.Parse("2006-01-02 15:04:05", value[0].(string))
		value[0] = t
		dataSet[idx] = append(temp, value...)
	}

	//  req.values, data set ([[time, value, ext_value, ...], [time, value, ext_value, ...], ...])
	for _, data := range dataSet {
		err = appender.Append(data...)
		if err != nil {
			rsp.Reason = err.Error()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	}

	rsp.Success = true
	ctx.JSON(http.StatusOK, rsp)
}
