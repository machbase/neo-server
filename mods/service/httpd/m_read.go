package httpd

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

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

	param := SelectRaw{}
	err := ctx.ShouldBind(&param)
	if err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	svr.log.Debugf("request param : %+v", param)

	timezone, err := svr.makeTimezone(ctx, param.Timezone)
	if err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	if param.Separator == "" {
		param.Separator = ","
	}

	// tagname list
	param.TagList = strings.Split(param.TagName, param.Separator)
	if len(param.TagList) > LimitSelTag { // mysql 에서 데이터 로드 필요

	}

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

// 사용자가 보낸 Timezone을 확인하고 machbase에서 사용 가능한 Timezone으로 변경하는 함수
func (svr *httpd) makeTimezone(ctx *gin.Context, timezone string) (string, error) {
	if timezone == "" {
		svr.log.Error("use default timezone 'Etc/UTC'")
		timezone = "Etc/UTC"
	}

	matched := regexp.MustCompile(`[+-](0[0-9]|1[0-4])[0-5][0-9]$`)
	if matched.MatchString(timezone) == true {
		svr.log.Infof("available timezone format : %s", timezone)
		return timezone, nil
	}

	return svr.convertTimezone(ctx, timezone)
}

func (svr *httpd) convertTimezone(ctx *gin.Context, timezone string) (string, error) {
	// convertTimezone 함수만 사용 하는 곳도 존재, 아래 기능이 있으면 makeTimezone 함수와 중복, convert 함수만 사용 가능
	// if timezone == "" {
	// 	return "", fmt.Errorf("timezone is empty")
	// }

	// matched := regexp.MustCompile(`[+-](0[0-9]|1[0-4])[0-5][0-9]$`)
	// if matched.MatchString(timezone) == true {
	// 	return timezone, nil
	// }

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		svr.log.Errorf("load location : %s", timezone)
		return "", err
	}

	sampleDate := time.Date(2021, 1, 1, 12, 0, 0, 0, time.UTC)
	locDate := sampleDate.In(loc).String()
	if len(locDate) < 25 {
		svr.log.Errorf("convert timezone failed : %s", locDate)
		return "", fmt.Errorf("convert timezone failed : %s", locDate)
	}

	machbaseTimezone := locDate[20:25]                                        // ex) +0900, -0900
	svr.log.Debugf("convert timezone (%s -> %s)", timezone, machbaseTimezone) // ex) aTimezone = Asia/Seoul,  sResTimezone = +0900

	return machbaseTimezone, nil
}

type SelectRaw struct {
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
