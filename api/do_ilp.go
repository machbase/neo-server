package api

import (
	"context"
	"fmt"
	"strings"
	"time"
)

/* Interpreting Influx line protocol

   | Machbase            | influxdb                                    |
   | ------------------- | ------------------------------------------- |
   | table name          | db                                          |
   | tag name            | measurement + '.' + field name              |
   | time                | timestamp                                   |
   | value               | value of the field (if it is not a number type, will be ignored and not inserted) |
*/

func WriteLineProtocol(ctx context.Context, conn Conn, dbName string, descColumns Columns, measurement string, fields map[string]any, tags map[string]string, ts time.Time) Result {
	columns := descColumns.Names()
	columns = columns[:3]

	/*
		Machbase : name, time, value, host
		influxdb : tags key[DC, HOST, NAME, SYSTEM]
		=> HOST append / DC, NAME, SYSTEM not append
	*/
	compareNames := descColumns.Names()
	compareTypes := descColumns.DataTypes()
	compareNames = compareNames[3:]
	compareTypes = compareTypes[3:]
	for idx, val := range compareNames {
		if _, ok := tags[val]; ok {
			if compareTypes[idx] == DataTypeString {
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
			// unsupported value type
			continue
		}

		for i := 3; i < len(columns); i++ {
			values = append(values, tags[columns[i]])
		}

		rows = append(rows, values)
	}

	if len(rows) == 0 {
		return &InsertResult{
			rowsAffected: 0,
			message:      "no rows inserted",
		}
	}

	vf := make([]string, len(columns))
	for i := range vf {
		vf[i] = "?"
	}
	tableName := dbName
	valuesPlaces := strings.Join(vf, ",")
	columnsPhrase := strings.Join(columns, ",")

	sqlText := fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", tableName, columnsPhrase, valuesPlaces)
	var numRows int
	for _, rec := range rows {
		result := conn.Exec(ctx, sqlText, rec...)
		if result.Err() != nil {
			return &InsertResult{
				err:          result.Err(),
				rowsAffected: numRows,
				message:      "batch inserts aborted - " + sqlText,
			}
		}
		numRows++
	}

	ret := &InsertResult{
		rowsAffected: numRows,
	}
	if numRows == 0 {
		ret.message = "no rows inserted"
	} else if numRows == 1 {
		ret.message = "a row inserted"
	} else {
		ret.message = fmt.Sprintf("%d rows inserted", numRows)
	}
	return ret
}

var _ Result = &InsertResult{}

type InsertResult struct {
	err          error
	rowsAffected int
	message      string
}

func (ir *InsertResult) Err() error {
	return ir.err
}

func (ir *InsertResult) RowsAffected() int64 {
	return int64(ir.rowsAffected)
}

func (ir *InsertResult) Message() string {
	return ir.message
}
