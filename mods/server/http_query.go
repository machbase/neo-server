package server

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid/v5"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/codec"
	"github.com/machbase/neo-server/v8/mods/codec/opts"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/mods/util/glob"
)

// Execute machbase SQL query
//
// @Summary     Execute query
// @Description execute query
// @Param       q           query   string true "sql query text" default(select * from example limit 3)
// @Success     200  {object}  msg.QueryResponse
// @Failure     400  {object}  msg.QueryResponse
// @Failure     500  {object}  msg.QueryResponse
// @Router      /db/query [get]
func (svr *httpd) handleQuery(ctx *gin.Context) {
	rsp := &QueryResponse{Success: false, Reason: "not specified"}
	tick := time.Now()

	var cypherQ string
	var err error
	req := &QueryRequest{Precision: -1}
	switch ctx.Request.Method {
	case http.MethodPost:
		contentType := ctx.ContentType()
		switch contentType {
		case "application/json":
			req.Timeformat = "ns"
			req.TimeLocation = "UTC"
			req.Format = "json"
			req.Rownum = false
			req.Heading = true
			req.Precision = -1
			if err = ctx.Bind(req); err != nil {
				rsp.Reason = err.Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusBadRequest, rsp)
				return
			}
			if req.SqlText == "" && svr.cypherAlg != "" && svr.cypherKey != "" {
				rr := struct {
					SqlText string `json:"Q"`
				}{}
				ctx.Bind(&rr)
				cypherQ = rr.SqlText
			}
		case "application/x-www-form-urlencoded":
			req.SqlText = ctx.PostForm("q")
			if req.SqlText == "" && svr.cypherAlg != "" && svr.cypherKey != "" {
				cypherQ = ctx.PostForm("Q")
			}
			req.Timeformat = strString(ctx.PostForm("timeformat"), "ns")
			req.TimeLocation = strString(ctx.PostForm("tz"), "UTC")
			req.Format = strString(ctx.PostForm("format"), "json")
			req.Compress = ctx.PostForm("compress")
			req.Rownum = strBool(ctx.PostForm("rownum"), false)
			req.Heading = strBool(ctx.PostForm("heading"), true)
			if h := ctx.Query("header"); h == "skip" {
				req.Heading = false
			}
			req.Precision = strInt(ctx.PostForm("precision"), -1)
			req.Transpose = strBool(ctx.PostForm("transpose"), false)
			req.RowsFlatten = strBool(ctx.PostForm("rowsFlatten"), false)
			req.RowsArray = strBool(ctx.PostForm("rowsArray"), false)
		default:
			rsp.Reason = fmt.Sprintf("unsupported content-type: %s", contentType)
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusBadRequest, rsp)
			return
		}
	case http.MethodGet:
		req.SqlText = ctx.Query("q")
		if req.SqlText == "" && svr.cypherAlg != "" && svr.cypherKey != "" {
			cypherQ = ctx.Query("Q")
		}
		req.Timeformat = strString(ctx.Query("timeformat"), "ns")
		req.TimeLocation = strString(ctx.Query("tz"), "UTC")
		req.Format = strString(ctx.Query("format"), "json")
		req.Compress = ctx.Query("compress")
		req.Rownum = strBool(ctx.Query("rownum"), false)
		req.Heading = strBool(ctx.Query("heading"), true)
		if h := ctx.Query("header"); h == "skip" {
			req.Heading = false
		}
		req.Precision = strInt(ctx.Query("precision"), -1)
		req.Transpose = strBool(ctx.Query("transpose"), false)
		req.RowsFlatten = strBool(ctx.Query("rowsFlatten"), false)
		req.RowsArray = strBool(ctx.Query("rowsArray"), false)
	}

	if len(req.SqlText) == 0 {
		if cypherQ == "" {
			rsp.Reason = "empty sql"
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusBadRequest, rsp)
			return
		} else {
			req.SqlText, err = util.DecryptString(cypherQ, svr.cypherAlg, svr.cypherKey)
			if err != nil {
				rsp.Reason = "decrypt sql fail, " + err.Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusBadRequest, rsp)
				return
			}
		}
	}

	timeLocation, err := util.ParseTimeLocation(req.TimeLocation, time.UTC)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	var output io.Writer
	switch req.Compress {
	case "gzip":
		output = gzip.NewWriter(ctx.Writer)
	default:
		req.Compress = ""
		output = ctx.Writer
	}

	encoder := codec.NewEncoder(req.Format,
		opts.OutputStream(output),
		opts.Timeformat(req.Timeformat),
		opts.Precision(req.Precision),
		opts.Rownum(req.Rownum),
		opts.Header(req.Heading),
		opts.TimeLocation(timeLocation),
		opts.Delimiter(","),
		opts.BoxStyle("default"),
		opts.BoxSeparateColumns(true),
		opts.BoxDrawBorder(true),
		opts.RowsFlatten(req.RowsFlatten),
		opts.RowsArray(req.RowsArray),
		opts.Transpose(req.Transpose),
	)

	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	defer conn.Close()

	query := &api.Query{
		Begin: func(q *api.Query) {
			if !q.IsFetch() {
				return
			}
			cols := q.Columns()
			ctx.Writer.Header().Set("Content-Type", encoder.ContentType())
			if len(req.Compress) > 0 {
				ctx.Writer.Header().Set("Content-Encoding", req.Compress)
			}
			codec.SetEncoderColumns(encoder, cols)
			encoder.Open()
		},
		Next: func(q *api.Query, nrow int64) bool {
			values, err := q.Columns().MakeBuffer()
			if err != nil {
				svr.log.Error("buffer", err.Error())
				return false
			}
			if err := q.Scan(values...); err != nil {
				svr.log.Error("scan", err.Error())
				return false
			}
			if err := encoder.AddRow(values); err != nil {
				// report error to client?
				svr.log.Error("render", err.Error())
				return false
			}
			return true
		},
		End: func(q *api.Query) {
			if q.IsFetch() {
				encoder.Close()
			} else {
				rsp.Success, rsp.Reason = true, q.UserMessage()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusOK, rsp)
			}
		},
	}

	if err := query.Execute(ctx, conn, req.SqlText); err != nil {
		svr.log.Error("query fail", err.Error())
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
	}
}

