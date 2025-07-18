package server

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/util/ymd"
)

const (
	MACHLAKE_PLAN_TINY       = "TINY"
	MACHLAKE_PLAN_BASIC      = "BASIC"
	MACHLAKE_PLAN_BUSINESS   = "BUSINESS"
	MACHLAKE_PLAN_ENTERPRISE = "ENTERPRISE"

	HTTP_TRACKID = "cemlib/trackid"

	EDGE_SELECT_LIMIT = 10000
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
type MachbaseResult struct {
	ErrorCode    int              `json:"errorCode"`
	ErrorMessage string           `json:"errorMessage"`
	Columns      []MachbaseColumn `json:"columns"`
	Data         [][]interface{}  `json:"data"` // chqd <----> lake , data struct =  map[string]interface{}
}

type MetaResult struct {
	Data []map[string]interface{} `json:"data"`
}

type ResSet struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type MachbaseColumn struct {
	Name   string `json:"name"`
	Type   int    `json:"type"`
	Length int    `json:"length"`
}

type SelectReturn struct {
	CalcMode string           `json:"calc_mode"`
	Columns  []MachbaseColumn `json:"columns"`
	Samples  interface{}      `json:"samples"`
}

type (
	lakesvr struct {
		Info      *LakeInfo
		tagColumn string
	}
	LakeInfo struct {
		LakeId      string   `json:"lake_id"`
		Owner       string   `json:"owner"`
		Cems        string   `json:"cems"`
		DefTimezone string   `json:"timezone"`
		DefTagCnt   int64    `json:"default_tag_count"`
		MaxTagCnt   int64    `json:"max_tag_count"`
		MaxQuery    int64    `json:"max_query"`
		LimitSelTag int      `json:"limit_select_tag"`
		LimitSelVal int64    `json:"limit_select_value"`
		LimitAppTag int64    `json:"limit_append_tag"`
		LimitAppVal int64    `json:"limit_append_value"`
		Concurrent  int      `json:"max_concurrent"`
		TagExtCol   []Schema `json:"tag_schema"`   // manage_control.go -> makeLakeInfo()
		ValExtCol   []Schema `json:"value_schema"` // manage_control.go -> makeLakeInfo()
	}
	Schema struct {
		ColName   string `json:"col_name"`
		ColType   string `json:"col_type"`
		Collength int    `json:"col_length"`
	}
	SelectTagList struct {
		TagList []map[string]interface{} `json:"tag"`
	}
	ReturnData struct {
		TagName string      `json:"tag_name"`
		Data    interface{} `json:"data"`
	}
	ReturnDataPivot struct {
		Data interface{} `json:"data"`
	}
)

var lakePlanMap = map[string]planLimit{}
var localPlan string
var lakeSvr = lakesvr{}

func init() {

	lakeSvr.Info = new(LakeInfo)
	// Receive values from CreateLake, temporary test
	lakeSvr.Info.TagExtCol = append(lakeSvr.Info.TagExtCol, Schema{
		ColName:   "name",
		ColType:   "varchar",
		Collength: 80,
	})
	lakeSvr.Info.ValExtCol = append(lakeSvr.Info.ValExtCol, Schema{
		ColName:   "time",
		ColType:   "datetime",
		Collength: 0,
	})
	lakeSvr.Info.ValExtCol = append(lakeSvr.Info.ValExtCol, Schema{
		ColName:   "value",
		ColType:   "double",
		Collength: 0,
	})
	SetColumnList()

	// =========== Temporary test ================
	localPlan = os.Getenv("PLAN_NAME")
	if localPlan == "" {
		localPlan = MACHLAKE_PLAN_TINY
	}
	//=========================================

	lakePlanMap[MACHLAKE_PLAN_TINY] = planLimit{
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

	lakePlanMap[MACHLAKE_PLAN_BASIC] = planLimit{
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

	lakePlanMap[MACHLAKE_PLAN_BUSINESS] = planLimit{
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

	lakePlanMap[MACHLAKE_PLAN_ENTERPRISE] = planLimit{
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
func SetColumnList() {
	cols := []string{}
	for _, row := range lakeSvr.Info.TagExtCol {
		cols = append(cols, `"`+row.ColName+`"`)
	}

	if len(cols) > 0 {
		lakeSvr.tagColumn = strings.Join(cols, ", ")
	}
}

func (svr *httpd) handleLakeGetTagList(ctx *gin.Context) {
	trackId := ctx.GetString(HTTP_TRACKID)
	svr.log.Trace(trackId, "start handleLakeGetTagList()")

	rsp := ResSet{Status: "fail"}
	hint := ""

	name := ctx.Query("name")
	svr.log.Debug(trackId, "param(name) : ", name)
	if name != "" {
		hint = " WHERE NAME LIKE '%" + name + "%'"
	}
	hint += " order by _ID"
	hint += " limit "

	offset := ctx.Query("offset")
	if offset != "" {
		svr.log.Debug(trackId, "param(offset) : ", offset)
		hint = hint + offset + ","
	}

	currentPlan := lakePlanMap[localPlan]

	limit := ctx.Query("limit")
	svr.log.Debug(trackId, "param(limit) : ", limit)
	if limit != "" && limit != "0" {
		check := svr.checkSelectTagLimit(ctx, limit, currentPlan.limitSelectTag)
		if check != "" {
			ctx.JSON(http.StatusPreconditionFailed, rsp)
			return
		}
		hint += limit
	} else {
		defaultLimit := currentPlan.limitSelectValue
		hint += fmt.Sprintf("%d", defaultLimit)
	}

	sqlText := fmt.Sprintf("SELECT %s FROM _TAG_META%s", lakeSvr.tagColumn, hint)
	svr.log.Debug(trackId, "query : ", sqlText)

	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		rsp.Message = err.Error()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	defer conn.Close()

	data, err := svr.selectTagMetaList(ctx, conn, sqlText)
	if err != nil {
		rsp.Message = err.Error()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	rsp.Status = "success"
	rsp.Message = "get tag meta list success"
	rsp.Data = data

	svr.log.Trace(trackId, "list tag meta success")

	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) selectTagMetaList(ctx *gin.Context, conn api.Conn, sqlText string) (*SelectTagList, error) {
	result := &SelectTagList{}
	metaList, err := svr.getMetaData(ctx, conn, sqlText)
	if err != nil {
		return result, err
	}

	result.TagList = metaList.Data
	return result, err
}

func (svr *httpd) checkSelectTagLimit(ctx *gin.Context, limitStr string, limitSelectTag int) string {
	trackId := ctx.Value(HTTP_TRACKID)

	result := ""
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		svr.log.Info(trackId, "ParseInt error : ", err.Error())
		result = "limit param is not number"
		return result
	}

	if limit > limitSelectTag {
		svr.log.Infof("%s limit over. (parameter:%d, Available:%d)", trackId, limit, limitSelectTag)
		result = fmt.Sprintf("limit over. (parameter:%d, Available:%d)", limit, limitSelectTag)
		svr.log.Error(result)
	}

	return result
}

func (svr *httpd) handleLakeGetValues(ctx *gin.Context) {
	trackId := ctx.GetString(HTTP_TRACKID)
	svr.log.Trace(trackId, "start handleLakeGetValues()")

	// Previous lake server used cli to use db
	dataType := ctx.Param("type")

	// Receiving form-data, query-string
	switch dataType {
	case "raw":
		svr.GetRawData(ctx)
	case "calculated":
		svr.GetCalculateData(ctx)
	case "group":
		svr.GetGroupData(ctx)
	case "last":
		svr.GetLastData(ctx)
	case "current":
		svr.GetCurrentData(ctx)
	case "pivoted":
		svr.GetPivotData(ctx)
	case "stat":
		svr.GetStatData(ctx)
	default:
		svr.log.Info(trackId, "not available type : ", dataType)
		rsp := lakeRsp{Success: false, Reason: "This type is not available"}
		ctx.JSON(http.StatusBadRequest, rsp)
	}
}

func (svr *httpd) GetRawData(ctx *gin.Context) {
	trackId := ctx.GetString(HTTP_TRACKID)
	svr.log.Trace(trackId, "start RawData()")

	rsp := ResSet{Status: "fail", Message: "not specified"}

	param := SelectRaw{}
	err := ctx.ShouldBind(&param)
	if err != nil {
		rsp.Message = err.Error()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	svr.log.Infof("param: %+v", param)

	switch param.ReturnType {
	case "":
		param.ReturnType = "0"
	case "0", "1":
		svr.log.Trace(trackId, "return type ok")
	default:
		svr.log.Info(trackId, "return form range over")
		rsp.Data = map[string]interface{}{
			"title": "Wrong Parameter. (value_return_form) : must be 0,1",
		}
		ctx.JSON(http.StatusPreconditionFailed, rsp)
		return
	}

	if param.Separator == "" {
		param.Separator = ","
	}

	currentPlan := lakePlanMap[localPlan]

	// It may be necessary to know the plan to know the LimitSelTag value
	if param.TagName != "" {
		param.TagList = strings.Split(param.TagName, param.Separator)
		if len(param.TagList) > currentPlan.limitSelectTag { // lakeserver conf 값,   mysql 에서 데이터 로드 필요
			rsp.Message = fmt.Sprintf("tag count over. (parameter:%d, Available:%d)", len(param.TagList), currentPlan.limitSelectTag)
			svr.log.Info(rsp.Message)
			ctx.JSON(http.StatusUnprocessableEntity, rsp)
			return
		}
	} else {
		svr.log.Info("Tag name is empty")
		rsp.Message = "Wrong Parameter. (tagname) : must be a least 1"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	// date format
	if param.DateFormat == "" {
		param.DateFormat = "YYYY-MM-DD HH24:MI:SS"
	}

	// start time
	param.StartType, err = svr.checkTimeFormat(ctx, param.StartTime, false)
	if err != nil {
		rsp.Message = "Wrong Parameter. (startTime)"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	//end time
	param.EndType, err = svr.checkTimeFormat(ctx, param.EndTime, false)
	if err != nil {
		rsp.Message = "Wrong Parameter. (endTime)"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	err = svr.checkTimePeriod(ctx, param.StartTime, param.StartType, param.EndTime, param.EndType)
	if err != nil {
		rsp.Message = err.Error()
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	// get column list
	if param.Columns != "" {
		param.ColumnList = strings.Split(param.Columns, param.Separator)
	} else {
		param.ColumnList = append(param.ColumnList, "VALUE")
	}

	// get alias list
	if param.Alias != "" {
		param.AliasList = strings.Split(param.Alias, param.Separator)
		//length check
		if len(param.ColumnList) != len(param.AliasList) {
			svr.log.Infof("The number of 'columns' and 'aliases' is different (column=%d, alias=%d)", len(param.ColumnList), len(param.AliasList))
			rsp.Message = "The number of 'columns' and 'aliases' is different"
			ctx.JSON(http.StatusUnprocessableEntity, rsp)
			return
		}
	}

	param.TableName = strings.ToUpper(param.TableName)

	// limit count
	switch param.TableName {
	case "TAG":
		if param.Limit != "" {
			if check := svr.checkSelectValueLimit(ctx, param.Limit, currentPlan.limitSelectValue); check != "" {
				rsp.Message = check
				ctx.JSON(http.StatusUnprocessableEntity, rsp)
				return
			}
		} else { // generally limit is received as a param?
			param.Limit = fmt.Sprintf("%d", currentPlan.limitSelectValue)
		}
	case "TAGDATA":
		if param.Limit == "" {
			param.Limit = fmt.Sprintf("%d", EDGE_SELECT_LIMIT) //default 5000, 10000
		}
	}

	// get direction type
	switch param.TableName {
	case "TAG":
		if param.Direction != "" {
			if param.Direction != "0" && param.Direction != "1" {
				svr.log.Info("direction range over")
				rsp.Message = "Wrong Parameter. (direction) : must be 0, 1"
				ctx.JSON(http.StatusUnprocessableEntity, rsp)
				return
			}
		} else {
			// TODO: remove this after solving nfx #128
			param.Direction = "0"
		}
	case "TAGDATA":
		param.Direction = "0"
	}

	sqlText := "SELECT "
	sqlText += makeScanHint(param.Direction, "TAG")                           // SELECT /*+ SCAN_BACKWARD(TAG) */
	sqlText += "NAME, "                                                       // SELECT /*+ SCAN_BACKWARD(TAG) */ NAME,
	sqlText += makeTimeColumn("TIME", param.DateFormat, "TIME")               // SELECT /*+ SCAN_BACKWARD(TAG) */ NAME, TO_TIMESTAMP(TIME) AS TIME
	sqlText += makeValueColumn(param.ColumnList, param.AliasList) + " "       // SELECT /*+ SCAN_BACKWARD(TAG) */ NAME, TO_TIMESTAMP(TIME) AS TIME , "value" AS "value"
	sqlText += "FROM " + param.TableName + " "                                // SELECT /*+ SCAN_BACKWARD(TAG) */ NAME, TO_TIMESTAMP(TIME) AS TIME , "value" AS "value" FROM TAG
	sqlText += "WHERE " + makeInCondition("NAME", param.TagList, false, true) // SELECT /*+ SCAN_BACKWARD(TAG) */ NAME, TO_TIMESTAMP(TIME) AS TIME , "value" AS "value" FROM TAG WHERE NAME IN(val, val, val)
	if param.StartType == "date" {
		sqlText += makeBetweenCondition("TIME", makeToDate(param.StartTime), makeToDate(param.EndTime), true) + " "
	} else {
		sqlText += makeBetweenCondition("TIME", svr.makeFromTimestamp(ctx, param.StartTime), svr.makeFromTimestamp(ctx, param.EndTime), true) + " "
	}
	sqlText += makeAndCondition(param.AndCondition, param.Separator, true)
	sqlText += makeLimit(param.Offset, param.Limit)

	svr.log.Debug(trackId, "query : ", sqlText)

	// scale의 수만큼 소수점 자릿수를 보여줌
	// 기존 Lake getDataCli() 에서는 scale 을 설정하는 함수가 존재
	// Neo는 scale 설정이 없으므로 데이터를 scan 후에 scale만큼 소수점을 잘라주고 리턴
	// dbData, err := svr.getData(sqlText, param.Scale)
	// if err != nil {
	// 	svr.log.Info(trackId, "get Data error : ", err.Error())
	// 	rsp.Message = err.Error()
	// 	ctx.JSON(http.StatusBadRequest, rsp)
	// 	return
	// }

	// data := MakeReturnFormat(dbData, "raw", param.ReturnType, "tag", param.TagList)

	// rsp.Status = "success"
	// rsp.Data = data

	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		rsp.Message = err.Error()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	defer conn.Close()

	data, err := svr.selectData(ctx, conn, sqlText, param.TagList)
	if err != nil {
		svr.log.Info(trackId, "select data error : ", err.Error())
		rsp.Message = err.Error()
		ctx.JSON(http.StatusFailedDependency, rsp)
		return
	}

	rsp.Status = "success"
	rsp.Data = data

	svr.log.Trace(trackId, "select raw data success")

	ctx.JSON(http.StatusOK, rsp)
}

/*
[Calculate - TAGDATA]

SELECT NAME, TO_TIMESTAMP(DATE_TRUNC('SEC', TIME, 38)/1000000) AS TIME, AVG(VALUE) AS VALUE
FROM (

	SELECT NAME, TIME, DECODE(type, 'float64', value, ivalue) as VALUE FROM TAGDATA WHERE NAME IN('tag1')  AND TIME BETWEEN
	FROM_TIMESTAMP(1690864685000000000) AND FROM_TIMESTAMP(1690875485000000000)
	)

GROUP BY NAME, TIME
ORDER BY TIME
LIMIT 1000
*/
func (svr *httpd) GetCalculateData(ctx *gin.Context) {
	trackId := ctx.GetString(HTTP_TRACKID)
	svr.log.Trace(trackId, "start GetCalculateData()")

	rsp := ResSet{Status: "fail"}
	param := SelectCalc{}

	err := ctx.ShouldBind(&param)
	if err != nil {
		svr.log.Info(trackId, "bind error : ", err.Error())
		rsp.Message = "get parameter failed"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}
	svr.log.Debugf("%s request data %+v", trackId, param)

	switch param.ReturnType {
	case "":
		param.ReturnType = "0"
	case "0", "1":
		svr.log.Trace(trackId, "return type ok")
	default:
		svr.log.Info(trackId, "return form range over")
		rsp.Message = "Wrong Parameter. (value_return_form) : must be 0,1"
		ctx.JSON(http.StatusPreconditionFailed, rsp)
		return
	}

	if param.Separator == "" {
		param.Separator = ","
	}

	currentPlan := lakePlanMap[localPlan]

	if param.TagName != "" {
		param.TagList = strings.Split(param.TagName, param.Separator)
		if len(param.TagList) > currentPlan.limitSelectTag {
			rsp.Message = fmt.Sprintf("tag count over. (parameter:%d, Available:%d)", len(param.TagList), currentPlan.limitSelectTag)
			svr.log.Info(trackId, rsp.Message)
			ctx.JSON(http.StatusUnprocessableEntity, rsp)
			return
		}
	} else {
		svr.log.Info(trackId, "Tag name is nil")
		rsp.Message = "Wrong Parameter. (tag_name) : must be at least 1"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	if param.DateFormat == "" {
		param.DateFormat = "YYYY-MM-DD HH24:MI:SS"
	}

	/* calc mode */
	if param.CalcMode != "" {
		if param.CalcMode, err = svr.checkCalcUnit(ctx, param.CalcMode); err != nil {
			rsp.Message = "Wrong Parameter. (calc_mode) : form must be min,max,cnt,avg,sum,sumsq"
			ctx.JSON(http.StatusUnprocessableEntity, rsp)
			return
		}
	} else {
		param.CalcMode = "AVG"
	}

	param.StartType, err = svr.checkTimeFormat(ctx, param.StartTime, false)
	if err != nil {
		rsp.Message = "Wrong Parameter. (start_time)"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	/* end time */
	param.EndType, err = svr.checkTimeFormat(ctx, param.EndTime, false)
	if err != nil {
		rsp.Message = "Wrong Parameter. (end_time)"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	err = svr.checkTimePeriod(ctx, param.StartTime, param.StartType, param.EndTime, param.EndType)
	if err != nil {
		rsp.Message = err.Error()
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	if param.IntervalType != "" {
		if param.IntervalType, err = svr.checkTimeUnit(ctx, param.IntervalType); err != nil {
			rsp.Message = "Wrong Parameter. (interval_type) : form must be sec,min,hour,day"
			ctx.JSON(http.StatusUnprocessableEntity, rsp)
			return
		}
	} else {
		param.IntervalType = "SEC"
	}

	/* interval value */
	if param.IntervalValue == "" {
		param.IntervalValue = "1"
	}

	/* limit count */
	if param.Limit != "" {
		if check := svr.checkSelectValueLimit(ctx, param.Limit, currentPlan.limitSelectValue); check != "" {
			rsp.Message = check
			ctx.JSON(http.StatusUnprocessableEntity, rsp)
			return
		}
	} else {
		param.Limit = fmt.Sprintf("%d", currentPlan.limitSelectValue)
	}

	/* direction type */
	if param.Direction != "" {
		if param.Direction != "0" && param.Direction != "1" {
			svr.log.Info("direction range over")
			rsp.Message = "Wrong Parameter. (direction) : must be 0, 1"
			ctx.JSON(http.StatusUnprocessableEntity, rsp)
			return
		}
	} else {
		// TODO: remove this after solving nfx #128
		param.Direction = "0"
	}

	/* get Interpolation type (reserved) */
	if param.Interpolation > 3 || param.Interpolation < 0 {
		svr.log.Info("%s interpolation range over : %d", trackId, param.Interpolation)
		rsp.Message = "Wrong Parameter. (interpolation) : form must be 0,1,2,3"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	columnList := []string{"TIME", "NAME"}
	var sqlText string
	switch param.TableName {
	case "TAG":
		sqlText += "SELECT NAME, "
		sqlText += makeTimeColumn(makeDateTrunc(param.IntervalType, "TIME", param.IntervalValue), param.DateFormat, "TIME") + ", "
		sqlText += makeCalculator("VALUE", param.CalcMode) + " AS VALUE "
		sqlText += "FROM "

		// sub
		sqlText += "(SELECT NAME, "
		sqlText += makeRollupHint("TIME", param.IntervalType, param.CalcMode, "VALUE") + " "
		sqlText += "FROM " + "TAG" + " "
		sqlText += "WHERE " + makeInCondition("NAME", param.TagList, false, true)

		if param.StartType == "date" {
			sqlText += makeBetweenCondition("TIME", makeToDate(param.StartTime), makeToDate(param.EndTime), true) + " "
		} else {
			sqlText += makeBetweenCondition("TIME", svr.makeFromTimestamp(ctx, param.StartTime), svr.makeFromTimestamp(ctx, param.EndTime), true) + " "
		}
		sqlText += makeGroupBy(columnList) + ") "

		// sub(end)
		sqlText += makeGroupBy(columnList) + " "

		sortList := make([]string, 0)
		if param.Direction != "" {
			columnList = []string{"TIME"}
			sortList = append(sortList, param.Direction)
			sqlText += makeOrderBy(columnList, sortList) + " "
		}
		sqlText += makeLimit(param.Offset, param.Limit)

	case "TAGDATA": // Use TAGDATA table, so there is no need to use ROLLUP
		sqlText = fmt.Sprintf(SqlTidy(`
		SELECT NAME, %s, %s(VALUE) AS VALUE
		FROM TAGDATA
		WHERE %s %s
		GROUP BY NAME, TIME
		ORDER BY TIME
		`),
			makeTimeColumn(makeDateTrunc(param.IntervalType, "TIME", param.IntervalValue), param.DateFormat, "TIME"), param.CalcMode,
			makeInCondition("NAME", param.TagList, false, true),
			makeBetweenCondition("TIME", svr.makeFromTimestamp(ctx, param.StartTime), svr.makeFromTimestamp(ctx, param.EndTime), true)+" ")
		sqlText += " " + makeLimit(param.Offset, param.Limit) // add space
	}
	svr.log.Debug(trackId, "query : ", sqlText)

	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		rsp.Message = err.Error()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	defer conn.Close()

	data, err := svr.selectData(ctx, conn, sqlText, param.TagList)
	if err != nil {
		svr.log.Info(trackId, "select data error : ", err.Error())
		rsp.Message = err.Error()
		ctx.JSON(http.StatusFailedDependency, rsp)
		return
	}
	data.CalcMode = param.CalcMode

	rsp.Status = "success"
	rsp.Data = data

	svr.log.Trace(trackId, "select calculate data success")

	ctx.JSON(http.StatusOK, rsp)
}

type SelectGroup struct {
	TagName       string `form:"tag_name" json:"tag_name"`
	StartTime     string `form:"start_time" json:"start_time"`
	EndTime       string `form:"end_time" json:"end_time"`
	CalculateMode string `form:"calc_mode" json:"calc_mode"`
	IntervalType  string `form:"interval_type" json:"interval_type"`
	IntervalValue string `form:"interval_value" json:"interval_value"`
}

func (svr *httpd) GetGroupData(ctx *gin.Context) {
	trackId := ctx.GetString(HTTP_TRACKID)
	svr.log.Trace(trackId, "start GetGroupData()")

	rsp := ResSet{Status: "fall"}
	param := SelectGroup{}

	err := ctx.ShouldBind(&param)
	if err != nil {
		svr.log.Info(trackId, "bind error: ", err.Error())
		rsp.Message = "get parameter failed"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	svr.log.Debugf("%s param: %+v", trackId, param)

	var tagList []string
	if param.TagName != "" {
		tagList = strings.Split(param.TagName, ",")
	} else {
		svr.log.Info(trackId, "tag name is empty")
		rsp.Message = "tag name is empty"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	calcMode := strings.ToUpper(param.CalculateMode)
	switch calcMode {
	case "MIN", "MAX", "AVG", "SUM", "COUNT", "SUMSQ":
	default:
		svr.log.Infof(trackId, "invalid calculate mode: %q", calcMode)
		rsp.Message = fmt.Sprintf("invalid calculate mode: %q", calcMode)
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	sqlText := fmt.Sprintf(SqlTidy(`
		SELECT TO_CHAR(MTIME, 'YYYY-MM-DD HH:MI:SS') AS TIME, %s(MVALUE) AS VALUE 
		FROM (
			SELECT %s AS MTIME, DECODE(type, 'float64', value, ivalue) AS MVALUE
			FROM TAG
			WHERE %s %s
			) 
		GROUP BY TIME 
		ORDER BY TIME
	`),
		calcMode,
		makeDateTrunc(param.IntervalType, "TIME", param.IntervalValue), param.TagName,
		makeInCondition("NAME", tagList, false, true),
		makeBetweenCondition("TIME", svr.makeFromTimestamp(ctx, param.StartTime), svr.makeFromTimestamp(ctx, param.EndTime), true),
	)

	svr.log.Debug(trackId, "query : ", sqlText)

	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		rsp.Message = err.Error()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	defer conn.Close()

	dbData, err := svr.getData(ctx, conn, sqlText, 0)
	if err != nil {
		svr.log.Info(trackId, "get data error : ", err.Error())
		rsp.Message = err.Error()
		ctx.JSON(http.StatusFailedDependency, rsp)
		return
	}

	data := MakeReturnFormat(dbData, param.CalculateMode, "0", "tag", tagList)

	rsp.Status = "success"
	rsp.Data = data

	svr.log.Trace(trackId, "select group data success")

	ctx.JSON(http.StatusOK, rsp)
}

type SelectLast struct {
	TagName       string `form:"tag_name" json:"tag_name"`
	StartTime     string `form:"start_time" json:"start_time"`
	EndTime       string `form:"end_time" json:"end_time"`
	CalculateMode string `form:"calc_mode" json:"calc_mode"`
}

func (svr *httpd) GetLastData(ctx *gin.Context) {
	trackId := ctx.GetString(HTTP_TRACKID)
	svr.log.Trace(trackId, "start GetLastData()")

	rsp := ResSet{Status: "fall"}
	param := SelectLast{}

	err := ctx.ShouldBind(&param)
	if err != nil {
		svr.log.Info(trackId, "bind error: ", err.Error())
		rsp.Message = "get parameter failed"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	var tagList []string
	if param.TagName != "" {
		tagList = strings.Split(param.TagName, ",")
	} else {
		svr.log.Info("tag name is empty")
		rsp.Message = "tag name is empty"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	selectText := ""
	calcMode := strings.ToUpper(param.CalculateMode)
	switch calcMode {
	case "SUM", "MIN", "MAX", "AVG", "SUMSQ", "STDDEV", "STDDEV_POP", "VARIANCE", "VAR_POP":
		selectText = fmt.Sprintf("TO_CHAR(LAST(TIME, TIME), 'YYYY-MM-DD HH:MI:SS') AS TIME, %s(VALUE) AS VALUE", calcMode)
	case "COUNT", "CNT":
		selectText = "TO_CHAR(LAST(TIME, TIME), 'YYYY-MM-DD HH:MI:SS') AS TIME, COUNT(*) AS VALUE"
	case "FIRST":
		selectText = "TO_CHAR(FIRST(TIME, TIME), 'YYYY-MM-DD HH:MI:SS') AS TIME, FIRST(TIME, VALUE) AS VALUE"
	case "LAST":
		selectText = "TO_CHAR(LAST(TIME, TIME), 'YYYY-MM-DD HH:MI:SS') AS TIME, LAST(TIME, VALUE) AS VALUE"
	default:
		svr.log.Infof("invalid calculate mode : %q", calcMode)
		rsp.Message = fmt.Sprintf("invalid calculate mode : %q", calcMode)
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	sqlText := fmt.Sprintf(SqlTidy(`
		SELECT %s 
		FROM TAG
		WHERE %s AND %s
	`), selectText,
		makeInCondition("NAME", tagList, false, true),
		makeBetweenCondition("TIME", svr.makeFromTimestamp(ctx, param.StartTime), svr.makeFromTimestamp(ctx, param.EndTime), false))

	svr.log.Debug(trackId, "query : ", sqlText)

	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		rsp.Message = err.Error()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	defer conn.Close()

	data, err := svr.selectData(ctx, conn, sqlText, tagList)
	if err != nil {
		svr.log.Info(trackId, "select data error : ", err.Error())
		rsp.Message = err.Error()
		ctx.JSON(http.StatusFailedDependency, rsp)
		return
	}
	data.CalcMode = calcMode

	rsp.Status = "success"
	rsp.Data = data

	svr.log.Trace(trackId, "select last data success")

	ctx.JSON(http.StatusOK, rsp)
}

// struct 를 이용한 데이터 receive or map[string]interface => 모든 api에서 name,time,value일 경우
// tagList []string 으로 매개변수 변경 후, split 된 길이를 체크 한 후에 2개 이상일 시 if문 추가
func (svr *httpd) selectData(ctx context.Context, conn api.Conn, sqlText string, tagList []string) (*SelectReturn, error) {
	t := time.Now()
	result := &SelectReturn{}

	rows, err := conn.Query(ctx, sqlText)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	columnsLen := len(columns.Names())
	columnsList := make([]MachbaseColumn, columnsLen)

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()

		for i, col := range columns {
			columnsList[i].Name = col.Name
			columnsList[i].Type = int(col.DataType.ColumnType())
		}
	}()

	dataList := []map[string]interface{}{}
	for rows.Next() {
		data := map[string]interface{}{}
		buffer, err := columns.MakeBuffer()
		if err != nil {
			svr.log.Error("make buffer error: ", err.Error())
			return nil, err
		}
		err = rows.Scan(buffer...)
		if err != nil {
			svr.log.Error("scan error: ", err.Error())
			return nil, err
		}
		for i, col := range columns {
			data[col.Name] = buffer[i]
		}
		dataList = append(dataList, data)
	}

	wg.Wait()

	tagName := strings.Join(tagList, ",")

	result.Columns = columnsList
	result.Samples = []map[string]interface{}{
		{
			"tag_name": tagName,
			"data":     dataList,
		},
	}

	svr.log.Info("Elapse : ", time.Since(t).String())
	return result, nil
}

func SqlTidy(sqlText string) string {
	lines := strings.Split(sqlText, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimSpace(ln)
	}
	return strings.TrimSpace(strings.Join(lines, " "))
}

func (svr *httpd) GetCurrentData(ctx *gin.Context) {
	trackId := ctx.GetString(HTTP_TRACKID)
	svr.log.Trace(trackId, "start GetCurrentData()")
	rsp := ResSet{Status: "fall"}

	param := SelectRaw{}

	err := ctx.ShouldBind(&param)
	if err != nil {
		svr.log.Info(trackId, "bind error : ", err.Error())
		rsp.Message = "get parameter failed"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	svr.log.Debugf("%s request data %+v", trackId, param)

	switch param.ReturnType {
	case "":
		param.ReturnType = "0"
	case "0", "1":
		svr.log.Trace(trackId, "return type ok")
	default:
		svr.log.Info(trackId, "return form range over")
		rsp.Data = map[string]interface{}{
			"title": "Wrong Parameter. (value_return_form) : must be 0,1",
		}
		ctx.JSON(http.StatusPreconditionFailed, rsp)
		return
	}

	if param.Separator == "" {
		param.Separator = ","
	}

	currentPlan := lakePlanMap[localPlan]

	if param.TagName != "" {
		param.TagList = strings.Split(param.TagName, param.Separator)

		if len(param.TagList) > currentPlan.limitSelectTag {
			rsp.Message = fmt.Sprintf("tag count over. (parameter:%d, Available:%d)", len(param.TagList), currentPlan.limitSelectTag)
			svr.log.Info(trackId, rsp.Message)
			ctx.JSON(http.StatusUnprocessableEntity, rsp)
			return
		}
	} else {
		svr.log.Info(trackId, "Tag name is nil")
		rsp.Message = "Wrong Parameter. (tag_name) : must be at least 1"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	if param.DateFormat == "" {
		param.DateFormat = "YYYY-MM-DD HH24:MI:SS"
	}

	if param.Columns != "" {
		param.ColumnList = strings.Split(param.Columns, param.Separator)
	} else {
		param.ColumnList = append(param.ColumnList, "VALUE")
	}

	if param.Alias != "" {
		param.AliasList = strings.Split(param.Alias, param.Separator)

		if len(param.ColumnList) != len(param.AliasList) {
			svr.log.Infof("%s The number of 'columns' and 'aliases' is different (column=%d, alias=%d)", trackId, len(param.ColumnList), len(param.AliasList))
			rsp.Message = "The number of 'columns' and 'aliases' is different"
			ctx.JSON(http.StatusUnprocessableEntity, rsp)
			return
		}
	}

	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		rsp.Message = err.Error()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	defer conn.Close()

	//  SELECT /*+ SCAN_BACKWARD(TAG) */ NAME, TO_TIMESTAMP(TIME) AS TIME, VALUE FROM TAG
	// sqlText := "SELECT " + makeScanHint("1", "TAG") //
	// sqlText += "NAME, "
	// sqlText += makeTimeColumn("TIME", param.DateFormat, "TIME")
	// sqlText += makeValueColumn(param.ColumnList, param.AliasList) + " "
	// sqlText += "FROM " + "TAG"

	//
	sqlText := "SELECT name, " + makeTimeColumn("last(time, time)", param.DateFormat, "TIME") + ", last(time, value) AS value "
	sqlText += "FROM TAG "
	sqlText += "WHERE name IN ("
	sqlText += "SELECT name FROM _TAG_META WHERE name LIKE " + makeLikeTag(param.TagList[0])
	sqlText += ") AND time >= (SELECT min(RECENT_ROW_TIME) FROM V$TAG_STAT WHERE name IN ("
	sqlText += "SELECT name FROM _TAG_META WHERE name LIKE " + makeLikeTag(param.TagList[0]) + "))"
	sqlText += "GROUP BY name"

	// dataChannel := make(chan []interface{}, len(param.TagList))
	result := MachbaseResult{}

	// wg := sync.WaitGroup{}
	// for idx, tagName := range param.TagList {
	// 	wg.Add(1)

	// 	go func(svr *httpd, where string, idx int) {
	// 		defer wg.Done()

	// 		sqlQuery := fmt.Sprintf("%s %s", sqlText, where)
	// 		svr.log.Debugf("%s [%d] query : %s", trackId, idx, sqlQuery)

	// 		dbData, err := svr.getData(ctx, conn, sqlQuery, param.Scale)
	// 		if err != nil {
	// 			svr.log.Infof("%s [%d] get data error : %s", trackId, idx, err.Error())
	// 			return
	// 		}

	// 		result.Columns = dbData.Columns //  columns 는 slice인데 append가 아닌 대입만 하는 이유는? 어차피 컬럼이 똑같아서 첫번째만 대입?

	// 		// add success select data
	// 		if len(dbData.Data) > 0 {
	// 			if len(dbData.Data[0]) > 0 {
	// 				dataChannel <- dbData.Data[0] // 첫번째 인덱스만 가져가는 이유 : WHERE절 Limit 1 , scan_backward로 가장 최근 데이터
	// 			}
	// 		}
	// 	}(svr, fmt.Sprintf("WHERE NAME='%s' LIMIT 1", tagName), idx)
	// }

	svr.log.Debugf("[current] sqlText : %s", sqlText)

	dbData, err := svr.getData(ctx, conn, sqlText, param.Scale)
	if err != nil {
		svr.log.Infof("%s get data error : %s", trackId, err.Error())
		return
	}

	result.Columns = dbData.Columns
	result.Data = dbData.Data

	// wg.Wait()
	// close(dataChannel)

	// for row := range dataChannel {
	// 	result.Data = append(result.Data, row)
	// }

	data := MakeReturnFormat(&result, "raw", param.ReturnType, "tag", param.TagList)

	rsp.Status = "success"
	rsp.Data = data

	svr.log.Trace(trackId, "select current data success")

	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) GetStatData(ctx *gin.Context) {
	trackId := ctx.GetString(HTTP_TRACKID)
	svr.log.Trace(trackId, "start GetStatDataV1()")

	rsp := ResSet{Status: "fail"}
	param := SelectRaw{}

	err := ctx.ShouldBind(&param)
	if err != nil {
		svr.log.Info(trackId, "bind error : ", err.Error())
		rsp.Message = "get parameter failed"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	svr.log.Debugf("%s request data %+v", trackId, param)

	switch param.ReturnType {
	case "":
		param.ReturnType = "0"
	case "0", "1":
		svr.log.Trace(trackId, "return type ok")
	default:
		svr.log.Info(trackId, "return form range over")
		rsp.Data = map[string]interface{}{
			"title": "Wrong Parameter. (value_return_form) : must be 0,1",
		}
		ctx.JSON(http.StatusPreconditionFailed, rsp)
		return
	}

	if param.Separator == "" {
		param.Separator = ","
	}

	currentPlan := lakePlanMap[localPlan]

	if param.TagName != "" {
		param.TagList = strings.Split(param.TagName, param.Separator)

		if len(param.TagList) > currentPlan.limitSelectTag {
			rsp.Message = fmt.Sprintf("tag count over. (parameter:%d, Available:%d)", len(param.TagList), currentPlan.limitSelectTag)
			svr.log.Info(trackId, rsp.Message)
			ctx.JSON(http.StatusUnprocessableEntity, rsp)
			return
		}
	} else {
		svr.log.Info(trackId, "Tag name is nil")
		rsp.Message = "Wrong Parameter. (tag_name) : must be at least 1"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	if param.DateFormat == "" {
		param.DateFormat = "YYYY-MM-DD HH24:MI:SS"
	}

	if param.Limit != "" {
		check := svr.checkSelectValueLimit(ctx, param.Limit, currentPlan.limitSelectValue)
		if check != "" {
			rsp.Message = check
			ctx.JSON(http.StatusUnprocessableEntity, rsp)
			return
		}
	} else {
		param.Limit = fmt.Sprintf("%d", currentPlan.limitSelectValue)
	}

	// SELECT NAME, ROW_COUNT, MIN_VALUE, MAX_VALUE TO_TIMESTAMP(MIN_TIME) AS MIN_TIME, TO_TIMESTAMP(MAX_TIME) AS MAX_TIME
	// 		  TO_TIMESTAMP(MIN_VALUE_TIME) AS MIN_VALUE_TIME, TO_TIMESTAMP(MAX_VALUE_TIME) AS MAX_VALUE_TIME, TO_TIMESTAMP(RECENT_ROW_TIME) AS RECENT_ROW_TIME
	// FROM V$TAG_STAT
	// WHERE NAME IN (tagvalue) LIMIT

	sqlText := "SELECT "
	sqlText += "NAME, ROW_COUNT, MIN_VALUE, MAX_VALUE, "
	sqlText += makeTimeColumn("MIN_TIME", param.DateFormat, "MIN_TIME") + ", "
	sqlText += makeTimeColumn("MAX_TIME", param.DateFormat, "MAX_TIME") + ", "
	sqlText += makeTimeColumn("MIN_VALUE_TIME", param.DateFormat, "MIN_VALUE_TIME") + ", "
	sqlText += makeTimeColumn("MAX_VALUE_TIME", param.DateFormat, "MAX_VALUE_TIME") + ", "
	sqlText += makeTimeColumn("RECENT_ROW_TIME", param.DateFormat, "RECENT_ROW_TIME") + " "
	sqlText += "FROM " + "V$TAG_STAT" + " "
	sqlText += "WHERE " + makeInCondition("NAME", param.TagList, false, true) + " "
	sqlText += makeLimit(param.Offset, param.Limit)

	svr.log.Debug(trackId, "query : ", sqlText)

	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		rsp.Message = err.Error()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	defer conn.Close()

	dbData, err := svr.getData(ctx, conn, sqlText, param.Scale)
	if err != nil {
		svr.log.Info(trackId, "get data error : ", err.Error())
		rsp.Message = err.Error()
		ctx.JSON(http.StatusFailedDependency, rsp)
		return
	}

	data := MakeReturnFormat(dbData, "raw", param.ReturnType, "tag", param.TagList)

	rsp.Status = "success"
	rsp.Data = data

	svr.log.Trace(trackId, "select stat data success")

	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) GetPivotData(ctx *gin.Context) {
	trackId := ctx.GetString(HTTP_TRACKID)
	svr.log.Trace(trackId, "start GetPivotData()")

	rsp := ResSet{Status: "fail"}
	param := SelectCalc{}

	err := ctx.ShouldBind(&param)
	if err != nil {
		svr.log.Info(trackId, "bind error : ", err.Error())
		rsp.Message = "get parameter failed"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	svr.log.Debugf("%s request data %+v", trackId, param)

	switch param.ReturnType {
	case "":
		param.ReturnType = "0"
	case "0", "1":
		svr.log.Trace(trackId, "return type ok")
	default:
		svr.log.Info(trackId, "return form range over")
		rsp.Data = map[string]interface{}{
			"title": "Wrong Parameter. (value_return_form) : must be 0,1",
		}
		ctx.JSON(http.StatusPreconditionFailed, rsp)
		return
	}
	if param.Separator == "" {
		param.Separator = ","
	}

	currentPlan := lakePlanMap[localPlan]

	if param.TagName != "" {
		param.TagList = strings.Split(param.TagName, param.Separator)

		if len(param.TagList) > currentPlan.limitSelectTag {
			rsp.Message = fmt.Sprintf("tag count over. (parameter:%d, Available:%d)", len(param.TagList), currentPlan.limitSelectTag)
			svr.log.Info(trackId, rsp.Message)
			ctx.JSON(http.StatusUnprocessableEntity, rsp)
			return
		}
	} else {
		svr.log.Info(trackId, "Tag name is nil")
		rsp.Message = "Wrong Parameter. (tag_name) : must be at least 1"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	if param.DateFormat == "" {
		param.DateFormat = "YYYY-MM-DD HH24:MI:SS"
	}

	/* calc mode */
	if param.CalcMode != "" {
		if param.CalcMode, err = svr.checkCalcUnit(ctx, param.CalcMode); err != nil {
			rsp.Message = "Wrong Parameter. (calc_mode) : form must be min,max,cnt,avg,sum,sumsq"
			ctx.JSON(http.StatusUnprocessableEntity, rsp)
			return
		}
	} else {
		param.CalcMode = "AVG"
	}

	param.StartType, err = svr.checkTimeFormat(ctx, param.StartTime, false)
	if err != nil {
		rsp.Message = "Wrong Parameter. (start_time)"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	/* end time */
	param.EndType, err = svr.checkTimeFormat(ctx, param.EndTime, false)
	if err != nil {
		rsp.Message = "Wrong Parameter. (end_time)"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	err = svr.checkTimePeriod(ctx, param.StartTime, param.StartType, param.EndTime, param.EndType)
	if err != nil {
		rsp.Message = err.Error()
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	if param.IntervalType != "" {
		if param.IntervalType, err = svr.checkTimeUnit(ctx, param.IntervalType); err != nil {
			rsp.Message = "Wrong Parameter. (interval_type) : form must be sec,min,hour,day"
			ctx.JSON(http.StatusUnprocessableEntity, rsp)
			return
		}
	} else {
		param.IntervalType = "SEC"
	}

	/* interval value */
	if param.IntervalValue == "" {
		param.IntervalValue = "1"
	}

	/* limit count */
	if param.Limit != "" {
		if check := svr.checkSelectValueLimit(ctx, param.Limit, currentPlan.limitSelectValue); check != "" {
			rsp.Message = check
			ctx.JSON(http.StatusUnprocessableEntity, rsp)
			return
		}
	} else {
		param.Limit = fmt.Sprintf("%d", currentPlan.limitSelectValue)
	}

	/* direction type */
	if param.Direction != "" {
		if param.Direction != "0" && param.Direction != "1" {
			svr.log.Info("direction range over")
			rsp.Message = "Wrong Parameter. (direction) : must be 0, 1"
			ctx.JSON(http.StatusUnprocessableEntity, rsp)
			return
		}
	} else {
		// nfx #128 해결 후 삭제 예정
		param.Direction = "0"
	}

	/* get Interpolation type (reserved) */
	if param.Interpolation > 3 || param.Interpolation < 0 {
		svr.log.Info("%s interpolation range over : %d", trackId, param.Interpolation)
		rsp.Message = "Wrong Parameter. (interpolation) : form must be 0,1,2,3"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	sqlText := "SELECT * FROM ("
	sqlText += "SELECT NAME, "
	sqlText += makeTimeColumn(makeDateTrunc(param.IntervalType, "TIME", param.IntervalValue), param.DateFormat, "TIME") + ", "
	sqlText += "VALUE "
	sqlText += "FROM " + param.TableName + " "
	sqlText += "WHERE " + makeInCondition("NAME", param.TagList, false, true)
	if param.StartType == "date" {
		sqlText += makeBetweenCondition("TIME", makeToDate(param.StartTime), makeToDate(param.EndTime), true) + ") "
	} else {
		sqlText += makeBetweenCondition("TIME", svr.makeFromTimestamp(ctx, param.StartTime), svr.makeFromTimestamp(ctx, param.EndTime), true) + ") "
	}

	sqlText += makePivotCondition(makeCalculator("VALUE", param.CalcMode), makeInCondition("NAME", param.TagList, false, true)) + " "

	if param.Direction != "" {
		sColumnList := []string{"TIME"}
		sSortList := []string{param.Direction}
		sqlText += makeOrderBy(sColumnList, sSortList) + " "
	}

	sqlText += makeLimit(param.Offset, param.Limit)

	svr.log.Debug(trackId, "query : ", sqlText)

	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		rsp.Message = err.Error()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	defer conn.Close()

	dbData, err := svr.getData(ctx, conn, sqlText, param.Scale)
	if err != nil {
		svr.log.Info(trackId, "get data error : ", err.Error())
		rsp.Message = err.Error()
		ctx.JSON(http.StatusFailedDependency, rsp)
		return
	}

	data := MakeReturnFormat(dbData, param.CalcMode, param.ReturnType, "log", param.TagList)

	rsp.Status = "success"
	rsp.Data = data

	svr.log.Trace(trackId, "select pivot data success")

	ctx.JSON(http.StatusOK, rsp)
}

func MakeReturnFormat(dbData *MachbaseResult, mode, format, dataType string, tagList []string) *SelectReturn {
	resultData := &SelectReturn{}

	resultData.CalcMode = mode
	if len(dbData.Columns) > 0 {
		if dbData.Columns[0].Name == "NAME" {
			resultData.Columns = dbData.Columns[1:]
		} else {
			resultData.Columns = dbData.Columns
		}
	}

	if len(dbData.Data) < 1 {
		resultData.Samples = make([]ReturnData, 0)
		return resultData
	}

	switch format {
	case "0":
		if dataType == "tag" {
			resultData.Samples = ConvertFormat0(dbData, tagList)
		} else {
			resultData.Samples = ConvertFormat0Log(dbData)
		}
	case "1":
		if dataType == "tag" {
			resultData.Samples = ConvertFormat1(dbData, tagList)
		} else {
			resultData.Samples = ConvertFormat1Log(dbData)
		}
	}

	return resultData
}

func ConvertFormat1Log(dbData *MachbaseResult) []ReturnDataPivot {
	var (
		returnData ReturnDataPivot          = ReturnDataPivot{}
		dataSet    map[string][]interface{} = make(map[string][]interface{})
		rowList    []ReturnDataPivot        = make([]ReturnDataPivot, 0)
		data       []interface{}            = nil
	)

	for idx, value := range dbData.Columns {
		data = make([]interface{}, 0)
		for _, row := range dbData.Data {
			data = append(data, row[idx])
		}

		dataSet[value.Name] = data
	}

	returnData.Data = dataSet
	rowList = append(rowList, returnData)

	return rowList
}

func ConvertFormat1(dbData *MachbaseResult, tagList []string) []ReturnData {
	rowList := make([]ReturnData, 0)
	dataChannel := make(chan ReturnData, len(tagList))

	wg := sync.WaitGroup{}
	for _, tagName := range tagList {
		wg.Add(1)

		go func(name string) {
			var (
				returnData ReturnData               = ReturnData{}
				dataSet    map[string][]interface{} = make(map[string][]interface{})
				data       []interface{}            = nil
				count      int                      = 0
			)

			defer wg.Done()

			for idx, value := range dbData.Columns {
				if idx == 0 {
					continue
				}

				data = make([]interface{}, 0)

				for _, row := range dbData.Data {
					switch nv := row[0].(type) {
					case *string:
						if *nv != name {
							continue
						}
					case string:
						if nv != name {
							continue
						}
					default:
						continue
					}

					data = append(data, row[idx])
					count++
				}

				dataSet[value.Name] = data
			}

			returnData.TagName = name
			returnData.Data = dataSet

			if count != 0 {
				dataChannel <- returnData
			}
		}(tagName)
	}

	wg.Wait()
	close(dataChannel)

	for row := range dataChannel {
		rowList = append(rowList, row)
	}

	return rowList
}

func ConvertFormat0Log(dbData *MachbaseResult) []ReturnDataPivot {
	var (
		returnData ReturnDataPivot          = ReturnDataPivot{}
		dataSet    []map[string]interface{} = make([]map[string]interface{}, 0)
		rowList    []ReturnDataPivot        = make([]ReturnDataPivot, 0)
		data       map[string]interface{}   = make(map[string]interface{})
	)

	for _, sValue := range dbData.Data {
		for i := 0; i < len(sValue); i++ {
			data[dbData.Columns[i].Name] = sValue[i]
		}

		dataSet = append(dataSet, data)
		data = make(map[string]interface{})
	}

	returnData.Data = dataSet
	rowList = append(rowList, returnData)

	return rowList
}

func ConvertFormat0(dbData *MachbaseResult, tagList []string) []ReturnData {
	rowList := []ReturnData{}
	dataChannel := make(chan ReturnData, len(tagList))

	wg := sync.WaitGroup{}
	for _, tagName := range tagList {
		wg.Add(1)

		go func(name string) {
			returnData := ReturnData{}
			dataSet := make([]map[string]interface{}, 0)
			data := make(map[string]interface{})

			defer wg.Done()

			for _, value := range dbData.Data {
				switch nv := value[0].(type) {
				case *string:
					if *nv != name {
						continue
					}
				case string:
					if nv != name {
						continue
					}
				default:
					continue
				}

				for i := 1; i < len(value); i++ {
					data[dbData.Columns[i].Name] = value[i]
				}

				dataSet = append(dataSet, data)
				data = make(map[string]interface{})
			}

			returnData.TagName = name
			returnData.Data = dataSet

			if dataSet != nil {
				dataChannel <- returnData
			}
		}(tagName)
	}

	wg.Wait()
	close(dataChannel)

	for sRow := range dataChannel {
		rowList = append(rowList, sRow)
	}

	return rowList
}

func makePivotCondition(column, inCondition string) string {
	return fmt.Sprintf("PIVOT (%s FOR %s)", column, inCondition)
}

func makeOrderBy(columns, sortList []string) string {
	result := "ORDER BY "
	format := "%s %s"

	for idx, value := range sortList {
		switch value {
		case "0":
			sortList[idx] = "ASC"
		case "1":
			sortList[idx] = "DESC"
		}
	}

	if len(columns) > 0 {
		result += fmt.Sprintf(format, columns[0], sortList[0])
	}

	for i := 1; i < len(columns); i++ {
		result += ", " + fmt.Sprintf(format, columns[i], sortList[i])
	}

	return result
}

func makeGroupBy(columns []string) string {
	result := "GROUP BY "

	if len(columns) > 0 {
		result += columns[0]
	}

	for i := 1; i < len(columns); i++ {
		result += ", " + columns[i]
	}

	return result
}

func makeRollupHint(timeColumn, intervalType, calcType, valueColumn string) string {
	if (intervalType != "SEC") && (intervalType != "MIN") {
		intervalType = "HOUR"
	}

	return fmt.Sprintf("%s ROLLUP 1 %s %s, %s(%s) %s", timeColumn, intervalType, timeColumn, calcType, valueColumn, valueColumn)
}

func makeCalculator(column, calcType string) string {
	if calcType == "COUNT" || calcType == "SUMSQ" {
		calcType = "SUM"
	}
	return fmt.Sprintf("%s(%s)", calcType, column)
}

func makeDateTrunc(intervalType, timeColumn, intervalValue string) string {
	result := ""
	switch intervalType {
	case "SEC", "MIN", "HOUR":
		result = fmt.Sprintf("DATE_TRUNC('%s', %s, %s)", intervalType, timeColumn, intervalValue)
	case "DAY":
		result = fmt.Sprintf("%s / (%s*86400*1000000000) * (%s*86400*1000000000)", timeColumn, intervalValue, intervalValue)
	}
	return result
}

func (svr *httpd) checkTimeUnit(ctx *gin.Context, intervalType string) (string, error) {
	var err error = nil
	intervalType = strings.ToUpper(intervalType)
	switch intervalType {
	case "SEC", "S":
		intervalType = "SEC"
	case "MIN", "M":
		intervalType = "MIN"
	case "HOUR", "H":
		intervalType = "HOUR"
	case "DAY", "D":
		intervalType = "DAY"
	default:
		svr.log.Infof("%s '%s' format not supported\n", ctx.Value(HTTP_TRACKID), intervalType)
		err = fmt.Errorf("wrong format : '%s' not supported", intervalType)
	}

	return intervalType, err
}

func (svr *httpd) checkCalcUnit(ctx *gin.Context, calcMode string) (string, error) {
	var err error = nil
	trackId := ctx.Value(HTTP_TRACKID)

	if calcMode == "" {
		svr.log.Info(trackId, "value is nil")
		err = fmt.Errorf("wrong format : value is nil")
		return calcMode, err
	}

	calcMode = strings.ToUpper(calcMode)

	switch calcMode {
	case "MIN", "MAX", "AVG", "SUM", "SUMSQ":
		svr.log.Debugf("%s '%s' is available function", trackId, calcMode)
	case "CNT", "COUNT":
		calcMode = "COUNT"
		svr.log.Debugf("%s '%s' is available function", trackId, calcMode)
	default:
		svr.log.Infof("%s '%s' format not supported\n", trackId, calcMode)
		err = fmt.Errorf("wrong format : '%s' not supported", calcMode)
	}

	return calcMode, err
}

func (svr *httpd) getMetaData(ctx context.Context, conn api.Conn, sqlText string) (*MetaResult, error) {
	result := &MetaResult{}

	rows, err := conn.Query(ctx, sqlText)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	for rows.Next() {
		meta := ""
		err = rows.Scan(&meta)
		if err != nil {
			svr.log.Warn("rows.Scan error : ", err.Error())
			return result, err
		}

		result.Data = append(result.Data, map[string]interface{}{"name": meta})
	}

	return result, nil
}

// scale 적용, 데이터 받은 후에 수정
func (svr *httpd) getData(ctx context.Context, conn api.Conn, sqlText string /*scale*/, _ int) (*MachbaseResult, error) {
	result := &MachbaseResult{}

	rows, err := conn.Query(ctx, sqlText)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return result, err
	}
	colsLen := len(cols.Names())
	colsList := make([]MachbaseColumn, colsLen)

	for idx, col := range cols {
		colsList[idx].Name = col.Name
		colsList[idx].Type = int(col.DataType.ColumnType())
	}

	for rows.Next() { // scale 적용을 어떻게 할 건가, 컬럼 여러개일때 value 컬럼을 찾아서 처리가 가능한가? ( rows.columns 으로 순서 확인 가능? )
		buffer, err := cols.MakeBuffer()
		if err != nil {
			svr.log.Warn("make buffer error : ", err.Error())
			return result, err
		}
		err = rows.Scan(buffer...)
		if err != nil {
			svr.log.Warn("scan error : ", err.Error())
			return result, err
		}
		result.Data = append(result.Data, buffer)
	}

	result.Columns = colsList

	return result, nil
}

func makeLimit(offset, limit string) string {
	if offset != "" {
		return fmt.Sprintf("LIMIT %s, %s", offset, limit)
	} else {
		return fmt.Sprintf("LIMIT %s", limit)
	}
}

func makeAndCondition(str, sep string, flag bool) string {
	result := ""

	conditionArr := strings.Split(str, sep)
	if len(conditionArr) > 0 {
		if conditionArr[0] != "" {
			if flag {
				result = " AND "
			}
			result += conditionArr[0]
		}

		for i := 1; i < len(conditionArr); i++ {
			result = result + " AND " + conditionArr[i]
		}
		result += " "
	}

	return result
}

func (svr *httpd) makeNanoTimeStamp(ctx *gin.Context, time string) string {
	var (
		sGap  int    = 0
		sPow  int64  = 0
		sRes  string = ""
		sTime int64  = 0
		err   error  = nil
	)

	trackId := ctx.Value(HTTP_TRACKID)
	sGap = 19 - len(time)
	sPow = int64(math.Pow10(sGap))

	sTime, err = strconv.ParseInt(time, 10, 64)
	if err != nil {
		svr.log.Info(trackId, "value is not TimeStamp : ", time)
		return sRes
	}

	sRes = strconv.FormatInt((sTime * sPow), 10)

	return sRes
}

func (svr *httpd) makeFromTimestamp(ctx *gin.Context, times string) string {
	var (
		sRes    string = ""
		sTime   string = ""
		sLength int    = len(times)
		err     error  = nil
	)

	if _, err = strconv.ParseInt(times, 10, 64); err == nil {
		if sLength > 13 {
			times = times[0:13]
		}

		if sTime = svr.makeNanoTimeStamp(ctx, times); sTime != "" {
			sRes = fmt.Sprintf("FROM_TIMESTAMP(%s)", sTime)
		}
	}

	return sRes
}

func makeToDate(times string) string {
	result := ""
	length := len(times)

	if length == 19 {
		times = times[:10] + " " + times[11:]
		result = fmt.Sprintf("TO_DATE('%s')", times)
	} else if length > 19 {
		times = times[:10] + " " + times[11:19] + " " + times[20:23]
		result = fmt.Sprintf("TO_DATE('%s', 'YYYY-MM-DD HH24:MI:SS mmm')", times)
	}

	return result
}

func makeBetweenCondition(column, value1, value2 string, flag bool) string {
	format := "%s BETWEEN %s AND %s"
	result := fmt.Sprintf(format, column, value1, value2)

	if flag {
		result = " AND " + result
	}

	return result
}

func makeInCondition(column string, value []string, flag, stringFlag bool) string {
	result := column + " IN(%s)" // NAME IN()
	list := ""
	format := "'%s'"

	if !stringFlag {
		format = "%s"
	}

	if len(value) > 0 {
		list = fmt.Sprintf(format, value[0])
	}

	for i := 1; i < len(value); i++ {
		list += "," + fmt.Sprintf(format, value[i])
	}

	result = fmt.Sprintf(result, list)

	if flag {
		result = " AND " + result
	}

	return result
}

func makeLikeTag(tag string) string {
	split := strings.Split(tag, ".")
	text := strings.Join(split[:2], ".")
	text = fmt.Sprintf("'%s.%%'", text)
	return text
}

func makeValueColumn(columns, aliases []string) string {
	result := ""
	colNameFormat := `, "%s"`
	aliasFormat := ` AS "%s"`

	if len(aliases) > 0 {
		for idx, name := range columns {
			result += fmt.Sprintf(colNameFormat, strings.TrimSpace(name)) // , "value"
			if aliases[idx] != "" {
				result += fmt.Sprintf(aliasFormat, strings.TrimSpace(aliases[idx])) // , "value" AS "value"
			}
		}
	} else {
		for _, name := range columns {
			result += fmt.Sprintf(colNameFormat, strings.TrimSpace(name)) // , "value" , "level", "job" ...
		}
	}

	return result
}

/*
make time column

	parameter
		aColumn : time column -> DATE_TRUNC('SEC', TIME, 1), TIME...
		aFormat : date_format parameter
		aAlias   : check alias
*/
func makeTimeColumn(column, format string, alias string) string {
	result := ""
	formatUpper := strings.ToUpper(format)

	switch formatUpper {
	case "NANOSECOND", "NS", "NANO": // SELECT /*+ SCAN_BACKWARD(TAG) */ NAME, TO_TIMESTAMP(TIME) AS TIME
		result = fmt.Sprintf("TO_TIMESTAMP(%s)", column)
	case "MICROSECOND", "US", "MICRO":
		result = fmt.Sprintf("TO_TIMESTAMP(%s%s)", column, "/1000")
	case "MILLISECOND", "MS", "MILLI":
		result = fmt.Sprintf("TO_TIMESTAMP(%s%s)", column, "/1000000")
	case "SECOND", "S", "SEC":
		result = fmt.Sprintf("TO_TIMESTAMP(%s%s)", column, "/1000000000")
	case "":
		result = column
	default:
		result = fmt.Sprintf("TO_CHAR(%s, '%s')", column, format)
	}

	if alias != "" {
		result += fmt.Sprintf(" AS %s", alias)
	}

	return result
}

func makeScanHint(flag, tableName string) string {
	if flag == "1" {
		return fmt.Sprintf("/*+ SCAN_BACKWARD(%s) */ ", tableName)
	} else {
		return ""
	}
}

func (svr *httpd) checkSelectValueLimit(_ *gin.Context, limit string, limitSelectValue int64) string {
	result := ""
	limitInt, err := strconv.ParseInt(limit, 10, 64)
	if err != nil {
		svr.log.Info("ParseInt error : ", err.Error())
		result = "limit param is not number"
	} else if limitInt > limitSelectValue {
		result = fmt.Sprintf("limit over. (parameter:%d, Available:%d)", limitInt, limitSelectValue)
		svr.log.Info(result)
	}

	return result
}

func (svr *httpd) checkTimeFormat(_ *gin.Context, timeValue string, nilOk bool) (string, error) {
	var err error
	var timeType string

	if timeValue == "" {
		if nilOk { // ?
			svr.log.Info("base time is nil")
			return "", nil
		} else {
			svr.log.Info("time is nil")
			return "", fmt.Errorf("time is nil")
		}
	}

	svr.log.Trace("time value : ", timeValue)

	_, err = strconv.ParseInt(timeValue, 10, 64)
	if err == nil {
		if len(timeValue) < 10 {
			svr.log.Infof("wrong format (%s)", timeValue)
			err = fmt.Errorf("wrong format (%s)", timeValue)
		} else {
			timeType = "timestamp"
			svr.log.Debugf("format : timestamp(%s)", timeValue)
		}
	} else {
		//ex: 2023-05-16.99:10:20.123.456.789
		matched := regexp.MustCompile(`[\d]{4}-[\d]{2}-[\d]{2}.\d{2}:\d{2}:\d{2}(.\d{3}){0,3}$`)
		if matched.MatchString(timeValue) {
			err = nil
			timeType = "date"
			svr.log.Debug("format : date format")
		} else {
			svr.log.Infof("wrong format (%s)", timeValue)
			err = fmt.Errorf("wrong format (%s)", timeValue)
		}
	}

	return timeType, err
}

func (svr *httpd) checkTimePeriod(_ *gin.Context, startTime, startType, endTime, endType string) error {
	if startType != endType {
		svr.log.Info("StartTime, EndTime Format Different")
		return fmt.Errorf("StartTime, EndTime Format Different")
	}

	if startType == "date" { //2023-05-16.99:10:20.123.456.789 ==> 2023-05-16 99:10:20 123456789
		startTime = strings.Replace(startTime, ".", " ", -1)
		endTime = strings.Replace(endTime, ".", " ", -1)
	} else {
		if len(startTime) == 19 { // len(unixnano) : 19  /  len(unix) : 10
			startTime = startTime[:10] + " " + startTime[11:]
		} else if len(startTime) > 19 {
			startTime = startTime[:10] + " " + startTime[11:19] + " " + startTime[20:23]
		}

		// startTime && endTime

		if len(endTime) == 19 {
			endTime = endTime[:10] + " " + endTime[11:]
		} else if len(endTime) > 19 {
			endTime = endTime[:10] + " " + endTime[11:19] + " " + endTime[20:23]
		}
	}

	if endTime <= startTime {
		svr.log.Info("EndTime less than StartTime")
		return fmt.Errorf("EndTime less than StartTime")
	} else {
		return nil
	}
}

type (
	SelectRaw struct {
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
		TableName    string `form:"table_name,default=TAG" json:"table_name"`
	}
	SelectCalc struct {
		Timezone      string `form:"timezone" json:"timezone"`
		TagName       string `form:"tag_name" json:"tag_name"`
		CalcMode      string `form:"calc_mode" json:"calc_mode"`
		Separator     string `form:"separator" json:"separator"`
		DateFormat    string `form:"date_format" json:"date_format"`
		StartTime     string `form:"start_time" json:"start_time"`
		EndTime       string `form:"end_time" json:"end_time"`
		IntervalType  string `form:"interval_type" json:"interval_type"`
		IntervalValue string `form:"interval_value" json:"interval_value"`
		Limit         string `form:"limit" json:"limit"`
		Offset        string `form:"offset" json:"offset"`
		Direction     string `form:"direction" json:"direction"`
		ReturnType    string `form:"value_return_form" json:"value_return_form"`
		Scale         int    `form:"scale" json:"scale"`
		Interpolation int    `form:"interpolation" json:"interpolation"`
		StartType     string
		EndType       string
		TagList       []string
		TableName     string `form:"table_name,default=TAG" json:"table_name"`
	}
)

type lakeReq interface {
	lakeRequest()
	count() int
	reset()
	next() ([]any, error)
}

var _ lakeReq = (*lakeDefaultReq)(nil)
var _ lakeReq = (*lakeStandardReq)(nil)

type lakeDefaultValue struct {
	Tag string
	Ts  int64
	Val float64
}

type lakeDefaultReq struct {
	Values []*lakeDefaultValue `json:"values"`
	cursor int                 `json:"-"`
}

func (r *lakeDefaultReq) lakeRequest() {}
func (r *lakeDefaultReq) count() int   { return len(r.Values) }
func (r *lakeDefaultReq) reset()       { r.cursor = 0 }

func (r *lakeDefaultReq) next() ([]any, error) {
	if r.cursor >= len(r.Values) {
		return nil, io.EOF
	}
	r.cursor++
	v := r.Values[r.cursor-1]
	return []any{v.Tag, v.Ts, v.Val}, nil
}

type lakeStandardValue [2]any

type lakeStandardReq struct {
	Tag        string              `json:"tag_name"`
	Dateformat string              `json:"date_format"`
	Values     []lakeStandardValue `json:"values"`
	cursor     int                 `json:"-"`
	timeParser *ymd.Parser         `json:"-"`
}

func (r *lakeStandardReq) lakeRequest() {}
func (r *lakeStandardReq) count() int   { return len(r.Values) }
func (r *lakeStandardReq) reset()       { r.cursor = 0 }

func (r *lakeStandardReq) next() ([]any, error) {
	if r.cursor >= len(r.Values) {
		return nil, io.EOF
	}
	var ts time.Time
	var val float64
	var err error

	rec := r.Values[r.cursor]
	if len(rec) != 2 {
		return nil, fmt.Errorf("values[%d] should have (time, value), got %d elements", r.cursor, len(rec))
	}

	switch tv := rec[0].(type) {
	case string:
		ts, err = r.timeParser.Parse(tv)
		if err != nil {
			return nil, fmt.Errorf("values[%d] has wrong timeformat %q, format:%q", r.cursor, tv, r.Dateformat)
		}
	case int64:
		ts = time.Unix(0, tv)
	default:
		return nil, fmt.Errorf("values[%d] has wrong time in %T (%v)", r.cursor, tv, tv)
	}
	switch vs := rec[1].(type) {
	case float64:
		val = vs
	case int:
		val = float64(vs)
	default:
		return nil, fmt.Errorf("values[%d] has wrong value in %T (%v)", r.cursor, vs, vs)
	}
	r.cursor++
	return []any{r.Tag, ts, val}, nil
}

type lakeRsp struct {
	Success bool        `json:"success"`
	Reason  string      `json:"reason"`
	Data    interface{} `json:"data,omitempty"`
}

func (svr *httpd) handleLakePostValues(ctx *gin.Context) {
	rsp := lakeRsp{Success: false}

	// legacy interface for a client uses cli to write data
	dataType := ctx.Param("type")

	var req lakeReq
	var err error

	switch dataType {
	case "standard":
		stdReq := lakeStandardReq{}
		err = ctx.Bind(&stdReq)
		if stdReq.Dateformat == "" {
			stdReq.Dateformat = `YYYY-MM-DD HH24:MI:SS mmm:uuu:nnn`
		}
		stdReq.timeParser = ymd.NewParser(stdReq.Dateformat).WithLocation(time.Local)
		req = &stdReq
		svr.log.Tracef("bind: %+v", stdReq.Values)
	default:
		defReq := lakeDefaultReq{}
		err = ctx.Bind(&defReq)
		svr.log.Tracef("bind: %+v", defReq.Values)
		req = &defReq
	}

	if err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusPreconditionFailed, rsp)
		return
	}

	if req.count() == 0 {
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

	appender, err := conn.Appender(ctx, "TAG")
	if err != nil {
		svr.log.Error("appender error: ", err)
		ctx.JSON(http.StatusInternalServerError, err)
		return
	}

	defer func() {
		succ, fail, err := appender.Close()
		data := map[string]any{
			"success": succ,
			"fail":    fail,
		}
		if err != nil {
			data["close"] = err.Error()
		}
		rsp.Data = data
		if rsp.Success {
			ctx.JSON(http.StatusOK, rsp)
		} else {
			ctx.JSON(http.StatusInternalServerError, rsp)
		}
	}()

	for {
		data, err := req.next()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				rsp.Reason = err.Error()
				return
			}
		}
		err = appender.Append(data...)
		if err != nil {
			rsp.Reason = err.Error()
			return
		}
	}

	rsp.Success = true
	rsp.Reason = "success"
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
	defer conn.Close()

	result, err := svr.getExec(ctx, conn, query.Sql)
	if err != nil {
		svr.log.Info("get data error : ", err.Error())
		rsp.Message = err.Error()
		ctx.JSON(http.StatusFailedDependency, rsp)
		return
	}

	rsp.Status = "success"
	rsp.Data = result

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

func (svr *httpd) getExec(ctx context.Context, conn api.Conn, sqlText string) (*ExecResult, error) {
	result := &ExecResult{}
	rows, err := conn.Query(ctx, sqlText)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return result, err
	}
	colsLen := len(cols.Names())
	colsList := make([]MachbaseColumn, colsLen)

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()

		for idx, col := range cols {
			colsList[idx].Name = col.Name
			colsList[idx].Type = int(col.DataType.ColumnType())
		}
	}()

	for rows.Next() {
		buffer, err := cols.MakeBuffer()
		if err != nil {
			svr.log.Warn("make buffer error : ", err.Error())
			return result, err
		}
		err = rows.Scan(buffer...)
		if err != nil {
			svr.log.Warn("scan error : ", err.Error())
			return result, err
		}

		mv := map[string]any{}
		mv["name"] = buffer[0]
		mv["time"] = buffer[1]
		mv["value"] = buffer[2]
		result.Data = append(result.Data, mv)
	}

	wg.Wait()

	result.Columns = colsList

	return result, nil
}
