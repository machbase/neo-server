package mqtt2

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

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/util"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

func (s *mqtt2) handleWrite(cl *mqtt.Client, pk packets.Packet) {
	tick := time.Now()
	var replyTopic string
	var rsp = &msg.WriteResponse{Reason: "not specified"}

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

	_, dbUser, tableName := do.TokenizeFullTableName(wp.Table)
	conn, err := s.db.Connect(ctx, api.WithTrustUser(dbUser))
	if err != nil {
		rsp.Reason = err.Error()
		s.log.Warn(cl.Net.Remote, rsp.Reason)
		return
	}
	defer conn.Close()

	exists, err := do.ExistsTable(ctx, conn, wp.Table)
	if err != nil {
		rsp.Reason = err.Error()
		s.log.Warn(cl.Net.Remote, rsp.Reason)
		return
	}
	if !exists {
		s.log.Warn(cl.Net.Remote, "Table", wp.Table, "does not exist")
		return
	}

	var desc *do.TableDescription
	if desc0, err := do.Describe(ctx, conn, wp.Table, false); err != nil {
		rsp.Reason = err.Error()
		s.log.Warn(cl.Net.Remote, rsp.Reason)
		return
	} else {
		desc = desc0.(*do.TableDescription)
	}

	var inputStream spec.InputStream
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
		inputStream = &stream.ReaderInputStream{Reader: gr}
	} else {
		inputStream = &stream.ReaderInputStream{Reader: bytes.NewReader(pk.Payload)}
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
	var columnTypes []string

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
		wr := msg.WriteRequest{}
		dec := json.NewDecoder(bytes.NewBuffer(bs))
		// ignore json decoder error, the payload json can be non-full-document json.
		dec.Decode(&wr)
		replyTopic = wr.ReplyTo
		if pk.ProtocolVersion == 5 && pk.Properties.ResponseTopic != "" {
			replyTopic = pk.Properties.ResponseTopic
		}

		if wr.Data != nil && len(wr.Data.Columns) > 0 {
			columnNames = wr.Data.Columns
			columnTypes = make([]string, 0, len(columnNames))
			_hold := make([]string, 0, len(columnNames))
			for _, colName := range columnNames {
				_hold = append(_hold, "?")
				_type := ""
				for _, d := range desc.Columns {
					if d.Name == strings.ToUpper(colName) {
						_type = d.TypeString()
						break
					}
				}
				if _type == "" {
					rsp.Reason = fmt.Sprintf("column %q not found in the table %q", colName, wp.Table)
					s.log.Warn(cl.Net.Remote, rsp.Reason)
					return
				}
				columnTypes = append(columnTypes, _type)
			}
			valueHolder := strings.Join(_hold, ",")
			insertQuery = fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", tableName, strings.Join(columnNames, ","), valueHolder)
		}
		inputStream = &stream.ReaderInputStream{Reader: bytes.NewBuffer(bs)}
	}

	if len(columnNames) == 0 {
		columnNames = desc.Columns.Columns().Names()
		columnTypes = make([]string, 0, len(desc.Columns))
		for _, c := range desc.Columns {
			columnTypes = append(columnTypes, c.TypeString())
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