func (svr *httpd) handleWatchQuery(ctx *gin.Context) {
	tick := time.Now()
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Watcher panic", r)
		}
	}()

	var period time.Duration
	if p, err := time.ParseDuration(ctx.Query("period")); err == nil {
		period = p
	}
	if period < 1*time.Second {
		period = 1 * time.Second
	}
	var keepAlive time.Duration
	if p, err := time.ParseDuration(ctx.Query("keep-alive")); err == nil {
		keepAlive = p
	}
	if keepAlive == 0 {
		keepAlive = 30 * time.Second
	}

	var maxRowNum = strInt(ctx.Query("max-rows"), 100)
	var parallelism = strInt(ctx.Query("parallelism"), 3)

	timeformat := strString(ctx.Query("timeformat"), "ns")
	tz := time.UTC
	if timezone := ctx.Query("tz"); timezone != "" {
		tz, _ = util.ParseTimeLocation(timezone, time.UTC)
	}

	watch, err := api.NewWatcher(ctx,
		api.WatcherConfig{
			ConnProvider: func() (api.Conn, error) { return svr.getTrustConnection(ctx) },
			TableName:    ctx.Param("table"),
			TagNames:     ctx.QueryArray("tag"),
			Timeformat:   timeformat,
			Timezone:     tz,
			Parallelism:  parallelism,
			ChanSize:     100,
			MaxRowNum:    maxRowNum,
		})
	if err != nil {
		svr.log.Debug("Watcher error", err.Error())
		rsp := QueryResponse{Reason: err.Error(), Elapse: time.Since(tick).String()}
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	defer watch.Close()

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")

	periodTick := time.NewTicker(period)
	defer periodTick.Stop()
	keepAliveTick := time.NewTicker(keepAlive)
	defer keepAliveTick.Stop()

	lastWriteTime := time.Now()
	svr.log.Infof("%s start period %v, keep-alive %v", watch.String(), period, keepAlive)
	watch.Execute()
	for {
		select {
		case <-keepAliveTick.C:
			if time.Since(lastWriteTime) < keepAlive {
				continue
			}
			ctx.Writer.Write([]byte(": keep-alive\n\n"))
			ctx.Writer.Flush()
			lastWriteTime = time.Now()
		case <-periodTick.C:
			watch.Execute()
		case data := <-watch.C:
			switch v := data.(type) {
			case api.WatchData:
				b, _ := json.Marshal(v)
				ctx.Writer.Write([]byte("data: "))
				ctx.Writer.Write(b)
				ctx.Writer.Write([]byte("\n\n"))
				ctx.Writer.Flush()
				lastWriteTime = time.Now()
			case error:
				ctx.Writer.Write([]byte(fmt.Sprintf("error: %s\n\n", v.Error())))
				ctx.Writer.Flush()
				lastWriteTime = time.Now()
			}
		case <-ctx.Writer.CloseNotify():
			svr.log.Infof("%s end", watch.String())
			return
		}
	}
}

