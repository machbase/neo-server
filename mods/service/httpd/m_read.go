package httpd

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

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
type MachbaseResult struct {
	ErrorCode    int              `json:"errorCode"`
	ErrorMessage string           `json:"errorMessage"`
	Columns      []MachbaseColumn `json:"columns"`
	Data         [][]interface{}  `json:"data"`
}

type MachbaseColumn struct {
	Name   string `json:"name"`
	Type   string `json:"type"` // 기존은 int 형,
	Length int    `json:"length"`
}

type SelectReturn struct {
	Columns []MachbaseColumn `json:"columns"`
	Rows    [][]interface{}  `json:"rows"`
}

const (
	MACHLAKE_PLAN_TINY       = "TINY"
	MACHLAKE_PLAN_BASIC      = "BASIC"
	MACHLAKE_PLAN_BUSINESS   = "BUSINESS"
	MACHLAKE_PLAN_ENTERPRISE = "ENTERPRISE"

	HTTP_TRACKID = "cemlib/trackid"
)

var lakePlanMap = map[string]planLimit{}
var localPlan string

func init() {
	// =========== 임시 테스트 ================
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

/* unused
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
*/

func (svr *httpd) GetRawData(ctx *gin.Context) {
	trackId := ctx.GetString(HTTP_TRACKID)
	svr.log.Trace(trackId, "start RawData()")

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

	svr.log.Info(timezone) // 에러 방지

	currentPlan := lakePlanMap[localPlan]

	// plan을 알아야 LimitSelTag 값을 알 수 있음
	if param.TagName != "" {
		param.TagList = strings.Split(param.TagName, param.Separator)
		if len(param.TagList) > currentPlan.limitSelectTag { // lakeserver conf 값,   mysql 에서 데이터 로드 필요
			rsp.Reason = fmt.Sprintf("tag count over. (parameter:%d, Available:%d)", len(param.TagList), currentPlan.limitSelectTag)
			svr.log.Info(rsp.Reason)
			ctx.JSON(http.StatusUnprocessableEntity, rsp)
			return
		}
	} else {
		svr.log.Info("Tag name is empty")
		rsp.Reason = "Wrong Parameter. (tagname) : must be a least 1"
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
		rsp.Reason = "Wrong Parameter. (startTime)"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	//end time
	param.EndType, err = svr.checkTimeFormat(ctx, param.EndTime, false)
	if err != nil {
		rsp.Reason = "Wrong Parameter. (endTime)"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	err = svr.checkTimePeriod(ctx, param.StartTime, param.StartType, param.EndTime, param.EndType)
	if err != nil {
		rsp.Reason = err.Error()
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
			rsp.Reason = "The number of 'columns' and 'aliases' is different"
			ctx.JSON(http.StatusUnprocessableEntity, rsp)
			return
		}
	}

	// limit count
	if param.Limit != "" {
		if check := svr.checkSelectValueLimit(ctx, param.Limit, currentPlan.limitSelectValue); check != "" {
			rsp.Reason = check
			ctx.JSON(http.StatusUnprocessableEntity, rsp)
			return
		}
	} else { // 일반적으로 limit을 param으로 받아오는지?
		param.Limit = fmt.Sprintf("%d", currentPlan.limitSelectValue)
	}

	// get direction type
	if param.Direction != "" {
		if param.Direction != "0" && param.Direction != "1" {
			svr.log.Info("direction range over")
			rsp.Reason = "Wrong Parameter. (direction) : must be 0, 1"
			ctx.JSON(http.StatusUnprocessableEntity, rsp)
			return
		}
	} else {
		// nfx #128 해결 후 삭제 예정
		param.Direction = "0"
	}

	sqlText := "SELECT "
	sqlText += makeScanHint(param.Direction, "TAG")                           // SELECT /*+ SCAN_BACKWARD(TAG) */
	sqlText += "NAME, "                                                       // SELECT /*+ SCAN_BACKWARD(TAG) */ NAME,
	sqlText += makeTimeColumn("TIME", param.DateFormat, "TIME")               // SELECT /*+ SCAN_BACKWARD(TAG) */ NAME, TO_TIMESTAMP(TIME) AS TIME
	sqlText += makeValueColumn(param.ColumnList, param.AliasList) + " "       // SELECT /*+ SCAN_BACKWARD(TAG) */ NAME, TO_TIMESTAMP(TIME) AS TIME , "value" AS "value"
	sqlText += "FROM " + "TAG" + " "                                          // SELECT /*+ SCAN_BACKWARD(TAG) */ NAME, TO_TIMESTAMP(TIME) AS TIME , "value" AS "value" FROM TAG
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
	dbData, err := svr.getData(sqlText, param.Scale)
	if err != nil {
		svr.log.Info(trackId, "get Data error : ", err.Error())
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	data := SelectReturn{Columns: dbData.Columns}

	if dbData.Data == nil {
		data.Rows = make([][]interface{}, 0)
	} else {
		data.Rows = dbData.Data
	}

	rsp.Success = true
	rsp.Reason = "success"
	rsp.Data = data

	svr.log.Trace(trackId, "select raw data success")

	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) GetCurrentData(ctx *gin.Context) {
	trackId := ctx.GetString(HTTP_TRACKID)
	svr.log.Trace(trackId, "start GetCurrentData()")
	rsp := lakeRsp{Success: false, Reason: "not specified"}

	param := SelectRaw{}

	err := ctx.ShouldBind(&param)
	if err != nil {
		svr.log.Info(trackId, "bind error : ", err.Error())
		rsp.Reason = "get parameter failed"
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	svr.log.Debugf("%s request data %+v", trackId, param)

	// machbaseCLI 통해서 데이터 가져올때 timezone을 설정 후 쿼리,
	// neo는 따로 설정이 없음,
	timezone, sError := svr.makeTimezone(ctx, param.Timezone)
	if sError != nil {
		rsp.Reason = sError.Error()
		ctx.JSON(http.StatusUnprocessableEntity, rsp)
		return
	}

	svr.log.Info(timezone) // 에러 방지

	if param.Separator == "" {
		param.Separator = ","
	}

	currentPlan := lakePlanMap[localPlan]

	if param.TagName != "" {
		param.TagList = strings.Split(param.TagName, param.Separator)

		if len(param.TagList) > currentPlan.limitSelectTag {
			rsp.Reason = fmt.Sprintf("tag count over. (parameter:%d, Available:%d)", len(param.TagList), currentPlan.limitSelectTag)
			svr.log.Info(trackId, rsp.Reason)
			ctx.JSON(http.StatusUnprocessableEntity, rsp)
			return
		}
	} else {
		svr.log.Info(trackId, "Tag name is nil")
		rsp.Reason = "Wrong Parameter. (tag_name) : must be at least 1"
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
			rsp.Reason = "The number of 'columns' and 'aliases' is different"
			ctx.JSON(http.StatusUnprocessableEntity, rsp)
			return
		}
	}

	//  SELECT /*+ SCAN_BACKWARD(TAG) */ NAME, TO_TIMESTAMP(TIME) AS TIME, VALUE FROM TAG
	sqlText := "SELECT " + makeScanHint("1", "TAG") //
	sqlText += "NAME, "
	sqlText += makeTimeColumn("TIME", param.DateFormat, "TIME")
	sqlText += makeValueColumn(param.ColumnList, param.AliasList) + " "
	sqlText += "FROM " + "TAG"

	data := SelectReturn{}
	dataChannel := make(chan []interface{}, len(param.TagList))

	wg := sync.WaitGroup{}
	for idx, tagName := range param.TagList {
		wg.Add(1)

		go func(svr *httpd, where string, idx int) {
			defer wg.Done()

			sql := fmt.Sprintf("%s %s", sqlText, where)
			svr.log.Debugf("%s [%d] query : %s", trackId, idx, sql)

			dbData, err := svr.getData(sql, param.Scale)
			if err != nil {
				svr.log.Infof("%s [%d] get data error : %s", trackId, idx, err.Error())
				return
			}

			data.Columns = dbData.Columns //  columns 는 slice인데 append가 아닌 대입만 하는 이유는? 어차피 컬럼이 똑같아서 첫번째만 대입?

			// add success select data
			if len(dbData.Data) > 0 {
				if len(dbData.Data[0]) > 0 {
					dataChannel <- dbData.Data[0]
				}
			}
		}(svr, fmt.Sprintf("WHERE NAME='%s' LIMIT 1", tagName), idx)
	}

	wg.Wait()
	close(dataChannel)

	for row := range dataChannel {
		data.Rows = append(data.Rows, row)
	}

	rsp.Success = true
	rsp.Reason = "success"
	rsp.Data = data

	svr.log.Trace(trackId, "select current data success")

	ctx.JSON(http.StatusOK, rsp)
}

// scale 적용, 데이터 받은 후에 수정
func (svr *httpd) getData(sqlText string, scale int) (*MachbaseResult, error) {
	result := &MachbaseResult{}

	rows, err := svr.db.Query(sqlText)
	if err != nil {
		return result, err
	}

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
			colsList[idx].Type = col.Type
			colsList[idx].Length = col.Length
		}
	}()

	for rows.Next() { // scale 적용을 어떻게 할 건가, 컬럼 여러개일때 value 컬럼을 찾아서 처리가 가능한가?
		row := make([]any, colsLen)
		err = rows.Scan(row...)
		if err != nil {
			return result, err
		}
		result.Data = append(result.Data, row)
	}

	wg.Wait()

	result.Columns = colsList

	return result, nil
}

func makeLimit(aOffset, aLimit string) string {
	if aOffset != "" {
		return fmt.Sprintf("LIMIT %s, %s", aOffset, aLimit)
	} else {
		return fmt.Sprintf("LIMIT %s", aLimit)
	}
}

func makeAndCondition(aStr, aSep string, aFlag bool) string {
	var (
		sRes          string   = ""
		sConditionArr []string = nil
	)

	sConditionArr = strings.Split(aStr, aSep)
	if len(sConditionArr) > 0 {
		if sConditionArr[0] != "" {
			if aFlag == true {
				sRes = " AND "
			}
			sRes += sConditionArr[0]
		}

		for i := 1; i < len(sConditionArr); i++ {
			sRes = sRes + " AND " + sConditionArr[i]
		}
		sRes += " "
	}

	return sRes
}

func (svr *httpd) makeNanoTimeStamp(ctx context.Context, aTime string) string {
	var (
		sGap   int    = 0
		sPow   int64  = 0
		sRes   string = ""
		sTime  int64  = 0
		sError error  = nil
	)

	trackId := ctx.Value(HTTP_TRACKID)
	sGap = 19 - len(aTime)
	sPow = int64(math.Pow10(sGap))

	sTime, sError = strconv.ParseInt(aTime, 10, 64)
	if sError != nil {
		svr.log.Info(trackId, "value is not TimeStamp : ", aTime)
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
		sError  error  = nil
	)

	if _, sError = strconv.ParseInt(times, 10, 64); sError == nil {
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

func (svr *httpd) checkSelectValueLimit(ctx context.Context, limit string, limitSelectValue int64) string {
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

func (svr *httpd) checkTimeFormat(ctx context.Context, timeValue string, nilOk bool) (string, error) {
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
		//[\d] = [0-9] , {4} = {반복횟수} ,  ( -  .  ) 일반 문자열로 사용
		// 2023-05-16.99:10:20.123.456.789
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

func (svr *httpd) checkTimePeriod(ctx context.Context, startTime, startType, endTime, endType string) error {
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

// 사용자가 보낸 Timezone을 확인하고 machbase에서 사용 가능한 Timezone으로 변경하는 함수
func (svr *httpd) makeTimezone(ctx *gin.Context, timezone string) (string, error) {
	trackId := ctx.Value(HTTP_TRACKID)
	resultTimezone := ""

	if timezone == "" {
		svr.log.Info(trackId, "custom timezone is nil. use default timezone 'Etc/UTC'")
		resultTimezone = "Etc/UTC"
		return resultTimezone, nil
	}

	matched := regexp.MustCompile(`[+-](0[0-9]|1[0-4])[0-5][0-9]$`)
	if matched.MatchString(timezone) {
		svr.log.Infof("available timezone format : %s", timezone)
		resultTimezone = timezone
		return resultTimezone, nil
	}

	return svr.convertTimezone(ctx, timezone)
}

// convertTimezone 함수만 사용 하는 곳도 존재, 아래 기능이 있으면 makeTimezone 함수와 중복, convert 함수만 사용 가능
func (svr *httpd) convertTimezone(ctx *gin.Context, timezone string) (string, error) {
	trackId := ctx.Value(HTTP_TRACKID)
	resultTimezone := ""

	if timezone == "" {
		svr.log.Info(trackId, "timezone is nil")
		return resultTimezone, fmt.Errorf("must entered timezone name")
	}

	matched := regexp.MustCompile(`[+-](0[0-9]|1[0-4])[0-5][0-9]$`)
	if matched.MatchString(timezone) {
		svr.log.Info(trackId, "available timezone format : ", timezone)
		resultTimezone = timezone
		return resultTimezone, nil
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		svr.log.Info(trackId, "not available Timezone name : ", timezone)
		return resultTimezone, fmt.Errorf("%s is not available Timezone name", timezone)
	}

	sampleDate := time.Date(2021, 1, 1, 12, 0, 0, 0, time.UTC)
	locDate := sampleDate.In(loc).String()
	if len(locDate) < 25 {
		svr.log.Info(trackId, "convert timezone failed : ", locDate)
		return resultTimezone, fmt.Errorf("convert timezone failed")
	}

	resultTimezone = locDate[20:25]                                                     // ex) +0900, -0900
	svr.log.Debugf("%s convert timezone (%s -> %s)", trackId, timezone, resultTimezone) // ex) aTimezone = Asia/Seoul,  sResTimezone = +0900
	return resultTimezone, nil
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
