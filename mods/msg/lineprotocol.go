package msg

import (
	"fmt"
	"time"

	"github.com/machbase/neo-shell/do"
	spi "github.com/machbase/neo-spi"
)

/* Interpreting Influx lineprotocol

   | Machbase            | influxdb                                    |
   | ------------------- | ------------------------------------------- |
   | table name          | db                                          |
   | tag name            | measurement + '.' + field name              |
   | time                | timestamp                                   |
   | value               | value of the field (if it is not a number type, will be ignored and not inserted) |
*/

func WriteLineProtocol(db spi.Database, dbName string, measurement string, fields map[string]any, tags map[string]string, ts time.Time) spi.Result {
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

	return do.Insert(db, dbName, columns, rows)
}