type SplitSQLResponse struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
	Data    any    `json:"data,omitempty"`
}

func (svr *httpd) handleSplitSQL(ctx *gin.Context) {
	rsp := &SplitSQLResponse{Success: false, Reason: "not specified"}
	tick := time.Now()
	stmts, err := util.SplitSqlStatements(ctx.Request.Body)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	rsp.Success = true
	rsp.Reason = "success"
	rsp.Data = map[string]any{
		"statements": stmts,
	}
	rsp.Elapse = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

type SplitHTTPResponse struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
	Data    any    `json:"data,omitempty"`
}

func (svr *httpd) handleSplitHTTP(ctx *gin.Context) {
	rsp := &SplitHTTPResponse{Success: false, Reason: "not specified"}
	tick := time.Now()
	stmts, err := util.SplitHttpStatements(ctx.Request.Body)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	rsp.Success = true
	rsp.Reason = "success"
	rsp.Data = map[string]any{
		"statements": stmts,
	}
	rsp.Elapse = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleFileQuery(ctx *gin.Context) {
	rsp := &QueryResponse{Success: false, Reason: "not specified"}
	tick := time.Now()

	tableName := ctx.Param("table")
	columnName := ctx.Param("column")
	fileID := ctx.Param("id")
	if len(tableName) == 0 || len(columnName) == 0 || len(fileID) == 0 ||
		strings.ContainsAny(tableName, "; \t\r\n()") ||
		strings.ContainsAny(columnName, "; \t\r\n()") {
		rsp.Reason = "invalid request"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	uid := uuid.UUID{}
	if err := uid.Parse(fileID); err != nil {
		rsp.Reason = fmt.Sprintf("invalid id, %s", err.Error())
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	uidTs, err := uuid.TimestampFromV6(uid)
	if err != nil {
		rsp.Reason = fmt.Sprintf("bad timestamp id, %s", err.Error())
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	ts, _ := uidTs.Time()

	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusUnauthorized, rsp)
		return
	}
	defer conn.Close()

	var sqlText string
	var sqlParams []any
	if tableType, err := api.QueryTableType(ctx, conn, tableName); err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	} else if tableType == api.TableTypeLog {
		sqlText = fmt.Sprintf("SELECT %s FROM %s WHERE _ARRIVAL_TIME BETWEEN ? and ? and %s->'$.ID' = ?",
			columnName, tableName, columnName)
		sqlParams = append(sqlParams, ts.Add(-2*time.Second).UnixNano())
		sqlParams = append(sqlParams, ts.Add(3*time.Second).UnixNano())
		sqlParams = append(sqlParams, fileID)
	} else if tableType == api.TableTypeTag {
		var desc *api.TableDescription
		if desc0, err := api.DescribeTable(ctx, conn, tableName, false); err != nil {
			rsp.Reason = fmt.Sprintf("fail to get table info '%s', %s", tableName, err.Error())
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		} else {
			desc = desc0
		}
		basetimeColumn := "TIME"
		nameColumn := "NAME"
		tagName := ctx.Query("tag")

		for _, c := range desc.Columns {
			if c.IsBaseTime() {
				basetimeColumn = c.Name
			} else if c.IsTagName() {
				nameColumn = c.Name
			}
		}
		if len(tagName) == 0 || strings.ContainsAny(tagName, "; \t\r\n()") {
			sqlText = fmt.Sprintf("SELECT %s FROM %s WHERE %s BETWEEN ? AND ? AND %s->'$.ID' = ?",
				columnName, tableName, basetimeColumn, columnName)
		} else {
			sqlText = fmt.Sprintf("SELECT %s FROM %s WHERE %s = ? AND %s BETWEEN ? AND ? AND %s->'$.ID' = ?",
				columnName, tableName, nameColumn, basetimeColumn, columnName)
			sqlParams = append(sqlParams, tagName)
		}
		sqlParams = append(sqlParams, ts.Add(-2*time.Second).UnixNano())
		sqlParams = append(sqlParams, ts.Add(3*time.Second).UnixNano())
		sqlParams = append(sqlParams, fileID)
	} else {
		rsp.Reason = fmt.Sprintf("Table '%s' is does not supported for files", tableName)
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusNotFound, rsp)
		return
	}

	row := conn.QueryRow(ctx, sqlText, sqlParams...)
	if row.Err() != nil {
		rsp.Reason = row.Err().Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	var fileDataStr string
	if err := row.Scan(&fileDataStr); err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	fileData := &UserFileData{}
	if err := json.Unmarshal([]byte(fileDataStr), fileData); err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	if fileData.ContentType != "" {
		ctx.Header("Content-Type", fileData.ContentType)
	} else {
		ctx.Header("Content-Type", "application/octet-stream")
	}
	if fileData.Filename != "" {
		ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileData.Filename))
	} else {
		ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileData.Id))
	}
	http.ServeFile(ctx.Writer, ctx.Request, filepath.Join(fileData.StoreDir, fileData.Id))
}

