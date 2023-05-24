package httpd

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/do"
	spi "github.com/machbase/neo-spi"
)

type Values struct {
	Tag string
	Ts  int64
	Val float64
}
type lakeReq struct {
	Values []*Values `json:"values"`
}

type lakeRsp struct {
	Success bool        `json:"success"`
	Reason  string      `json:"reason"`
	Data    interface{} `json:"data,omitempty"`
}

var once sync.Once
var appender spi.Appender

const TableName = "TAG"

func (svr *httpd) handleLakePostValues(ctx *gin.Context) {
	rsp := lakeRsp{Success: false}

	req := lakeReq{}
	err := ctx.Bind(&req)
	if err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusPreconditionFailed, rsp)
		return
	}

	if req.Values == nil || len(req.Values) == 0 {
		rsp.Reason = "values is nil"
		ctx.JSON(http.StatusPreconditionFailed, rsp)
		return
	}

	// log.Printf("req : %+v\n", req)

	// api 요청 시 table 확인?
	once.Do(func() {
		exists, err := do.ExistsTable(svr.db, TableName)
		if err != nil {
			rsp.Reason = err.Error()
			ctx.JSON(http.StatusPreconditionFailed, rsp)
			return
		}

		if !exists {
			rsp.Reason = fmt.Sprintf("%s table is not exist", TableName)
			ctx.JSON(http.StatusPreconditionFailed, rsp)
			return
		}

		appender, err = svr.db.Appender(TableName)
		if err != nil {
			rsp.Reason = err.Error()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}

		// close 시점 언제?
		// defer appender.Close()
	})

	if appender == nil {
		svr.log.Error("appender is nil")
		appender, err = svr.db.Appender(TableName)
		if err != nil {
			rsp.Reason = err.Error()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	}

	for _, data := range req.Values {
		err = appender.Append(data.Tag, data.Ts, data.Val)
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
