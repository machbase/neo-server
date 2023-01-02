package msg

import (
	"fmt"
	"strings"

	mach "github.com/machbase/dbms-mach-go"
)

type WriteRequest struct {
	Table string            `json:"table"`
	Data  *WriteRequestData `json:"data"`
}

type WriteRequestData struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

type WriteResponse struct {
	Success bool               `json:"success"`
	Reason  string             `json:"reason"`
	Elapse  string             `json:"elapse"`
	Data    *WriteResponseData `json:"data,omitempty"`
}

type WriteResponseData struct {
	AffectedRows uint64 `json:"affectedRows"`
}

func Write(db *mach.Database, req *WriteRequest, rsp *WriteResponse) {
	vf := make([]string, len(req.Data.Columns))
	for i := range vf {
		vf[i] = "?"
	}
	valuesFormat := strings.Join(vf, ",")
	columns := strings.Join(req.Data.Columns, ",")

	sqlText := fmt.Sprintf("insert into %s (%s) values(%s)", req.Table, columns, valuesFormat)
	var nrows uint64
	for i, rec := range req.Data.Rows {
		_, err := db.Exec(sqlText, rec...)
		if err != nil {
			rsp.Reason = fmt.Sprintf("record[%d] %s", i, err.Error())
			rsp.Data = &WriteResponseData{
				AffectedRows: nrows,
			}
			return
		}
		nrows++
	}

	rsp.Success = true
	rsp.Reason = fmt.Sprintf("%d rows inserted", nrows)
	rsp.Data = &WriteResponseData{
		AffectedRows: nrows,
	}
}
