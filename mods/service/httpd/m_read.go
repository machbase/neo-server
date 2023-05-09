package httpd

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

type planLimit struct {
	maxQuery         int64
	maxStorage       int64
	maxNetwork       int64
	maxTagCount      int64
	maxConcurrent    int
	limitSelectTag   int
	limitSelectValue int64
	limitAppendTag   int64
	limitAppendValue int64
	defaultTagCount  int64
}

const (
	MACHLAKE_PLAN_TINY       = "TINY"
	MACHLAKE_PLAN_BASIC      = "BASIC"
	MACHLAKE_PLAN_BUSINESS   = "BUSINESS"
	MACHLAKE_PLAN_ENTERPRISE = "ENTERPRISE"
)

var gradeMap = map[string]planLimit{}
var localPlan string

func init() {
	// =========== Termporary env ================
	localPlan = os.Getenv("PLAN_NAME")
	if localPlan == "" {
		localPlan = MACHLAKE_PLAN_TINY
	}
	//=========================================

	gradeMap[MACHLAKE_PLAN_TINY] = planLimit{
		maxQuery:         100000,
		maxNetwork:       10737418240,
		maxStorage:       10737418240,
		limitSelectValue: 1000,
		limitAppendValue: 1000,
		limitAppendTag:   1000,
		limitSelectTag:   1000,
		maxConcurrent:    5,
		defaultTagCount:  100,
		maxTagCount:      500,
	}

	gradeMap[MACHLAKE_PLAN_BASIC] = planLimit{
		maxQuery:         750000,
		maxNetwork:       10737418240,
		maxStorage:       107374182400,
		limitSelectValue: 5000,
		limitAppendValue: 5000,
		limitAppendTag:   5000,
		limitSelectTag:   5000,
		maxConcurrent:    20,
		defaultTagCount:  500,
		maxTagCount:      5000,
	}

	gradeMap[MACHLAKE_PLAN_BUSINESS] = planLimit{
		maxQuery:         4000000,
		maxNetwork:       10737418240,
		maxStorage:       1099511627776,
		limitSelectValue: 50000,
		limitAppendValue: 50000,
		limitAppendTag:   50000,
		limitSelectTag:   50000,
		maxConcurrent:    50,
		defaultTagCount:  5000,
		maxTagCount:      50000,
	}

	gradeMap[MACHLAKE_PLAN_ENTERPRISE] = planLimit{
		maxQuery:         10000000,
		maxNetwork:       10737418240,
		maxStorage:       5497558138880,
		limitSelectValue: 100000,
		limitAppendValue: 100000,
		limitAppendTag:   100000,
		limitSelectTag:   100000,
		maxConcurrent:    100,
		defaultTagCount:  50000,
		maxTagCount:      500000,
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

	// svr.log.Debugf("request param : %+v", param)

	// timezone, err := svr.makeTimezone(ctx, param.Timezone)
	// if err != nil {
	// 	rsp.Reason = err.Error()
	// 	ctx.JSON(http.StatusUnprocessableEntity, rsp)
	// 	return
	// }

	// if param.Separator == "" {
	// 	param.Separator = ","
	// }

	// currentPlan := gradeMap[localPlan]

	// Decide LimitSelTag by Plan
	// if param.TagName != "" {
	// 	param.TagList = strings.Split(param.TagName, param.Separator)
	// 	if len(param.TagList) > currentPlan.limitSelectTag {
	// 		// lakeserver conf value, loading data from  rdbms
	// 	}
	// }
	// } else {
	// 	svr.log.Info("tag name is empty")
	// 	rsp.Reason = "wrong prameter. tagname is empty"
	// 	ctx.JSON(http.StatusBadRequest, rsp)
	// 	return
	// }
	// tagname list
}

func (svr *httpd) CurrentData(ctx *gin.Context) {
	rsp := lakeRsp{Success: false, Reason: "not specified"}
	ctx.JSON(http.StatusOK, rsp)

}
func (svr *httpd) StatData(ctx *gin.Context) {
	rsp := lakeRsp{Success: false, Reason: "not specified"}
	ctx.JSON(http.StatusOK, rsp)
}
func (svr *httpd) CalcData(ctx *gin.Context) {
	rsp := lakeRsp{Success: false, Reason: "not specified"}
	ctx.JSON(http.StatusOK, rsp)
}
func (svr *httpd) PivotData(ctx *gin.Context) {
	rsp := lakeRsp{Success: false, Reason: "not specified"}
	ctx.JSON(http.StatusOK, rsp)
}

/* unused
// VerifyTimezone, convert to Machbase's Timezone variable
func (svr *httpd) makeTimezone(ctx *gin.Context, timezone string) (string, error) {
	if timezone == "" {
		svr.log.Error("use default timezone 'Etc/UTC'")
		timezone = "Etc/UTC"
	}

	matched := regexp.MustCompile(`[+-](0[0-9]|1[0-4])[0-5][0-9]$`)
	if matched.MatchString(timezone) {
		svr.log.Infof("available timezone format : %s", timezone)
		return timezone, nil
	}

	return svr.convertTimezone(ctx, timezone)
}
*/

/* unused
func (svr *httpd) convertTimezone(ctx *gin.Context, timezone string) (string, error) {
	// some time-zones can be applied convertTimezone
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
*/

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
