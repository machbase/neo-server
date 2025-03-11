package server

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid/v5"
	"github.com/influxdata/line-protocol/v2/lineprotocol"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/codec"
	"github.com/machbase/neo-server/v8/mods/codec/opts"
	"github.com/machbase/neo-server/v8/mods/util"
)

func (svr *httpd) handleWrite(ctx *gin.Context) {
	rsp := &WriteResponse{Reason: "not specified"}
	tick := time.Now()

	if ctx.ContentType() == "multipart/form-data" {
		svr.handleFileWrite(ctx)
		return
	}
	format := "json"
	if ctx.ContentType() == "text/csv" {
		format = "csv"
	} else if ctx.ContentType() == "application/x-ndjson" {
		format = "ndjson"
	}
	compress := "-"
	switch ctx.Request.Header.Get("Content-Encoding") {
	case "gzip":
		compress = "gzip"
	default:
		compress = "-"
	}

	errRsp := func(status int, reason string) {
		rsp.Reason = reason
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(status, rsp)
	}

	tableName := ctx.Param("table")
	timeformat := strString(ctx.Query("timeformat"), "ns")
	timeLocation, err := util.ParseTimeLocation(ctx.Query("tz"), time.UTC)
	if err != nil {
		errRsp(http.StatusBadRequest, err.Error())
		return
	}
	method := strString(ctx.Query("method"), "insert")
	format = strString(ctx.Query("format"), format)
	compress = strString(ctx.Query("compress"), compress)
	delimiter := strString(ctx.Query("delimiter"), ",")

	// check `heading` for backward compatibility
	headerSkip := strBool(ctx.Query("heading"), false)
	headerColumns := false
	switch strings.ToLower(ctx.Query("header")) {
	case "skip":
		headerSkip = true
	case "column", "columns":
		headerColumns = true
		headerSkip = true
	default:
	}

	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		errRsp(http.StatusUnauthorized, err.Error())
		return
	}
	defer conn.Close()

	var in io.Reader
	if compress == "gzip" {
		gr, err := gzip.NewReader(ctx.Request.Body)
		if err != nil {
			errRsp(http.StatusInternalServerError, err.Error())
			return
		}
		in = bufio.NewReader(gr)
	} else {
		in = ctx.Request.Body
	}

	codecOpts := []opts.Option{
		opts.TableName(tableName),
		opts.Timeformat(timeformat),
		opts.TimeLocation(timeLocation),
		opts.Delimiter(delimiter),
		opts.Header(headerSkip),
		opts.HeaderColumns(headerColumns),
	}

	var appender api.Appender
	var recNo int
	var insertQuery string
	var desc *api.TableDescription

	if method == "append" {
		overrideUseAppenderWorker := ctx.GetHeader(TqlHeaderAppendWorker)
		// set HTTP Header 'X-Append-Worker: no' to disable appender worker
		if svr.useAppendWroker && overrideUseAppenderWorker == "" {
			svr.appendersLock.Lock()
			defer svr.appendersLock.Unlock()
			if aw, exists := svr.appenders[tableName]; exists {
				aw.lastTime = time.Now()
				appender = aw.appender
				desc = aw.tableDesc
			}
			if appender == nil {
				if tableDesc, err := api.DescribeTable(ctx, conn, tableName, false); err != nil {
					errRsp(http.StatusInternalServerError, fmt.Sprintf("fail to get table info '%s', %s", tableName, err.Error()))
					return
				} else {
					desc = tableDesc
				}

				appendConn, err := svr.getTrustConnection(ctx)
				if err != nil {
					errRsp(http.StatusInternalServerError, err.Error())
					return
				}
				appender, err = appendConn.Appender(ctx, tableName)
				if err != nil {
					errRsp(http.StatusInternalServerError, err.Error())
					return
				}
				aw := &AppenderWrapper{
					conn:      appendConn,
					appender:  appender,
					tableDesc: desc,
					lastTime:  time.Now(),
				}
				aw.ctx, aw.ctxCancel = context.WithCancel(context.Background())
				svr.appenders[tableName] = aw
			}
		} else {
			if tableDesc, err := api.DescribeTable(ctx, conn, tableName, false); err != nil {
				errRsp(http.StatusInternalServerError, fmt.Sprintf("fail to get table info '%s', %s", tableName, err.Error()))
				return
			} else {
				desc = tableDesc
			}

			appender, err = conn.Appender(ctx, tableName)
			if err != nil {
				errRsp(http.StatusInternalServerError, err.Error())
				return
			}
			defer appender.Close()
		}

		colNames := desc.Columns.Names()
		colTypes := desc.Columns.DataTypes()
		if appender.TableType() == api.TableTypeLog && colNames[0] == "_ARRIVAL_TIME" {
			colNames = colNames[1:]
			colTypes = colTypes[1:]
		}

		codecOpts = append(codecOpts,
			opts.InputStream(in),
			opts.Columns(colNames...),
			opts.ColumnTypes(colTypes...),
		)
	} else { // insert
		if tableDesc, err := api.DescribeTable(ctx, conn, tableName, false); err != nil {
			errRsp(http.StatusInternalServerError, fmt.Sprintf("fail to get table info '%s', %s", tableName, err.Error()))
			return
		} else {
			desc = tableDesc
		}

		var columnNames []string
		var columnTypes []api.DataType
		if format == "json" {
			bs, err := io.ReadAll(in)
			if err != nil {
				errRsp(http.StatusBadRequest, err.Error())
				return
			}

			wr := WriteRequest{}
			dec := json.NewDecoder(bytes.NewBuffer(bs))
			if err := dec.Decode(&wr); err != nil {
				errRsp(http.StatusBadRequest, err.Error())
				return
			}
			if wr.Data != nil && len(wr.Data.Columns) > 0 {
				columnNames = wr.Data.Columns
				columnTypes = make([]api.DataType, 0, len(columnNames))
				_hold := make([]string, 0, len(columnNames))
				for _, colName := range columnNames {
					_hold = append(_hold, "?")
					_type := api.ColumnTypeUnknown
					for _, d := range desc.Columns {
						if d.Name == strings.ToUpper(colName) {
							_type = d.Type
							break
						}
					}
					if _type == api.ColumnTypeUnknown {
						errRsp(http.StatusBadRequest, fmt.Sprintf("column %q not found in the table %q", colName, tableName))
						return
					}
					columnTypes = append(columnTypes, _type.DataType())
				}
				valueHolder := strings.Join(_hold, ",")
				insertQuery = fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", tableName, strings.Join(columnNames, ","), valueHolder)
			}
			in = bytes.NewBuffer(bs)
		}
		if len(columnNames) == 0 {
			columnNames = desc.Columns.Names()
			columnTypes = make([]api.DataType, 0, len(desc.Columns))
			_hold := make([]string, 0, len(desc.Columns))
			for _, c := range desc.Columns {
				_hold = append(_hold, "?")
				columnTypes = append(columnTypes, c.Type.DataType())
			}
			valueHolder := strings.Join(_hold, ",")
			insertQuery = fmt.Sprintf("INSERT INTO %s VALUES(%s)", tableName, valueHolder)
		}
		codecOpts = append(codecOpts,
			opts.InputStream(in),
			opts.Columns(columnNames...),
			opts.ColumnTypes(columnTypes...),
		)
	}

	decoder := codec.NewDecoder(format, codecOpts...)

	if decoder == nil {
		errRsp(http.StatusInternalServerError, "codec not found")
		return
	}

	var prevCols []string
	var hasProcessedHeader bool
	for {
		vals, cols, err := decoder.NextRow()
		if err != nil {
			if err != io.EOF {
				rsp.Reason = err.Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusBadRequest, rsp)
				return
			}
			break
		}
		recNo++

		if method == "insert" {
			if len(cols) > 0 && !slices.Equal(prevCols, cols) {
				prevCols = cols
				_hold := make([]string, len(cols))
				for i := range desc.Columns {
					_hold[i] = "?" // for prepared statement
				}
				insertQuery = fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", tableName, strings.Join(cols, ","), strings.Join(_hold, ","))
			}
			if result := conn.Exec(ctx, insertQuery, vals...); result.Err() != nil {
				errRsp(http.StatusInternalServerError, result.Err().Error())
				return
			}
		} else { // append
			if !hasProcessedHeader && headerColumns && len(cols) > 0 {
				appender = appender.WithInputColumns(cols...)
				hasProcessedHeader = true
			}
			err = appender.Append(vals...)
			if err != nil {
				errRsp(http.StatusInternalServerError, err.Error())
				return
			}
		}
	}
	rsp.Success, rsp.Reason = true, fmt.Sprintf("success, %d record(s) %sed", recNo, method)
	rsp.Elapse = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleFileWrite(ctx *gin.Context) {
	rsp := &WriteResponse{Reason: "not specified"}
	tick := time.Now()

	if ctx.ContentType() != "multipart/form-data" {
		rsp.Reason = "content-type must be 'multipart/form-data'"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	tableName := ctx.Param("table")
	timeformat := strString(ctx.Query("timeformat"), "ns")
	timeLocation, err := util.ParseTimeLocation(ctx.Query("tz"), time.UTC)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusUnauthorized, rsp)
		return
	}
	defer conn.Close()

	tableType, err := api.QueryTableType(ctx, conn, tableName)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if tableType != api.TableTypeLog && tableType != api.TableTypeTag {
		rsp.Reason = fmt.Sprintf("Table '%s' is does not supported for files", tableName)
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusNotFound, rsp)
		return
	}

	var desc *api.TableDescription
	if desc0, err := api.DescribeTable(ctx, conn, tableName, false); err != nil {
		rsp.Reason = fmt.Sprintf("fail to get table info '%s', %s", tableName, err.Error())
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	} else {
		desc = desc0
	}

	findColumn := func(name string) *api.Column {
		for _, c := range desc.Columns {
			if strings.EqualFold(c.Name, name) {
				return c
			}
		}
		return nil
	}

	form, err := ctx.MultipartForm()
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	ts := time.Now()
	columns := []string{}
	values := []any{}
	for k, v := range form.Value {
		if c := findColumn(k); c != nil {
			columns = append(columns, c.Name)
			val, err := c.Type.DataType().Apply(v[0], timeformat, timeLocation)
			if err != nil {
				rsp.Reason = err.Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusBadRequest, rsp)
				return
			}
			if tableType == api.TableTypeTag && c.IsBaseTime() {
				ts = val.(time.Time)
			}
			values = append(values, val)
		} else {
			rsp.Reason = fmt.Sprintf("column %q not found in the table %q", k, tableName)
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusBadRequest, rsp)
			return
		}
	}

	// request header X-Store-Dir is used as default store directory
	defaultStoreDir := ctx.Request.Header.Get("X-Store-Dir")

	// store file fields
	for k, v := range form.File {
		if len(v) == 0 {
			continue
		}
		mff := &UserFileData{
			Filename: v[0].Filename,
			Size:     v[0].Size,
		}

		// store file to the specified upload directory
		storeDir := defaultStoreDir

		if d := v[0].Header.Get("X-Store-Dir"); d != "" {
			storeDir = d
		}
		if storeDir == "" {
			rsp.Reason = fmt.Sprintf("file %q requires X-Store-Dir header", k)
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusBadRequest, rsp)
			return
		}
		// replace pathMap
		for k, v := range svr.pathMap {
			storeDir = strings.ReplaceAll(storeDir, fmt.Sprintf("${%s}", k), v)
		}
		storeDir = filepath.FromSlash(storeDir)

		mff.StoreDir = storeDir
		idv6, _ := idGen.NewV6AtTime(ts)
		mff.Id = idv6.String()

		if c := v[0].Header.Get("Content-Type"); c != "" {
			mff.ContentType = c
		}
		mffJson, _ := json.Marshal(mff)
		columns = append(columns, k)
		values = append(values, string(mffJson))

		src, err := v[0].Open()
		if err != nil {
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		defer src.Close()
		if err := os.MkdirAll(mff.StoreDir, 0755); err != nil {
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		dst, err := os.OpenFile(filepath.Join(mff.StoreDir, mff.Id), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		defer dst.Close()
		if _, err := io.Copy(dst, src); err != nil {
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		if rsp.Data == nil {
			rsp.Data = &WriteResponseData{}
			rsp.Data.Files = make(map[string]*UserFileData)
		}
		rsp.Data.Files[strings.ToUpper(k)] = mff
	}
	holders := make([]string, len(columns))
	for i := range holders {
		holders[i] = "?"
	}

	insertQuery := fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", tableName, strings.Join(columns, ","), strings.Join(holders, ","))

	if result := conn.Exec(ctx, insertQuery, values...); result.Err() != nil {
		rsp.Reason = result.Err().Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp.Success, rsp.Reason = true, fmt.Sprintf("success, %d record(s) inserted", 1)
	rsp.Elapse = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

var idGen = uuid.NewGen()

// Configure telegraf.conf
//
//	[[outputs.http]]
//	url = "http://127.0.0.1:4088/metrics/write"
//	data_format = "influx"
//	content_encoding = "gzip"
func (svr *httpd) handleLineProtocol(ctx *gin.Context) {
	oper := ctx.Param("oper")
	method := ctx.Request.Method

	if method == http.MethodPost && oper == "write" {
		svr.handleLineWrite(ctx)
	} else {
		ctx.JSON(
			http.StatusNotImplemented,
			gin.H{"error": fmt.Sprintf("%s %s is not implemented", method, oper)})
	}
}

func (svr *httpd) handleLineWrite(ctx *gin.Context) {
	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	defer conn.Close()

	dbName := ctx.Query("db")
	var desc *api.TableDescription
	if desc0, err := api.DescribeTable(ctx, conn, dbName, false); err != nil {
		ctx.JSON(
			http.StatusBadRequest,
			gin.H{"error": fmt.Sprintf("column error: %s", err.Error())})
		return
	} else {
		desc = desc0
	}

	precision := lineprotocol.Nanosecond
	switch ctx.Query("precision") {
	case "us":
		precision = lineprotocol.Microsecond
	case "ms":
		precision = lineprotocol.Millisecond
	}
	var body io.Reader
	switch ctx.Request.Header.Get("Content-Encoding") {
	default:
		body = ctx.Request.Body
	case "gzip":
		gz, err := gzip.NewReader(ctx.Request.Body)
		if err != nil {
			ctx.JSON(
				http.StatusBadRequest,
				gin.H{"error": fmt.Sprintf("invalid gzip compression: %s", err.Error())})
			return
		}
		defer gz.Close()
		body = gz
	}

	dec := lineprotocol.NewDecoder(body)
	for dec != nil && dec.Next() {
		m, err := dec.Measurement()
		if err != nil {
			ctx.JSON(
				http.StatusInternalServerError,
				gin.H{"error": fmt.Sprintf("measurement error: %s", err.Error())})
			return
		}
		measurement := string(m)
		tags := make(map[string]string)
		fields := make(map[string]any)

		for {
			key, val, err := dec.NextTag()
			if err != nil {
				ctx.JSON(
					http.StatusInternalServerError,
					gin.H{"error": fmt.Sprintf("tag error: %s", err.Error())})
				return
			}
			if key == nil {
				break
			}
			tags[strings.ToUpper(string(key))] = string(val)
		}

		for {
			key, val, err := dec.NextField()
			if err != nil {
				ctx.JSON(
					http.StatusInternalServerError,
					gin.H{"error": fmt.Sprintf("field error: %s", err.Error())})
				return
			}
			if key == nil {
				break
			}
			fields[string(key)] = val.Interface()
		}

		ts, err := dec.Time(precision, time.Time{})
		if err != nil {
			ctx.JSON(
				http.StatusBadRequest,
				gin.H{"error": fmt.Sprintf("time error: %s", err.Error())})
			return
		}
		if ts.IsZero() {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "no timestamp"})
			return
		}

		result := api.WriteLineProtocol(ctx, conn, dbName, desc.Columns, measurement, fields, tags, ts)
		if err := result.Err(); err != nil {
			svr.log.Warnf("lineprotocol fail: %s", err.Error())
			ctx.JSON(
				http.StatusBadRequest,
				gin.H{"error": fmt.Sprintf("%s; %s", err.Error(), result.Message())})
			return
		}
	}
	ctx.JSON(http.StatusNoContent, "")
}
