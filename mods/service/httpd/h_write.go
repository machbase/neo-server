package httpd

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/transcoder"
	spi "github.com/machbase/neo-spi"
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
	timeLocation := strTimeLocation(ctx.Query("tz"), time.UTC)
	method := strString(ctx.Query("method"), "insert")
	format = strString(ctx.Query("format"), format)
	compress = strString(ctx.Query("compress"), compress)
	delimiter := strString(ctx.Query("delimiter"), ",")
	heading := strBool(ctx.Query("heading"), false)
	createTable := strBool(ctx.Query("create-table"), false)
	truncateTable := strBool(ctx.Query("truncate-table"), false)
	trans := strString(ctx.Query("transcoder"), "")

	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusUnauthorized, rsp)
		return
	}
	defer conn.Close()

	exists, _, _, err := do.ExistsTableOrCreate(ctx, conn, tableName, createTable, truncateTable)
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
		opts.InputStream(in),
		opts.TableName(tableName),
		opts.Columns(desc.Columns.Columns().Names()...),
		opts.ColumnTypes(desc.Columns.Columns().Types()...),
		opts.Timeformat(timeformat),
		opts.TimeLocation(timeLocation),
		opts.Delimiter(delimiter),
		opts.Heading(heading),
	}

	if len(trans) > 0 {
		transcoderOpts := []transcoder.Option{}
		if exepath, err := os.Executable(); err == nil {
			transcoderOpts = append(transcoderOpts, transcoder.OptionPath(filepath.Dir(exepath)))
		}
		transcoderOpts = append(transcoderOpts, transcoder.OptionPname("http"))
		trans := transcoder.New(trans, transcoderOpts...)

		codecOpts = append(codecOpts, opts.Transcoder(trans))
	}
	decoder := codec.NewDecoder(format, codecOpts...)

	if decoder == nil {
		rsp.Reason = "codec not found"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	var appender spi.Appender
	lineno := 0

	_hold := []string{}
	for i := 0; i < len(desc.Columns); i++ {
		_hold = append(_hold, "?")
	}
	valueHolder := strings.Join(_hold, ",")
	insertQuery := fmt.Sprintf("insert into %s values(%s)", tableName, valueHolder)

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
		lineno++

		if method == "insert" {
			if result := conn.Exec(ctx, insertQuery, vals...); result.Err() != nil {
				rsp.Reason = result.Err().Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusInternalServerError, rsp)
				return
			}
		} else { // append
			if appender == nil {
				appender, err = conn.Appender(ctx, tableName)
				if err != nil {
					rsp.Reason = err.Error()
					rsp.Elapse = time.Since(tick).String()
					ctx.JSON(http.StatusInternalServerError, rsp)
					return
				}
				defer appender.Close()
			}
			err = appender.Append(vals...)
			if err != nil {
				rsp.Reason = err.Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusInternalServerError, rsp)
				return
			}
		}
	}
	rsp.Success, rsp.Reason = true, fmt.Sprintf("success, %d record(s) %sed", lineno, method)
	rsp.Elapse = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}
