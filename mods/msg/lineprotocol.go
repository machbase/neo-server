package msg

import (
	"errors"
	"fmt"
	"time"

	mach "github.com/machbase/dbms-mach-go"
)

/* Interpreting Influx lineprotocol

   | Machbase            | influxdb                                    |
   | ------------------- | ------------------------------------------- |
   | table name          | db                                          |
   | tag name            | measurement + '.' + field name              |
   | time                | timestamp                                   |
   | value               | value of the field (if it is not a number type, will be ignored and not inserted) |
*/

func WriteLineProtocol(db *mach.Database, dbName string, measurement string, fields map[string]any, tags map[string]string, ts time.Time) error {
	columns := make([]string, 0)
	rows := make([][]any, 0)

	columns = append(columns, "name", "time", "value")

	for k, v := range fields {
		name := fmt.Sprintf("%s.%s", measurement, k)
		value := float64(0)
		timestamp := ts

		switch val := v.(type) {
		case float32:
			value = float64(val)
		case float64:
			value = val
		case int:
			value = float64(val)
		case int32:
			value = float64(val)
		case int64:
			value = float64(val)
		default:
			// fmt.Printf("unsupproted value type '%T' of field '%s'\n", val, k)
			continue
		}
		rows = append(rows, []any{name, timestamp, value})
	}

	if len(rows) == 0 {
		// fields의 value가 string 일 경우 입력할 값이 없을 수 있다.
		return nil
	}
	writeReq := &WriteRequest{
		Table: dbName,
		Data: &WriteRequestData{
			Columns: columns,
			Rows:    rows,
		},
	}
	writeRsp := &WriteResponse{}
	// fmt.Printf("REQ ==> %s %+v\n", writeReq.Table, writeReq.Data)
	Write(db, writeReq, writeRsp)
	// fmt.Printf("RSP ==> %#v\n", writeRsp)
	if writeRsp.Success {
		return nil
	} else {
		return errors.New(writeRsp.Reason)
	}
}
