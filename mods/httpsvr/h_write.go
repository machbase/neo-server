package httpsvr

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/msg"
	"github.com/machbase/neo-shell/codec"
	"github.com/machbase/neo-shell/do"
	spi "github.com/machbase/neo-spi"
)

func (svr *Server) handleWrite(ctx *gin.Context) {
	if ctx.ContentType() == "text/csv" {
		svr.handleWriteCSV(ctx)
	} else {
		svr.handleWriteJSON(ctx)
	}
}

func (svr *Server) handleWriteCSV(ctx *gin.Context) {
	rsp := &msg.WriteResponse{Reason: "not specified"}
	tick := time.Now()
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	tableName := ctx.Param("table")
	timeformat := strString(ctx.Query("timeformat"), "ns")
	timeLocation := strTimeLocation(ctx.Query("tz"), time.UTC)
	format := strString(ctx.Query("format"), "csv")
	method := strString(ctx.Query("method"), "insert")
	compress := strString(ctx.Query("compress"), "-")
	delimiter := strString(ctx.Query("delimiter"), ",")
	createTable := strBool(ctx.Query("create-table"), false)
	truncateTable := strBool(ctx.Query("truncate-table"), false)

	exists, _, _, err := do.ExistsTableOrCreate(svr.db, tableName, createTable, truncateTable)
	if err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !exists {
		rsp.Reason = fmt.Sprintf("Table '%s' does not exist", tableName)
		ctx.JSON(http.StatusNotFound, rsp)
		return
	}

	var desc *do.TableDescription
	if desc0, err := do.Describe(svr.db, tableName, false); err != nil {
		rsp.Reason = fmt.Sprintf("fail to get table info '%s', %s", tableName, err.Error())
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	} else {
		desc = desc0.(*do.TableDescription)
	}

	var r io.Reader
	if compress == "gzip" {
		gr, err := gzip.NewReader(ctx.Request.Body)
		if err != nil {
			rsp.Reason = err.Error()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		r = bufio.NewReader(gr)
	} else {
		r = ctx.Request.Body
	}

	decoder := codec.NewDecoderBuilder(format).
		SetInputStream(r).
		SetColumns(desc.Columns.Columns()).
		SetTimeFormat(timeformat).
		SetTimeLocation(timeLocation).
		SetCsvDelimieter(delimiter).
		Build()

	var appender spi.Appender
	hold := []string{}
	lineno := 0

	for {
		vals, err := decoder.NextRow()
		if err != nil {
			if err != io.EOF {
				rsp.Reason = err.Error()
				ctx.JSON(http.StatusBadRequest, rsp)
				return
			}
			break
		}
		lineno++

		if method == "insert" {
			for i := 0; i < len(desc.Columns); i++ {
				hold = append(hold, "?")
			}
			query := fmt.Sprintf("insert into %s values(%s)", tableName, strings.Join(hold, ","))
			if result := svr.db.Exec(query, vals...); result.Err() != nil {
				rsp.Reason = result.Err().Error()
				ctx.JSON(http.StatusInternalServerError, rsp)
				return
			}
			hold = hold[:0]
		} else { // append
			if appender == nil {
				appender, err := svr.db.Appender(tableName)
				if err != nil {
					rsp.Reason = err.Error()
					ctx.JSON(http.StatusInternalServerError, rsp)
					return
				}
				defer appender.Close()
			}
			err = appender.Append(vals...)
			if err != nil {
				rsp.Reason = err.Error()
				ctx.JSON(http.StatusInternalServerError, rsp)
				return
			}
		}
	}
	rsp.Success, rsp.Reason = true, "success"
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *Server) handleWriteJSON(ctx *gin.Context) {
	req := &msg.WriteRequest{}
	rsp := &msg.WriteResponse{Reason: "not specified"}
	tick := time.Now()
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	err := ctx.Bind(req)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	// post body로 전달되는 table name이 우선한다.
	if len(req.Table) == 0 {
		// table명이 path param으로 입력될 수도 있고
		req.Table = ctx.Param("table")
	}

	if len(req.Table) == 0 {
		rsp.Reason = "table is not specified"
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	if req.Data == nil {
		rsp.Reason = "no data found"
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	result := do.Insert(svr.db, req.Table, req.Data.Columns, req.Data.Rows)
	if result.Err() == nil {
		rsp.Success = true
		rsp.Reason = result.Message()
		ctx.JSON(http.StatusOK, rsp)
	} else {
		rsp.Success = false
		rsp.Reason = result.Message()
		ctx.JSON(http.StatusInternalServerError, rsp)
	}
}
