package httpsvr

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/mods/msg"
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
	timeformat := ctx.Query("timeformat")
	preAction := ctx.Query("pre-action")

	createIfNotExist := false
	truncate := false
	if preAction == "1" {
		createIfNotExist = true
	} else if preAction == "2" {
		truncate = true
	} else if preAction == "3" {
		createIfNotExist = true
		truncate = true
	}

	if err := prepareAheadWritingTable(svr.db, tableName, createIfNotExist, truncate); err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	appender, err := svr.db.Appender(tableName)
	if err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	defer appender.Close()

	columnTypes := []string{"string", "datetime", "double"}

	csvReader := csv.NewReader(ctx.Request.Body)
	for {
		fields, err := csvReader.Read()
		if err != nil {
			if err != io.EOF {
				rsp.Reason = err.Error()
				ctx.JSON(http.StatusBadRequest, rsp)
				return
			}
			break
		}

		values := make([]any, len(columnTypes))
		for i, field := range fields {
			if i >= len(columnTypes) {
				rsp.Reason = fmt.Sprintf("too many columns; table %s has %d columns", tableName, len(columnTypes))
				ctx.JSON(http.StatusBadRequest, rsp)
				return
			}
			switch columnTypes[i] {
			case "string":
				values[i] = field
			case "datetime":
				var ts int64
				if ts, err = strconv.ParseInt(field, 10, 64); err != nil {
					rsp.Reason = fmt.Sprintf("unable parse time in timeformat")
					ctx.JSON(http.StatusBadRequest, rsp)
					return
				}
				switch timeformat {
				case "s":
					values[i] = time.Unix(ts, 0)
				case "ms":
					values[i] = time.Unix(0, ts*int64(time.Millisecond))
				case "us":
					values[i] = time.Unix(0, ts*int64(time.Microsecond))
				default: // "ns"
					values[i] = time.Unix(0, ts)
				}
			case "double":
				if values[i], err = strconv.ParseFloat(field, 64); err != nil {
					values[i] = math.NaN()
				}
			case "int":
				if values[i], err = strconv.ParseInt(field, 10, 32); err != nil {
					values[i] = math.NaN()
				}
			case "int64":
				if values[i], err = strconv.ParseInt(field, 10, 64); err != nil {
					values[i] = math.NaN()
				}
			default:
				rsp.Reason = fmt.Sprintf("unsupported column type; %s", columnTypes[i])
				ctx.JSON(http.StatusBadRequest, rsp)
				return
			}
		}
		appender.Append(values...)
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

	msg.Write(svr.db, req, rsp)

	if rsp.Success {
		ctx.JSON(http.StatusOK, rsp)
	} else {
		ctx.JSON(http.StatusInternalServerError, rsp)
	}
}

func prepareAheadWritingTable(db *mach.Database, tableName string, createIfNotExist bool, truncate bool) error {
	tableName = strings.ToUpper(tableName)
	var tableExists = 0
	row := db.QueryRow("select count(*) from M$SYS_TABLES where name = ?", tableName)
	if err := row.Scan(&tableExists); err != nil {
		return err
	}
	if tableExists == 0 {
		if createIfNotExist {
			result := db.Exec("create tag table " + tableName + " (name varchar(100) primary key, time datetime basetime, value double)")
			if err := result.Err(); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("table does not exist")
		}
	}
	if truncate {
		result := db.Exec("truncate table " + tableName)
		if err := result.Err(); err != nil {
			return err
		}
	}
	return nil
}
