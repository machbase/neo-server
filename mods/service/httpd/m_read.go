package httpd

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (svr *httpd) lakeRead(ctx *gin.Context) {
	rsp := lakeRsp{Success: false, Reason: "not specified"}

	// 기존 lake에서는 cli를 통해서 db 사용
	dataType := ctx.Query("type")

	switch dataType {
	case "raw", "":
		svr.RawData(ctx)
	case "current":
		svr.CurrentData(ctx)
	case "stat":
		svr.StatData(ctx)
	case "calc":
		svr.CalcData(ctx)
	case "pivot":
		svr.PivotData(ctx)
	default:
		rsp.Reason = fmt.Sprintf("unsupported data-type: %s", dataType)
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
}

func (svr *httpd) RawData(ctx *gin.Context) {
	rsp := lakeRsp{Success: false, Reason: "not specified"}

}
func (svr *httpd) CurrentData(ctx *gin.Context) {
	rsp := lakeRsp{Success: false, Reason: "not specified"}

}
func (svr *httpd) StatData(ctx *gin.Context) {
	rsp := lakeRsp{Success: false, Reason: "not specified"}

}
func (svr *httpd) CalcData(ctx *gin.Context) {
	rsp := lakeRsp{Success: false, Reason: "not specified"}

}
func (svr *httpd) PivotData(ctx *gin.Context) {
	rsp := lakeRsp{Success: false, Reason: "not specified"}

}
