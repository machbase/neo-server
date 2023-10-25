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
	// conn, err := svr.getTrustConnection(ctx)
	// if err != nil {
	// 	rsp.Reason = err.Error()
	// 	ctx.JSON(http.StatusUnauthorized, rsp)
	// 	return
	// }
	// defer conn.Close()

	for _, data := range req.Values {
		err = svr.lake.appender.Append(data.Tag, data.Ts, data.Val)
		if err != nil {
			rsp.Reason = err.Error()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	}

	rsp.Success = true
	ctx.JSON(http.StatusOK, rsp)
}
