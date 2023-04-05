package httpsvr

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
)

type selectRaw struct {
	Timezone     string `form:"timezone" json:"timezone"`
	TagName      string `form:"tag_name" json:"tag_name"`
	DateFormat   string `form:"date_format" json:"date_format"`
	StartTime    string `form:"start_time" json:"start_time"`
	EndTime      string `form:"end_time" json:"end_time"`
	Columns      string `form:"columns" json:"columns"`
	AndCondition string `form:"and_condition" json:"and_condition"`
	Separator    string `form:"separator" json:"separator"`
	Alias        string `form:"aliases" json:"aliases"`
	Limit        string `form:"limit" json:"limit"`
	Offset       string `form:"offset" json:"offset"`
	Direction    string `form:"direction" json:"direction"`
	ReturnType   string `form:"value_return_form" json:"value_return_form"`
	Scale        int    `form:"scale" json:"scale"`
	StartType    string
	EndType      string
	TagList      []string
	ColumnList   []string
	AliasList    []string
}

func (svr *Server) lakeRead(ctx *gin.Context) {
	rsp := lakeRsp{Success: false}

	dataType := ctx.Query("type")

	switch dataType {
	case "raw", "":
		svr.selectRawData(ctx)
	case "current":
		svr.selectCurrentData(ctx)
	case "stat":
		svr.selectStatData(ctx)
	case "calc":
		svr.selectCalcData(ctx)
	case "pivot":
		svr.selectPivotData(ctx)
	default:
		rsp.Reason = fmt.Sprintf("invalid type : %v", dataType)
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
	}

	rsp.Success = true
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *Server) selectRawData(ctx *gin.Context) {
	rsp := lakeRsp{Success: false}

	req := selectRaw{}
	err := ctx.ShouldBind(&req)
	if err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	// request에 있는 timezone을 체크하고 machbase에 사용 가능한 timezone으로 변경 후 반환하는 함수
	makeTimezone(ctx, req.Timezone)

}

func makeTimezone(ctx *gin.Context, timezone string) (string, error) {
	// 빈 값일 경우 디폴트 timezone으로 대체
	if timezone == "" {
		timezone = "+0000" //  default인지 정확하지 않음  +0000, Etc/UTC 둘 중 하나
		return timezone, nil
	}

	validTimezone := regexp.MustCompile(`[+-](0[0-9]|1[0-4])[0-5][0-9]$`)
	if validTimezone.MatchString(timezone) {
		return timezone, nil
	}

	return convertTimezone(ctx, timezone), nil
}

func convertTimezone(ctx *gin.Context, timezone string) string {

	return ""
}

func (svr *Server) selectCurrentData(ctx *gin.Context) {

}

func (svr *Server) selectStatData(ctx *gin.Context) {

}

func (svr *Server) selectCalcData(ctx *gin.Context) {

}

func (svr *Server) selectPivotData(ctx *gin.Context) {

}
