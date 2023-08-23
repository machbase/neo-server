package do

import (
	"fmt"
	"time"

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

func WriteLineProtocol(db spi.Database, dbName string, descColumns ColumnDescriptions, measurement string, fields map[string]any, tags map[string]string, ts time.Time) spi.Result {
	columns := descColumns.Columns().Names()
	columns = columns[:3]

	colTypes := descColumns.Columns().Types()
	for idx, val := range descColumns.Columns().Names() {
		if _, ok := tags[val]; ok {
			if colTypes[idx] == spi.ColumnBufferTypeString {
				columns = append(columns, val)
			}
		}
	}

	rows := make([][]any, 0)

	for k, v := range fields {
		values := make([]any, 0)
		values = append(values, fmt.Sprintf("%s.%s", measurement, k))
		values = append(values, ts)

		switch val := v.(type) {
		case float32:
			values = append(values, float64(val))
		case float64:
			values = append(values, val)
		case int:
			values = append(values, float64(val))
		case int32:
			values = append(values, float64(val))
		case int64:
			values = append(values, float64(val))
		default:
			// fmt.Printf("unsupproted value type '%T' of field '%s'\n", val, k)
			continue
		}

		for i := 3; i < len(columns); i++ {
			values = append(values, tags[columns[i]])
		}

		rows = append(rows, values)
	}

	return Insert(db, dbName, columns, rows)
}
