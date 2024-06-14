package httpd

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/util"
)

func (svr *httpd) handleWrite(ctx *gin.Context) {
	rsp := &msg.WriteResponse{Reason: "not specified"}
	tick := time.Now()

	format := "json"
	if ctx.ContentType() == "text/csv" {
		format = "csv"
	}
	compress := "-"
	switch ctx.Request.Header.Get("Content-Encoding") {
	case "gzip":
		compress = "gzip"
	default:
		compress = "-"
	}

	tableName := ctx.Param("table")
	timeformat := strString(ctx.Query("timeformat"), "ns")
	timeLocation := util.ParseTimeLocation(ctx.Query("tz"), time.UTC)
	method := strString(ctx.Query("method"), "insert")
	format = strString(ctx.Query("format"), format)
	compress = strString(ctx.Query("compress"), compress)
	delimiter := strString(ctx.Query("delimiter"), ",")
	heading := strBool(ctx.Query("heading"), false)

	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusUnauthorized, rsp)
		return
	}
	defer conn.Close()

	exists, err := do.ExistsTable(ctx, conn, tableName)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !exists {
		rsp.Reason = fmt.Sprintf("Table '%s' does not exist", tableName)
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusNotFound, rsp)
		return
	}

	var desc *do.TableDescription
	if desc0, err := do.Describe(ctx, conn, tableName, false); err != nil {
		rsp.Reason = fmt.Sprintf("fail to get table info '%s', %s", tableName, err.Error())
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	} else {
		desc = desc0.(*do.TableDescription)
	}

	var in spec.InputStream
	if compress == "gzip" {
		gr, err := gzip.NewReader(ctx.Request.Body)
		if err != nil {
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		in = &stream.ReaderInputStream{Reader: bufio.NewReader(gr)}
	} else {
		in = &stream.ReaderInputStream{Reader: ctx.Request.Body}
	}

	codecOpts := []opts.Option{
		opts.TableName(tableName),
		opts.Timeformat(timeformat),
		opts.TimeLocation(timeLocation),
		opts.Delimiter(delimiter),
		opts.Heading(heading),
	}

	var appender api.Appender
	var recno int
	var insertQuery string

	if method == "append" {
		appender, err = conn.Appender(ctx, tableName)
		if err != nil {
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		defer appender.Close()
		cols := desc.Columns.Columns()
		colNames := cols.Names()
		colTypes := cols.Types()
		if api.AppenderTableType(appender) == api.LogTableType && colNames[0] == "_ARRIVAL_TIME" {
			colNames = colNames[1:]
			colTypes = colTypes[1:]
		}

		codecOpts = append(codecOpts,
			opts.InputStream(in),
			opts.Columns(colNames...),
			opts.ColumnTypes(colTypes...),
		)
	} else { // insert
		var columnNames []string
		var columnTypes []string
		if format == "json" {
			bs, err := io.ReadAll(in)
			if err != nil {
				rsp.Reason = err.Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusBadRequest, rsp)
				return
			}

			wr := msg.WriteRequest{}
			dec := json.NewDecoder(bytes.NewBuffer(bs))
			if err := dec.Decode(&wr); err != nil {
				rsp.Reason = err.Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusBadRequest, rsp)
				return
			}
			if wr.Data != nil && len(wr.Data.Columns) > 0 {
				columnNames = wr.Data.Columns
				columnTypes = make([]string, 0, len(columnNames))
				_hold := make([]string, 0, len(columnNames))
				for _, colName := range columnNames {
					_hold = append(_hold, "?")
					_type := ""
					for _, d := range desc.Columns {
						if d.Name == strings.ToUpper(colName) {
							_type = d.TypeString()
							break
						}
					}
					if _type == "" {
						rsp.Reason = fmt.Sprintf("column %q not found in the table %q", colName, tableName)
						rsp.Elapse = time.Since(tick).String()
						ctx.JSON(http.StatusBadRequest, rsp)
						return
					}
					columnTypes = append(columnTypes, _type)
				}
				valueHolder := strings.Join(_hold, ",")
				insertQuery = fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", tableName, strings.Join(columnNames, ","), valueHolder)
			}
			in = &stream.ReaderInputStream{Reader: bytes.NewBuffer(bs)}
		}
		if len(columnNames) == 0 {
			columnNames = desc.Columns.Columns().Names()
			columnTypes = make([]string, 0, len(desc.Columns))
			_hold := make([]string, 0, len(desc.Columns))
			for _, c := range desc.Columns {
				_hold = append(_hold, "?")
				columnTypes = append(columnTypes, c.TypeString())
			}
			valueHolder := strings.Join(_hold, ",")
			insertQuery = fmt.Sprintf("INsERT INTO %s VALUES(%s)", tableName, valueHolder)
		}
		codecOpts = append(codecOpts,
			opts.InputStream(in),
			opts.Columns(columnNames...),
			opts.ColumnTypes(columnTypes...),
		)
	}

	decoder := codec.NewDecoder(format, codecOpts...)

	if decoder == nil {
		rsp.Reason = "codec not found"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	for {
		vals, err := decoder.NextRow()
		if err != nil {
			if err != io.EOF {
				rsp.Reason = err.Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusBadRequest, rsp)
				return
			}
			break
		}
		recno++

		if method == "insert" {
			if result := conn.Exec(ctx, insertQuery, vals...); result.Err() != nil {
				rsp.Reason = result.Err().Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusInternalServerError, rsp)
				return
			}
		} else { // append
			err = appender.Append(vals...)
			if err != nil {
				rsp.Reason = err.Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusInternalServerError, rsp)
				return
			}
		}
	}
	rsp.Success, rsp.Reason = true, fmt.Sprintf("success, %d record(s) %sed", recno, method)
	rsp.Elapse = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}
