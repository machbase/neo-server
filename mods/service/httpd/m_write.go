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

func (svr *httpd) handleLakePostValues(ctx *gin.Context) {
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

//=================================

type queryRequest struct {
	EdgeId    string
	StartTime string
	EndTime   string
	Offset    string
	Limit     string
	Level     string
	Job       string
	Keyword   string
	//filename string
}

type queryResponse struct {
	Success bool     `json:"success"`
	Reason  string   `json:"reason,omitempty"`
	Lines   []string `json:"lines"`
}

func (svr *httpd) handleLakeGetLogs(ctx *gin.Context) {
	rsp := queryResponse{Success: false}

	req := queryRequest{}
	if ctx.Request.Method == http.MethodGet {
		req.EdgeId = ctx.Query("edgeId")
		req.StartTime = ctx.Query("startTime") // strString() -> default?
		req.EndTime = ctx.Query("endTime")
		req.Level = ctx.Query("level")
		req.Limit = ctx.Query("limit")
		req.Offset = ctx.Query("offset")
		req.Job = ctx.Query("job")
		req.Keyword = ctx.Query("keyword")
	} else {
		rsp.Reason = fmt.Sprintf("unsupported method %s", ctx.Request.Method)
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	// chaeck table existence ? or just use fixed table.
	// exists, err := do.ExistsTable(svr.db, tableName)

	params := []any{}
	sqlText := "SELECT line FROM logdata WHERE "

	andFlag := false
	if req.EdgeId != "" {
		sqlText += "edgeid = ?"
		params = append(params, req.EdgeId)
		andFlag = true
	}
	if req.StartTime != "" {
		if andFlag {
			sqlText += " AND "
		}
		sqlText += "time > ?"
		params = append(params, req.StartTime)
		andFlag = true
	}
	if req.EndTime != "" {
		if andFlag {
			sqlText += " AND "
		}
		sqlText += "time < ?"
		params = append(params, req.EndTime)
		andFlag = true
	}
	if req.Level != "" {
		if andFlag {
			sqlText += " AND "
		}
		sqlText += "level = ?"
		params = append(params, req.EdgeId)
		andFlag = true
	}
	if req.Job != "" {
		if andFlag {
			sqlText += " AND "
		}
		sqlText += "job = ?"
		params = append(params, req.Job)
		andFlag = true
	}
	if req.Keyword != "" {
		if andFlag {
			sqlText += " AND "
		}
		sqlText += "line search '?'"
		params = append(params, req.Keyword)
		andFlag = false
	}
	if req.Limit != "" {
		if andFlag {
			sqlText += " "
		}
		if req.Offset != "" {
			sqlText += "limit ?, ?"
			params = append(params, req.Offset)
			params = append(params, req.Limit)
		} else {
			sqlText += "limit ?"
			params = append(params, req.Limit)
		}
	}

	rows, err := svr.db.Query(sqlText, params)
	if err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	for rows.Next() {
		line := ""
		err = rows.Scan(&line)
		if err != nil {
			rsp.Reason = err.Error()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		rsp.Lines = append(rsp.Lines, line)
	}

	rsp.Success = true
	ctx.JSON(http.StatusOK, rsp)
}
