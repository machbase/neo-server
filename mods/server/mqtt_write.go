package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/influxdata/line-protocol/v2/lineprotocol"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/codec"
	"github.com/machbase/neo-server/v8/mods/codec/opts"
	"github.com/machbase/neo-server/v8/mods/util"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"github.com/tidwall/gjson"
)

func (s *mqttd) handleWrite(cl *mqtt.Client, pk packets.Packet) {
	tick := time.Now()
	var replyTopic string
	var rsp = &WriteResponse{Reason: "not specified"}

	defer func() {
		if replyTopic == "" {
			return
		}
		rsp.Elapse = time.Since(tick).String()
		buff, _ := json.Marshal(rsp)
		qos := pk.FixedHeader.Qos
		packetId := uint16(0)
		if qos > 0 {
			packetId = pk.PacketID
		}
		reply := packets.Packet{
			TopicName:       replyTopic,
			Origin:          cl.ID,
			Payload:         buff,
			ProtocolVersion: cl.Properties.ProtocolVersion,
			PacketID:        packetId, // if qos==0, packet id must be 0
			FixedHeader:     packets.FixedHeader{Remaining: len(buff), Type: packets.Publish, Qos: qos},
			Created:         time.Now().Unix(),
			Expiry:          time.Now().Unix() + s.broker.Options.Capabilities.MaximumMessageExpiryInterval,
		}
		code := reply.PublishValidate(s.broker.Options.Capabilities.TopicAliasMaximum)
		if code != packets.CodeSuccess {
			s.log.Error("publish validate", code.String())
			return
		}
		if err := cl.WritePacket(reply); err != nil {
			s.log.Error("write reply", err.Error())
		}
	}()

	headerSkip := false
	headerColumns := false
	delimiter := ","
	timeformat := "ns"
	tz := time.UTC

	writePath := strings.ToUpper(strings.TrimPrefix(pk.TopicName, "db/write/"))
	wp, err := util.ParseWritePath(writePath)
	if err != nil {
		rsp.Reason = err.Error()
		s.log.Warn(cl.Net.Remote, pk.TopicName, rsp.Reason)
		return
	}
	if pk.ProtocolVersion == 5 {
		if pk.Properties.ResponseTopic != "" {
			replyTopic = pk.Properties.ResponseTopic
		}
		for _, p := range pk.Properties.User {
			switch p.Key {
			case "format":
				wp.Format = p.Val
			case "compress":
				wp.Compress = p.Val
			case "reply":
				replyTopic = p.Val
			case "delimiter":
				delimiter = p.Val
			case "timeformat":
				timeformat = p.Val
			case "tz":
				tz, _ = util.ParseTimeLocation(p.Val, time.UTC)
			case "header":
				switch strings.ToLower(p.Val) {
				case "skip":
					headerSkip = true
				case "column", "columns":
					headerColumns = true
					headerSkip = true
				default:
				}
			}
		}
	}
	if wp.Format == "" {
		wp.Format = "json"
	}

	switch wp.Format {
	case "json":
	case "csv":
	case "ndjson":
	default:
		rsp.Reason = fmt.Sprintf("%s unsupported format %q", pk.TopicName, wp.Format)
		s.log.Warnf(cl.Net.Remote, rsp.Reason)
		return
	}
	switch wp.Compress {
	case "": // no compression
	case "-": // no compression
	case "gzip": // gzip compression
	default: // others
		rsp.Reason = fmt.Sprintf("%s unsupported compress %q", pk.TopicName, wp.Compress)
		s.log.Warn(cl.Net.Remote, rsp.Reason)
		return
	}

	if wp.Table == "" {
		rsp.Reason = "table is not specified"
		s.log.Warn(cl.Net.Remote, pk.TopicName, rsp.Reason)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, dbUser, tableName := api.TableName(wp.Table).Split()
	conn, err := s.db.Connect(ctx, api.WithTrustUser(dbUser))
	if err != nil {
		rsp.Reason = err.Error()
		s.log.Warn(cl.Net.Remote, rsp.Reason)
		return
	}
	defer conn.Close()

	exists, err := api.ExistsTable(ctx, conn, wp.Table)
	if err != nil {
		rsp.Reason = err.Error()
		s.log.Warn(cl.Net.Remote, rsp.Reason)
		return
	}
	if !exists {
		s.log.Warn(cl.Net.Remote, "Table", wp.Table, "does not exist")
		return
	}

	var desc *api.TableDescription
	if desc0, err := api.DescribeTable(ctx, conn, wp.Table, false); err != nil {
		rsp.Reason = err.Error()
		s.log.Warn(cl.Net.Remote, rsp.Reason)
		return
	} else {
		desc = desc0
	}

	var inputStream io.Reader
	if wp.Compress == "gzip" {
		gr, err := gzip.NewReader(bytes.NewBuffer(pk.Payload))
		defer func() {
			if gr != nil {
				err = gr.Close()
				if err != nil {
					rsp.Reason = fmt.Sprintf("fail to close decompressor, %s", err.Error())
					s.log.Warn(cl.Net.Remote, rsp.Reason)
				}
			}
		}()
		if err != nil {
			rsp.Reason = fmt.Sprintf("fail to unzip, %s", err.Error())
			s.log.Warn(cl.Net.Remote, rsp.Reason)
			return
		}
		inputStream = gr
	} else {
		inputStream = bytes.NewReader(pk.Payload)
	}

	codecOpts := []opts.Option{
		opts.InputStream(inputStream),
		opts.Timeformat(timeformat),
		opts.TimeLocation(tz),
		opts.TableName(wp.Table),
		opts.Delimiter(delimiter),
		opts.Header(headerSkip),
		opts.HeaderColumns(headerColumns),
	}

	var recNo int
	var insertQuery string
	var columnNames []string
	var columnTypes []api.DataType

	if wp.Format == "json" {
		bs, err := io.ReadAll(inputStream)
		if err != nil {
			rsp.Reason = err.Error()
			s.log.Warn(cl.Net.Remote, rsp.Reason)
			return
		}
		/// the json of payload can have 3 types of forms.
		// 1. Array of Array: [[field1, field2],[field1,field]]
		// 2. Array : [field1, field2]
		// 3. Full document:  {data:{rows:[[field1, field2],[field1,field2]]}}

		if reply := extractReplyTo(bs); reply != "" {
			replyTopic = reply
		}
		if names := extractColumns(bs); len(names) > 0 {
			columnNames = names
			columnTypes = make([]api.DataType, 0, len(columnNames))
			_hold := make([]string, 0, len(columnNames))
			for _, colName := range columnNames {
				_hold = append(_hold, "?")
				_type := api.ColumnTypeUnknown
				for _, d := range desc.Columns {
					if d.Name == strings.ToUpper(colName) {
						_type = d.Type
						break
					}
				}
				if _type == api.ColumnTypeUnknown {
					rsp.Reason = fmt.Sprintf("column %q not found in the table %q", colName, wp.Table)
					s.log.Warn(cl.Net.Remote, rsp.Reason)
					return
				}
				columnTypes = append(columnTypes, _type.DataType())
			}
			valueHolder := strings.Join(_hold, ",")
			insertQuery = fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", tableName, strings.Join(columnNames, ","), valueHolder)
		}
		inputStream = bytes.NewBuffer(bs)
	}

	if len(columnNames) == 0 {
		columnNames = desc.Columns.Names()
		columnTypes = make([]api.DataType, 0, len(desc.Columns))
		for _, c := range desc.Columns {
			columnTypes = append(columnTypes, c.Type.DataType())
		}
	}

	codecOpts = append(codecOpts,
		opts.InputStream(inputStream),
		opts.Columns(columnNames...),
		opts.ColumnTypes(columnTypes...),
	)
	if insertQuery == "" {
		_hold := []string{}
		for i := 0; i < len(desc.Columns); i++ {
			_hold = append(_hold, "?")
		}
		valueHolder := strings.Join(_hold, ",")
		_hold = []string{}
		for i := 0; i < len(desc.Columns); i++ {
			_hold = append(_hold, desc.Columns[i].Name)
		}
		columnsHolder := strings.Join(_hold, ",")
		insertQuery = fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", wp.Table, columnsHolder, valueHolder)
	}

	decoder := codec.NewDecoder(wp.Format, codecOpts...)

	if decoder == nil {
		rsp.Reason = fmt.Sprintf("codec %q not found", wp.Format)
		s.log.Error(cl.Net.Remote, rsp.Reason)
		return
	}

	var prevCols []string
	for {
		vals, cols, err := decoder.NextRow()
		if err != nil {
			if err != io.EOF {
				rsp.Reason = err.Error()
				s.log.Warn(cl.Net.Remote, pk.TopicName, rsp.Reason)
				return
			}
			break
		}
		recNo++

		if len(cols) != len(prevCols) && !slices.Equal(prevCols, cols) {
			prevCols = cols
			_hold := make([]string, len(cols))
			for i := range _hold {
				_hold[i] = "?"
			}
			insertQuery = fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", tableName, strings.Join(cols, ","), strings.Join(_hold, ","))
		}
		if result := conn.Exec(ctx, insertQuery, vals...); result.Err() != nil {
			rsp.Reason = result.Err().Error()
			s.log.Warn(cl.Net.Remote, pk.TopicName, rsp.Reason)
			return
		}
	}

	rsp.Success, rsp.Reason = true, fmt.Sprintf("success, %d record(s) inserted", recNo)
	s.log.Trace(cl.Net.Remote, rsp.Reason)
}

func (s *mqttd) handleAppend(cl *mqtt.Client, pk packets.Packet) {
	writePath := strings.TrimPrefix(strings.TrimPrefix(pk.TopicName, "db/append/"), "db/write/")
	writePath = strings.ToUpper(writePath)
	wp, err := util.ParseWritePath(writePath)
	if err != nil {
		s.log.Warn(cl.Net.Remote, pk.TopicName, err.Error())
		return
	}

	headerSkip := false
	headerColumns := false
	delimiter := ","
	timeformat := "ns"
	tz := time.UTC

	if pk.ProtocolVersion == 5 {
		for _, p := range pk.Properties.User {
			switch p.Key {
			case "format":
				wp.Format = p.Val
			case "compress":
				wp.Compress = p.Val
			case "delimiter":
				delimiter = p.Val
			case "timeformat":
				timeformat = p.Val
			case "tz":
				tz, _ = util.ParseTimeLocation(p.Val, time.UTC)
			case "header":
				switch strings.ToLower(p.Val) {
				case "skip":
					headerSkip = true
				case "column", "columns":
					headerSkip = true
					headerColumns = true
				default:
				}
			}
		}
	}

	if wp.Format == "" {
		wp.Format = "json"
	}

	switch wp.Format {
	case "json":
	case "csv":
	case "ndjson":
	default:
		s.log.Warn(cl.Net.Remote, "unsupported format:", wp.Format)
		return
	}
	switch wp.Compress {
	case "": // no compression
	case "-": // no compression
	case "gzip": // gzip compression
	default: // others
		s.log.Warn(cl.Net.Remote, "unsupported compression:", wp.Compress)
		return
	}

	tableNameFields := strings.SplitN(wp.Table, ".", 2)
	tableUser := "SYS"
	if len(tableNameFields) == 2 {
		tableUser = strings.ToUpper(tableNameFields[0])
		wp.Table = strings.ToUpper(tableNameFields[1])
	} else {
		wp.Table = strings.ToUpper(wp.Table)
	}
	var appenderName = tableUser + "." + wp.Table
	var appender api.Appender
	if aw, err := api.GetAppendWorker(context.TODO(), s.db, appenderName); err != nil {
		s.log.Warn(cl.Net.Remote, "fail to get append worker,", err.Error())
		return
	} else {
		appender = aw
	}
	defer appender.Close()

	var inputStream io.Reader

	if wp.Compress == "gzip" {
		gr, err := gzip.NewReader(bytes.NewBuffer(pk.Payload))
		defer func() {
			if gr != nil {
				err = gr.Close()
				if err != nil {
					s.log.Warn(cl.Net.Remote, "fail to close decompressor,", err.Error())
				}
			}
		}()
		if err != nil {
			s.log.Warn(cl.Net.Remote, "fail to gunzip,", err.Error())
			return
		}
		inputStream = gr
	} else {
		inputStream = bytes.NewReader(pk.Payload)
	}

	cols, _ := appender.Columns()
	colNames := cols.Names()
	colTypes := cols.DataTypes()
	if appender.TableType() == api.TableTypeLog && colNames[0] == "_ARRIVAL_TIME" {
		colNames = colNames[1:]
		colTypes = colTypes[1:]
	}
	codecOpts := []opts.Option{
		opts.InputStream(inputStream),
		opts.Timeformat(timeformat),
		opts.TimeLocation(tz),
		opts.TableName(wp.Table),
		opts.Columns(colNames...),
		opts.ColumnTypes(colTypes...),
		opts.Delimiter(delimiter),
		opts.Header(headerSkip),
		opts.HeaderColumns(headerColumns),
	}

	decoder := codec.NewDecoder(wp.Format, codecOpts...)

	recNo := 0
	hasProcessedHeader := false
	for {
		vals, cols, err := decoder.NextRow()
		if err != nil {
			if err != io.EOF {
				s.log.Warn(cl.Net.Remote, "append", wp.Format, err.Error())
				return
			}
			break
		}
		if !hasProcessedHeader && headerColumns && len(cols) > 0 {
			appender = appender.WithInputColumns(cols...)
			hasProcessedHeader = true
		}
		err = appender.Append(vals...)
		if err != nil {
			s.log.Warn(cl.Net.Remote, "append", wp.Format, err.Error())
			break
		}
		recNo++
	}
	s.log.Trace(cl.Net.Remote, "appended", recNo, "record(s),", wp.Table)
}

func (s *mqttd) handleMetrics(cl *mqtt.Client, pk packets.Packet) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := s.db.Connect(ctx, api.WithTrustUser("sys"))
	if err != nil {
		s.log.Warn(cl.Net.Remote, pk.TopicName, err.Error())
		return
	}
	defer conn.Close()

	dbName := strings.TrimPrefix(pk.TopicName, "db/metrics/")

	var desc *api.TableDescription
	if desc0, err := api.DescribeTable(ctx, conn, dbName, false); err != nil {
		s.log.Warn(cl.Net.Remote, "column error:", err.Error())
		return
	} else {
		desc = desc0
	}
	tableName := strings.ToUpper(dbName)
	timePrecision := lineprotocol.Nanosecond

	dec := lineprotocol.NewDecoder(bytes.NewBuffer(pk.Payload))
	if dec == nil {
		s.log.Warn(cl.Net.Remote, "lineprotocol decoder fail")
		return
	}
	for dec.Next() {
		m, err := dec.Measurement()
		if err != nil {
			s.log.Warn(cl.Net.Remote, "measurement error:", err.Error())
			return
		}
		measurement := string(m)
		tags := make(map[string]string)
		fields := make(map[string]any)

		for {
			key, val, err := dec.NextTag()
			if err != nil {
				s.log.Warn(cl.Net.Remote, "tag error:", err.Error())
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
				s.log.Warn(cl.Net.Remote, "field error:", err.Error())
				return
			}
			if key == nil {
				break
			}
			fields[string(key)] = val.Interface()
		}

		ts, err := dec.Time(timePrecision, time.Time{})
		if err != nil {
			s.log.Warn(cl.Net.Remote, "time error:", err.Error())
			return
		}
		if ts.IsZero() {
			s.log.Warn(cl.Net.Remote, "timestamp is zero")
			return
		}

		result := api.WriteLineProtocol(ctx, conn, tableName, desc.Columns, measurement, fields, tags, ts)
		if result.Err() != nil {
			s.log.Warnf(cl.Net.Remote, "lineprotocol fail:", result.Err().Error())
		}
	}
}

// extractColumns extracts column names from the payload.
func extractColumns(payload []byte) []string {
	cols := gjson.Get(string(payload), "data.columns")
	if !cols.Exists() || !cols.IsArray() {
		return nil
	}
	ret := []string{}
	cols.ForEach(func(key, value gjson.Result) bool {
		ret = append(ret, value.String())
		return true
	})
	return ret
}

func extractReplyTo(payload []byte) string {
	reply := gjson.Get(string(payload), "reply")
	if reply.Exists() {
		return reply.String()
	}
	return ""
}
