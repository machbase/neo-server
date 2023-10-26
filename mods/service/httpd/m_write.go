package httpd

import (
	"net/http"

	"github.com/gin-gonic/gin"
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

	data, err := svr.getData(ctx, conn, query.Sql, 0)
	if err != nil {
		svr.log.Info("get data error : ", err.Error())
		rsp.Message = err.Error()
		ctx.JSON(http.StatusFailedDependency, rsp)
		return
	}

	svr.log.Info("[getData] data ==> %+v\n", data)

	rsp.Status = "success"
	rsp.Data = data

	ctx.JSON(http.StatusOK, rsp)
}
