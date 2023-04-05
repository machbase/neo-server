package mqttsvr

import (
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/machbase/neo-server/mods/util"
	spi "github.com/machbase/neo-spi"
)

func Query(db spi.Database, req *msg.QueryRequest, rsp *msg.QueryResponse) {
	rows, err := db.Query(req.SqlText)
	if err != nil {
		rsp.Reason = err.Error()
		return
	}
	defer rows.Close()

	if !rows.IsFetchable() {
		rsp.Success = true
		rsp.Reason = "success"
		return
	}
	data := &msg.QueryData{}
	data.Rows = make([][]any, 0)
	cols, err := rows.Columns()
	if err != nil {
		rsp.Reason = err.Error()
		return
	}
	data.Columns = cols.NamesWithTimeLocation(time.UTC)
	data.Types = cols.Types()

	timeformat := util.GetTimeformat(req.Timeformat)
	nrow := 0
	if req.Format == "csv" {
		bytesBuff := &bytes.Buffer{}
		var csvWriter *csv.Writer
		var gzipWriter *gzip.Writer
		rsp.ContentType = "text/csv"
		switch req.Compress {
		case "gzip":
			rsp.ContentEncoding = "gzip"
			gzipWriter = gzip.NewWriter(bytesBuff)
			csvWriter = csv.NewWriter(gzipWriter)
		default:
			csvWriter = csv.NewWriter(bytesBuff)
		}
		csvWriter.Write(data.Columns)
		values := make([]string, len(cols))
		buffer := cols.MakeBuffer()
		for rows.Next() {
			err := rows.Scan(buffer...)
			if err != nil {
				rsp.Reason = err.Error()
				return
			}
			nrow++
			for i, n := range buffer {
				values[i] = ""
				if n == nil {
					continue
				}
				switch v := n.(type) {
				case *int:
					values[i] = strconv.FormatInt(int64(*v), 10)
				case *int32:
					values[i] = strconv.FormatInt(int64(*v), 10)
				case *int64:
					values[i] = strconv.FormatInt(*v, 10)
				case *float32:
					values[i] = strconv.FormatFloat(float64(*v), 'f', -1, 32)
				case *float64:
					values[i] = strconv.FormatFloat(*v, 'f', -1, 64)
				case *string:
					values[i] = *v
				case *time.Time:
					switch timeformat {
					case "ns":
						values[i] = strconv.FormatInt(v.UnixNano(), 10)
					case "ms":
						values[i] = strconv.FormatInt(v.UnixMilli(), 10)
					case "us":
						values[i] = strconv.FormatInt(v.UnixMicro(), 10)
					case "s":
						values[i] = strconv.FormatInt(v.Unix(), 10)
					default: // ns
						values[i] = strconv.FormatInt(v.UnixNano(), 10)
					}
				default:
					values[i] = fmt.Sprintf("%v", v)
				}
			}
			csvWriter.Write(values)
		}
		csvWriter.Flush()
		if gzipWriter != nil {
			gzipWriter.Flush()
		}
		rsp.Content = bytesBuff.Bytes()
	} else {
		rsp.ContentType = "application/json"
		for rows.Next() {
			rec := cols.MakeBuffer()
			err := rows.Scan(rec...)
			if err != nil {
				rsp.Reason = err.Error()
				return
			}
			nrow++
			for i, r := range rec {
				if v, ok := r.(time.Time); ok {
					switch timeformat {
					case "ns":
						rec[i] = strconv.FormatInt(v.UnixNano(), 10)
					case "ms":
						rec[i] = strconv.FormatInt(v.UnixMilli(), 10)
					case "us":
						rec[i] = strconv.FormatInt(v.UnixMicro(), 10)
					case "s":
						rec[i] = strconv.FormatInt(v.Unix(), 10)
					default: // ns
						rec[i] = strconv.FormatInt(v.UnixNano(), 10)
					}
				}
			}
			data.Rows = append(data.Rows, rec)
		}
		rsp.Data = data
	}

	rsp.Success = true
	rsp.Reason = fmt.Sprintf("%d rows selected", nrow)
}
