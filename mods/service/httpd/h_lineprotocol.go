package httpd

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/influxdata/line-protocol/v2/lineprotocol"
	"github.com/machbase/neo-server/mods/do"
)

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
	var desc *do.TableDescription
	if desc0, err := do.Describe(ctx, conn, dbName, false); err != nil {
		ctx.JSON(
			http.StatusBadRequest,
			gin.H{"error": fmt.Sprintf("column error: %s", err.Error())})
		return
	} else {
		desc = desc0.(*do.TableDescription)
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

		result := do.WriteLineProtocol(ctx, conn, dbName, desc.Columns, measurement, fields, tags, ts)
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
