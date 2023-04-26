package mqttd

import (
	"fmt"
	"strings"

	"github.com/machbase/neo-server/mods/service/msg"
	spi "github.com/machbase/neo-spi"
)

func Write(db spi.Database, req *msg.WriteRequest, rsp *msg.WriteResponse) {
	vf := make([]string, len(req.Data.Columns))
	for i := range vf {
		vf[i] = "?"
	}
	valuesFormat := strings.Join(vf, ",")
	columns := strings.Join(req.Data.Columns, ",")

	sqlText := fmt.Sprintf("insert into %s (%s) values(%s)", req.Table, columns, valuesFormat)
	var nrows uint64
	for i, rec := range req.Data.Rows {
		result := db.Exec(sqlText, rec...)
		if result.Err() != nil {
			rsp.Reason = fmt.Sprintf("record[%d] %s", i, result.Err().Error())
			rsp.Data = &msg.WriteResponseData{
				AffectedRows: nrows,
			}
			return
		}
		nrows++
	}

	rsp.Success = true
	rsp.Reason = fmt.Sprintf("%d rows inserted", nrows)
	rsp.Data = &msg.WriteResponseData{
		AffectedRows: nrows,
	}
}
