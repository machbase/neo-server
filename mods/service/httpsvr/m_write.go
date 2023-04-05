package httpsvr

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-shell/do"
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

// var once sync.Once
const tableName = "TAG"

func (svr *Server) handleAppender(ctx *gin.Context) {
	rsp := lakeRsp{Success: false}

	req := lakeReq{}
	err := ctx.Bind(&req)
	if err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusPreconditionFailed, rsp)
		return
	}

	if req.TagName == "" {
		rsp.Reason = "tagName is empty"
		ctx.JSON(http.StatusPreconditionFailed, rsp)
		return
	}

	if req.Values == nil || len(req.Values) == 0 {
		rsp.Reason = "values is nil"
		ctx.JSON(http.StatusPreconditionFailed, rsp)
		return
	}

	log.Println("[Request] : ", req)

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

	appender, err := svr.db.Appender(tableName)
	if err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	defer appender.Close()

	dataSet := make([][]interface{}, len(req.Values))
	for idx, value := range req.Values {
		temp := []interface{}{req.TagName}
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

// once.Do(func() {
// 	sqlText := `
// 	create tag table TAG (
// 		name VARCHAR(40) PRIMARY KEY,
// 		time datetime basetime,
// 		value double
// 	)`

// 	result := svr.db.Exec(sqlText)
// 	if result != nil {
// 		rsp.Reason = result.Err().Error()
// 		ctx.JSON(http.StatusInternalServerError, rsp)
// 		return
// 	}

// 	exists, err := do.ExistsTable(svr.db, tableName)
// 	if err != nil {
// 		rsp.Reason = err.Error()
// 		ctx.JSON(http.StatusPreconditionFailed, rsp)
// 		return
// 	}

// 	if !exists {
// 		rsp.Reason = fmt.Sprintf("%s table is not exist", tableName)
// 		ctx.JSON(http.StatusPreconditionFailed, rsp)
// 		return
// 	}
// })
