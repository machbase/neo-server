package httpd

import (
	"fmt"
	"net/http"
	"strings"
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
			svr.log.Info("append error : ", err.Error())
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	}

	rsp.Success = true
	ctx.JSON(http.StatusOK, rsp)
}

//=================================

type queryRequest struct {
	edgeId    string
	startTime string
	endTime   string
	offset    int
	limit     int
	level     int
	job       string
	keyword   string
	tableName string
}

type queryResponse struct {
	Success bool     `json:"success"`
	Reason  string   `json:"reason,omitempty"`
	Columns []string `json:"columns"`
	Data    []any    `json:"data"`
}

type queryRow struct {
	EdgeId   string `json:"EDGEID"`
	Time     string `json:"TIME"`
	FileName string `json:"FILENAME"`
	Job      string `json:"JOB"`
	Level    int    `json:"LEVEL"`
	Line     string `json:"LINE"`
}

func (svr *httpd) handleLakeGetLogs(ctx *gin.Context) {
	rsp := queryResponse{Success: false}
	req := queryRequest{}

	if ctx.Request.Method == http.MethodGet {
		req.edgeId = ctx.Query("edgeid")
		req.startTime = ctx.Query("startTime")
		req.endTime = ctx.Query("endTime")
		req.level = strInt(ctx.Query("level"), 0)
		req.limit = strInt(ctx.Query("limit"), 0)
		req.offset = strInt(ctx.Query("offset"), 0)
		req.job = ctx.Query("job")
		req.keyword = ctx.Query("keyword") //  % -> URL escape code '%25'
		req.tableName = ctx.Query("tablename")
	} else {
		rsp.Reason = fmt.Sprintf("unsupported method %s", ctx.Request.Method)
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	if req.tableName == "" {
		rsp.Reason = "table name is empty"
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	// check table existence ? or just use fixed table.
	exists, _ := do.ExistsTable(svr.db, req.tableName)
	if !exists {
		rsp.Reason = fmt.Sprintf("%q table does not exist.", req.tableName)
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	sqlText := fmt.Sprintf("SELECT * FROM %s WHERE ", req.tableName)
	queryLen := len(ctx.Request.URL.Query())
	if queryLen == 2 { // tableName, limit
		limit := ctx.Request.URL.Query().Get("limit")
		if limit != "" {
			sqlText = fmt.Sprintf("SELECT * FROM %s ", req.tableName)
		}
	} else if queryLen == 1 {
		sqlText = fmt.Sprintf("SELECT * FROM %s", req.tableName)
	}

	params := []any{}
	andFlag := false
	if req.edgeId != "" {
		sqlText += "edgeid = ?"
		params = append(params, req.edgeId)
		andFlag = true
	}
	if req.startTime != "" {
		if andFlag {
			sqlText += " AND "
		}
		sqlText += "time >= ?"
		params = append(params, req.startTime)
		andFlag = true
	}
	if req.endTime != "" {
		if andFlag {
			sqlText += " AND "
		}
		sqlText += "time <= ?"
		params = append(params, req.endTime)
		andFlag = true
	}
	if req.level >= 1 && req.level <= 5 {
		if andFlag {
			sqlText += " AND "
		}
		sqlText += "level = ?"
		params = append(params, req.level)
		andFlag = true
	}
	if req.job != "" {
		if andFlag {
			sqlText += " AND "
		}
		sqlText += "job = ?"
		params = append(params, req.job)
		andFlag = true
	}
	if req.keyword != "" {
		if andFlag {
			sqlText += " AND "
		}
		if strings.Contains(req.keyword, "%") {
			sqlText += "line esearch ?"
		} else {
			sqlText += "line search ?"
		}
		params = append(params, req.keyword)
		andFlag = true
	}
	if andFlag {
		sqlText += " "
	}
	if req.offset > 0 {
		sqlText += "limit ?, ?"
		params = append(params, req.offset)
		params = append(params, req.limit)
	} else if req.limit > 0 {
		sqlText += "limit ?"
		params = append(params, req.limit)
	}

	rows, err := svr.db.Query(sqlText, params...)
	if err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	cols, err := rows.Columns()
	if err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	} else {
		rsp.Columns = cols.Names()
	}

	defer rows.Close()

	for rows.Next() {
		row := queryRow{}
		err = rows.Scan(&row.EdgeId, &row.Time, &row.FileName, &row.Job, &row.Level, &row.Line)
		if err != nil {
			rsp.Reason = err.Error()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		rsp.Data = append(rsp.Data, row)
	}

	rsp.Success = true
	ctx.JSON(http.StatusOK, rsp)
}