// Get a list of existing tables
//
// @Summary     Get table list
// @Description Get table list
// @Param       name         query string false "table name prefix or glob pattern"
// @Param       showall      query boolean false "show all hidden tables"
// @Success     200  {object}  msg.QueryResponse
// @Failure     500 {object}  msg.QueryResponse
// @Router      /web/api/tables [get]
func (svr *httpd) handleTables(ctx *gin.Context) {
	tick := time.Now()
	nameFilter := strings.ToUpper(ctx.Query("name"))
	nameFilterGlob := false
	showAll := strBool(ctx.Query("showall"), false)

	if nameFilter != "" {
		nameFilterGlob = glob.IsGlob(nameFilter)
	}

	rsp := &QueryResponse{Success: true, Reason: "success"}
	data := &QueryData{
		Columns: []string{"ROWNUM", "DB", "USER", "NAME", "TYPE"},
		Types: []api.DataType{
			api.DataTypeInt32,  // rownum
			api.DataTypeString, // db
			api.DataTypeString, // user
			api.DataTypeString, // name
			api.DataTypeString, // type
		},
	}

	conn, err := svr.getUserConnection(ctx)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	defer conn.Close()

	rownum := 0
	api.ListTablesWalk(ctx, conn, showAll, func(ti *api.TableInfo) bool {
		if ti.Err() != nil {
			rsp.Success, rsp.Reason = false, ti.Err().Error()
			return false
		}
		if nameFilter != "" {
			if nameFilterGlob {
				matched, err := glob.Match(nameFilter, ti.Name)
				if err != nil {
					rsp.Success, rsp.Reason = false, err.Error()
					return false
				}
				if !matched {
					return true
				}
			} else if !strings.HasPrefix(ti.Name, nameFilter) {
				return true
			}
		}
		rownum++
		data.Rows = append(data.Rows, []any{
			rownum,
			ti.Database,
			ti.User,
			ti.Name,
			ti.Kind(),
		})
		return true
	})

	rsp.Elapse = time.Since(tick).String()
	if rsp.Success {
		rsp.Data = data
		ctx.JSON(http.StatusOK, rsp)
	} else {
		ctx.JSON(http.StatusInternalServerError, rsp)
	}
}

// Get tag names of the given table
//
// @Summary     Get tag list of the table
// @Description Get tag list of the table
// @Param       table         path string true "table name"
// @Param       name          query string false "tag name filter"
// @Success     200  {object}  msg.QueryResponse
// @Failure     500 {object}  msg.QueryResponse
// @Router      /web/api/tables/:table/tags [get]
func (svr *httpd) handleTags(ctx *gin.Context) {
	tick := time.Now()
	table := strings.ToUpper(ctx.Param("table"))
	nameFilter := strings.ToUpper(ctx.Query("name"))

	rsp := &QueryResponse{Success: true, Reason: "success"}
	data := &QueryData{
		Columns: []string{"ROWNUM", "NAME"},
		Types: []api.DataType{
			api.DataTypeInt32,  // rownum
			api.DataTypeString, // name
		},
		Rows: [][]any{},
	}
	rownum := 0

	conn, err := svr.getUserConnection(ctx)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	defer conn.Close()

	var isCancelled bool
	go func() {
		<-ctx.Request.Context().Done()
		isCancelled = true
	}()

	api.ListTagsWalk(ctx, conn, table, func(tag *api.TagInfo) bool {
		if tag.Err != nil {
			rsp.Success, rsp.Reason = false, tag.Err.Error()
			return false
		}
		if nameFilter != "" && !strings.HasPrefix(tag.Name, nameFilter) {
			return true
		}
		rownum++
		data.Rows = append(data.Rows, []any{
			rownum,
			tag.Name,
		})
		return !isCancelled
	})

	rsp.Elapse = time.Since(tick).String()
	if rsp.Success {
		rsp.Data = data
		ctx.JSON(http.StatusOK, rsp)
	} else {
		ctx.JSON(http.StatusInternalServerError, rsp)
	}
}

