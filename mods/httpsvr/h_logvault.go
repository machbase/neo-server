package httpsvr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (svr *Server) handleLogVault(ctx *gin.Context) {
	oper := ctx.Param("oper")
	contentType := ctx.Request.Header.Get("Content-type")
	switch contentType {
	case "application/protobuf":
	default:
		switch oper {
		case "push":
			svr.handlePushLogVaultJson(ctx)
		default:
			ctx.String(http.StatusNotFound, fmt.Sprintf("unsupproted operation '%s'", oper))
		}
	}
}

type PushJson struct {
	Streams []*StreamJson `json:"streams"`
}

type StreamJson struct {
	Labels map[string]string `json:"stream"`
	Values []*EntryJson      `json:"values"`
}

type EntryJson struct {
	Timestamp int64  `json:"timestamp"`
	Line      string `json:"line"`
}

func (e *EntryJson) UnmarshalJSON(data []byte) error {
	d := [2]string{}
	json.Unmarshal(data, &d)
	ts, err := strconv.ParseInt(d[0], 10, 64)
	if err != nil {
		return err
	}
	e.Timestamp = ts
	e.Line = d[1]
	return nil
}

func (svr *Server) handlePushLogVaultJson(ctx *gin.Context) {
	if svr.logvaultAppender == nil {
		ctx.String(http.StatusInternalServerError, "logvault appender not found")
		return
	}

	push := &PushJson{}
	err := ctx.Bind(push)
	if err != nil {
		svr.log.Errorf(err.Error())
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}
	values := [6]any{}
	for _, stream := range push.Streams {
		if host, ok := stream.Labels["host"]; ok {
			values[1] = host
		}
		if dc, ok := stream.Labels["dc"]; ok {
			values[2] = dc
		}
		if job, ok := stream.Labels["job"]; ok {
			values[3] = job
		}
		if labels, err := json.Marshal(stream.Labels); err == nil {
			values[4] = string(labels)
		}
		for _, v := range stream.Values {
			values[0] = v.Timestamp
			values[5] = v.Line
			// svr.log.Tracef("%#v", values)
			err := svr.logvaultAppender.Append(values[:]...)
			if err != nil {
				svr.log.Error(err.Error())
				ctx.String(http.StatusInternalServerError, err.Error())
				return
			}
		}
	}
	ctx.String(http.StatusNoContent, "")
}

var LogVaultTable = "LOGVAULT"

func (svr *Server) checkLogTable() {
	row := svr.db.QueryRow("select count(*) from M$SYS_TABLES where name = ?", LogVaultTable)

	var createTable = true
	var n = 0
	err := row.Scan(&n)
	if err != nil || n == 0 {
		createTable = true
	} else {
		/// drop table for test
		// svr.db.Exec("drop table " + LogVaultTable)
		// createTable = true
		///
		createTable = false
	}

	if createTable {
		_, err := svr.db.Exec(svr.db.SqlTidy(
			`CREATE TABLE ` + LogVaultTable + `(
				ts     datetime,
				host   varchar(80),
				dc     varchar(80),
				job    varchar(100),
				labels varchar(1000),
				line   text
			)`))
		if err != nil {
			svr.log.Error("fail to create log vault table", LogVaultTable, err.Error())
		}

		_, err = svr.db.Exec(`CREATE INDEX ` + LogVaultTable + `_IDX ON ` + LogVaultTable + ` (line) INDEX_TYPE KEYWORD`)
		if err != nil {
			svr.log.Error("fail to create log vault index", LogVaultTable, err.Error())
		}
	}

	svr.logvaultAppender, err = svr.db.Appender(LogVaultTable)
	if err != nil {
		svr.log.Error("fail to create log vault appender", LogVaultTable, err.Error())
	}
}
