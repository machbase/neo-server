package httpd

import (
	"context"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
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

	if len(req.Values) == 0 {
		rsp.Reason = "values is empty"
		ctx.JSON(http.StatusPreconditionFailed, rsp)
		return
	}

	// TODO: 1. change it to svr.getUserConnection()
	// TODO: 2. Appender should take care of multiple session and terminated at the end.
	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusUnauthorized, rsp)
		return
	}
	defer conn.Close()

	appender, err := conn.Appender(svr.lake.ctx, "TAG")
	if err != nil {
		svr.log.Error("appender error: ", err)
		return
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

type Query struct {
	Sql string `json:"query"`
}

func (svr *httpd) handleLakeExecQuery(ctx *gin.Context) {
	rsp := ResSet{Status: "fail"}
	query := Query{}

	svr.log.Trace("start ExecQuery()")

	err := ctx.Bind(&query)
	if err != nil {
		svr.log.Info("data bind error: ", err.Error())
		rsp.Data = map[string]interface{}{"title": "data is wrong. check data."}
		ctx.JSON(http.StatusPreconditionFailed, rsp)
		return
	}
	svr.log.Debugf("request data : %+v", query)

	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		rsp.Message = err.Error()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	data, err := svr.getExec(ctx, conn, query.Sql)
	if err != nil {
		svr.log.Info("get data error : ", err.Error())
		rsp.Message = err.Error()
		ctx.JSON(http.StatusFailedDependency, rsp)
		return
	}

	rsp.Status = "success"
	rsp.Data = data

	ctx.JSON(http.StatusOK, rsp)
}

type ExecResult struct {
	Columns      []MachbaseColumn         `json:"columns"`
	Data         []map[string]interface{} `json:"data"`
	ErrorCode    int                      `json:"error_code"`
	ErrorMessage string                   `json:"error_message"`
}

type ExecData struct {
	Name  string  `json:"name"`
	Time  string  `json:"time"`
	Value float64 `json:"value"`
}

func (svr *httpd) getExec(ctx context.Context, conn spi.Conn, sqlText string) (*ResSet, error) {
	resp := &ResSet{}
	rows, err := conn.Query(ctx, sqlText)
	if err != nil {
		return resp, err
	}

	cols, err := rows.Columns()
	if err != nil {
		return resp, err
	}
	colsLen := len(cols.Names())
	colsList := make([]MachbaseColumn, colsLen)

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()

		for idx, col := range cols {
			colsList[idx].Name = col.Name
			// colsList[idx].Type = col.Type
			colsList[idx].Type = ColumnTypeConvert(col.Type)
			colsList[idx].Length = col.Length
		}
	}()

	result := &ExecResult{}
	for rows.Next() { // scale 적용을 어떻게 할 건가, 컬럼 여러개일때 value 컬럼을 찾아서 처리가 가능한가? ( rows.columns 으로 순서 확인 가능? )
		buffer := cols.MakeBuffer()
		err = rows.Scan(buffer...)
		if err != nil {
			svr.log.Warn("scan error : ", err.Error())
			return resp, err
		}

		mv := map[string]any{}
		mv[cols[0].Name] = buffer[0]
		mv[cols[1].Name] = buffer[1]
		mv[cols[2].Name] = buffer[2]
		result.Data = append(result.Data, mv)
	}

	wg.Wait()

	result.Columns = colsList
	resp.Data = result

	return resp, nil
}