// Get tag stat
//
// @Summary     Get tag stat
// @Description Get tag stat
// @Param       table         path string true "table name"
// @Param       tag           path string true "tag name"
// @Param       timeformat    query string false "timeformat (ns, us, ms, s, timeformat)"
// @Param       tz            query string false "timezone"
// @Success     200  {object}  msg.QueryResponse
// @Failure     500 {object}  msg.QueryResponse
// @Router      /web/api/tables/:table/tags/:tag/stat [get]
func (svr *httpd) handleTagStat(ctx *gin.Context) {
	tick := time.Now()
	rsp := &QueryResponse{Success: true, Reason: "success"}
	table := strings.ToUpper(ctx.Param("table"))
	tag := ctx.Param("tag")
	timeformat := strString(ctx.Query("timeformat"), "ns")
	timeLocation, err := util.ParseTimeLocation(ctx.Query("tz"), time.UTC)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	conn, err := svr.getUserConnection(ctx)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	defer conn.Close()

	nfo, err := api.TagStat(ctx, conn, table, tag)
	if err != nil {
		rsp.Success, rsp.Reason = false, err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	data := &QueryData{
		Columns: []string{
			"ROWNUM", "NAME", "ROW_COUNT", "MIN_TIME", "MAX_TIME",
			"MIN_VALUE", "MIN_VALUE_TIME", "MAX_VALUE", "MAX_VALUE_TIME", "RECENT_ROW_TIME"},
		Types: []api.DataType{
			api.DataTypeInt32,    // rownum
			api.DataTypeString,   // name
			api.DataTypeInt64,    // row_count
			api.DataTypeDatetime, // min_time
			api.DataTypeDatetime, // max_time
			api.DataTypeFloat64,  // min_value
			api.DataTypeDatetime, // min_value_time
			api.DataTypeFloat64,  // max_value
			api.DataTypeDatetime, // max_value_time
			api.DataTypeDatetime, // recent_row_time
		},
		Rows: [][]any{},
	}

	timeToJson := func(v time.Time) any {
		switch timeformat {
		case "ns":
			return v.UnixNano()
		case "ms":
			return v.UnixMilli()
		case "us":
			return v.UnixMicro()
		case "s":
			return v.Unix()
		default:
			return v.In(timeLocation).Format(timeformat)
		}
	}

	vs := []any{1, nfo.Name, nfo.RowCount}
	if nfo.MinTime.IsZero() {
		vs = append(vs, nil)
	} else {
		vs = append(vs, timeToJson(nfo.MinTime))
	}
	if nfo.MaxTime.IsZero() {
		vs = append(vs, nil)
	} else {
		vs = append(vs, timeToJson(nfo.MaxTime))
	}
	if nfo.MinValueTime.IsZero() {
		vs = append(vs, nil, nil)
	} else {
		vs = append(vs, nfo.MinValue, timeToJson(nfo.MinValueTime))
	}
	if nfo.MaxValueTime.IsZero() {
		vs = append(vs, nil, nil)
	} else {
		vs = append(vs, nfo.MaxValue, timeToJson(nfo.MaxValueTime))
	}
	if nfo.RecentRowTime.IsZero() {
		vs = append(vs, nil)
	} else {
		vs = append(vs, timeToJson(nfo.RecentRowTime))
	}
	data.Rows = append(data.Rows, vs)

	rsp.Elapse = time.Since(tick).String()
	rsp.Data = data
	ctx.JSON(http.StatusOK, rsp)
}

const TqlHeaderChartType = "X-Chart-Type"
const TqlHeaderChartOutput = "X-Chart-Output"
const TqlHeaderTqlOutput = "X-Tql-Output"
const TqlHeaderConsoleId = "X-Console-Id"

type ConsoleInfo struct {
	consoleId       string
	consoleLogLevel tql.Level
	logLevel        tql.Level
}

func parseConsoleId(ctx *gin.Context) *ConsoleInfo {
	ret := &ConsoleInfo{}
	ret.consoleId = ctx.GetHeader(TqlHeaderConsoleId)
	if fields := util.SplitFields(ret.consoleId, true); len(fields) > 1 {
		ret.consoleId = fields[0]
		for _, field := range fields[1:] {
			kvpair := strings.SplitN(field, "=", 2)
			if len(kvpair) == 2 {
				switch strings.ToLower(kvpair[0]) {
				case "console-log-level":
					ret.consoleLogLevel = tql.ParseLogLevel(kvpair[1])
				case "log-level":
					ret.logLevel = tql.ParseLogLevel(kvpair[1])
				}
			}
		}
	}
	return ret
}

const TQL_SCRIPT_PARAM = "$"
const TQL_TOKEN_PARAM = "$token"

// POST "/tql/tql-exec" accepts the access token in the query parameter
func (svr *httpd) handleTqlQueryExec(ctx *gin.Context) {
	if token := ctx.Query(TQL_TOKEN_PARAM); token != "" {
		ctx.Request.Header.Set("Authorization", "Bearer "+token)
	}
	svr.handleJwtToken(ctx)
	if ctx.IsAborted() {
		return
	}
	svr.handleTqlQuery(ctx)
}

// POST "/tql"
// POST "/tql?$=...."
// GET  "/tql?$=...."
func (svr *httpd) handleTqlQuery(ctx *gin.Context) {
	rsp := &QueryResponse{Success: false, Reason: "not specified"}
	tick := time.Now()

	claim, _ := svr.getJwtClaim(ctx)
	consoleInfo := parseConsoleId(ctx)

	params, err := url.ParseQuery(ctx.Request.URL.RawQuery)
	if err != nil {
		svr.log.Error("tql params error", err.Error())
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	var codeReader io.Reader
	var input io.Reader
	var debug = false
	switch ctx.Request.Method {
	case http.MethodPost:
		if script := ctx.Query(TQL_SCRIPT_PARAM); script == "" {
			if debug {
				b, _ := io.ReadAll(ctx.Request.Body)
				fmt.Println("...", string(b), "...")
				codeReader = bytes.NewBuffer(b)
			} else {
				codeReader = ctx.Request.Body
			}
		} else {
			codeReader = bytes.NewBufferString(script)
			if debug {
				fmt.Println("...", script, "...")
			}
			params.Del(TQL_SCRIPT_PARAM)
			params.Del(TQL_TOKEN_PARAM)
			input = ctx.Request.Body
		}
	case http.MethodGet:
		if script := ctx.Query(TQL_SCRIPT_PARAM); script != "" {
			codeReader = bytes.NewBufferString(script)
			params.Del(TQL_SCRIPT_PARAM)
			params.Del(TQL_TOKEN_PARAM)
		} else {
			rsp.Reason = "script not found"
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusBadRequest, rsp)
			return
		}
	default:
		rsp.Reason = "unsupported method"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusMethodNotAllowed, rsp)
		return
	}

	task := tql.NewTaskContext(ctx)
	task.SetParams(params)
	task.SetInputReader(input)
	task.SetLogWriter(logging.GetLog("_nonamed.tql"))
	task.SetConsoleLogLevel(consoleInfo.consoleLogLevel)
	if claim != nil && consoleInfo.consoleId != "" {
		if svr.authServer == nil {
			task.SetConsole(claim.Subject, consoleInfo.consoleId, "")
		} else {
			otp, _ := svr.authServer.GenerateOtp(claim.Subject)
			task.SetConsole(claim.Subject, consoleInfo.consoleId, "$otp$:"+otp)
		}
	}
	task.SetOutputWriterJson(&util.NopCloseWriter{Writer: ctx.Writer}, true)
	task.SetDatabase(svr.db)
	if err := task.Compile(codeReader); err != nil {
		svr.log.Error("tql parse error", err.Error())
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	task.SetVolatileAssetsProvider(svr.memoryFs)
	ctx.Writer.Header().Set("Content-Type", task.OutputContentType())
	ctx.Writer.Header().Set("Content-Encoding", task.OutputContentEncoding())
	if chart := task.OutputChartType(); len(chart) > 0 {
		ctx.Writer.Header().Set(TqlHeaderChartType, chart)
	}
	if headers := task.OutputHttpHeaders(); len(headers) > 0 {
		for k, vs := range headers {
			for _, v := range vs {
				ctx.Writer.Header().Set(k, v)
			}
		}
	}
	go func() {
		<-ctx.Request.Context().Done()
		task.Cancel()
	}()

	result := task.Execute()
	if result == nil {
		svr.log.Error("tql execute return nil")
		rsp.Reason = "task result is empty"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
	} else if result.IsDbSink {
		ctx.JSON(http.StatusOK, result)
	} else if !ctx.Writer.Written() {
		// clear headers for the json result
		ctx.Writer.Header().Set("Content-Type", "application/json")
		ctx.Writer.Header().Del("Content-Encoding")
		ctx.Writer.Header().Del(TqlHeaderChartType)
		ctx.JSON(http.StatusOK, result)
	}
}

func handleError(ctx *gin.Context, statusCode int, msg string, tick time.Time) {
	rsp := &QueryResponse{
		Success: false,
		Reason:  msg,
		Elapse:  time.Since(tick).String(),
	}
	ctx.JSON(statusCode, rsp)
}

// tql as RESTful API
//
// GET  "/tql/*path"
// POST "/tql/*path"
func (svr *httpd) handleTqlFile(ctx *gin.Context) {
	tick := time.Now()

	path := ctx.Param("path")
	if !strings.HasSuffix(path, ".tql") {
		contentType := contentTypeOfFile(path)
		if contentType != "" && ctx.Request.Method == http.MethodGet {
			if ent, err := svr.serverFs.Get(path); err == nil && !ent.IsDir {
				ctx.Header("Content-Type", contentType)
				ctx.Writer.Write(ent.Content)
				return
			}
		}
		handleError(ctx, http.StatusNotFound, "tql not found", tick)
		return
	}
	params, err := url.ParseQuery(ctx.Request.URL.RawQuery)
	if err != nil {
		svr.log.Error("tql params error", path, err.Error())
		handleError(ctx, http.StatusBadRequest, err.Error(), tick)
		return
	}

	script, err := svr.tqlLoader.Load(path)
	if err != nil {
		svr.log.Error("tql load fail", path, err.Error())
		handleError(ctx, http.StatusNotFound, err.Error(), tick)
		return
	}

	task := tql.NewTaskContext(ctx)
	task.SetDatabase(svr.db)
	task.SetInputReader(ctx.Request.Body)
	task.SetParams(params)
	task.SetLogWriter(logging.GetLog(filepath.Base(path)))

	// Set output writer based on headers
	if ctx.Request.Header.Get(TqlHeaderChartOutput) == "json" || ctx.Request.Header.Get(TqlHeaderTqlOutput) == "json" {
		task.SetOutputWriterJson(&util.NopCloseWriter{Writer: ctx.Writer}, true)
	} else {
		task.SetOutputWriter(&util.NopCloseWriter{Writer: ctx.Writer})
	}

	// Compile the script
	if err := task.CompileScript(script); err != nil {
		svr.log.Error("tql parse fail", path, err.Error())
		handleError(ctx, http.StatusInternalServerError, err.Error(), tick)
		return
	}

	contentType := task.OutputContentType()
	if contentType == "application/xhtml+xml" {
		contentType = "text/html"
	}
	ctx.Writer.Header().Set("Content-Type", contentType)
	ctx.Writer.Header().Set("Content-Encoding", task.OutputContentEncoding())
	if chart := task.OutputChartType(); len(chart) > 0 {
		ctx.Writer.Header().Set(TqlHeaderChartType, chart)
	}
	if headers := task.OutputHttpHeaders(); len(headers) > 0 {
		for k, vs := range headers {
			for _, v := range vs {
				ctx.Writer.Header().Set(k, v)
			}
		}
	}

	// Handle task cancellation
	go func() {
		<-ctx.Request.Context().Done()
		task.Cancel()
	}()

	// Exeute the task
	result := task.Execute()
	if result == nil {
		svr.log.Error("tql execute return nil")
		handleError(ctx, http.StatusInternalServerError, "task result is empty", tick)
		return
	}

	if result.IsDbSink {
		ctx.JSON(http.StatusOK, result)
		return
	}

	if !ctx.Writer.Written() {
		ctx.JSON(http.StatusOK, result)
	}
}
