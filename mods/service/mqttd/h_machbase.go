package mqttd

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/service/mqttd/mqtt"
	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/tql"
	"github.com/machbase/neo-server/mods/util"
)

func (svr *mqttd) onMachbase(evt *mqtt.EvtMessage) error {
	topic := evt.Topic
	topic = strings.TrimPrefix(topic, "db/")
	peer, ok := svr.mqttd.GetPeer(evt.PeerId)
	if !ok {
		peer = nil
	}

	if topic == "query" {
		return svr.handleQuery(peer, evt.Raw, "db/reply", 1)
	} else if strings.HasPrefix(topic, "write/") {
		return svr.handleWrite(peer, topic, evt.Raw)
	} else if strings.HasPrefix(topic, "append/") {
		return svr.handleAppend(peer, topic, evt.Raw)
	} else if strings.HasPrefix(topic, "tql/") && svr.tqlLoader != nil {
		return svr.handleTql(peer, topic, evt.Raw)
	} else {
		peer.GetLog().Warnf("---- invalid topic '%s'", evt.Topic)
	}
	return nil
}

func (svr *mqttd) handleQuery(peer mqtt.Peer, payload []byte, defaultReplyTopic string, replyQoS byte) error {
	tick := time.Now()
	req := &msg.QueryRequest{Format: "json", Timeformat: "ns", TimeLocation: "UTC", Precision: -1, Heading: true}
	rsp := &msg.QueryResponse{Reason: "not specified"}
	replyTopic := defaultReplyTopic
	defer func() {
		if peer == nil {
			return
		}
		rsp.Elapse = time.Since(tick).String()
		if len(rsp.Content) == 0 {
			buff, _ := json.Marshal(rsp)
			peer.Publish(replyTopic, replyQoS, buff)
		} else {
			peer.Publish(replyTopic, replyQoS, rsp.Content)
		}
	}()

	err := json.Unmarshal(payload, req)
	if err != nil {
		rsp.Reason = err.Error()
		return nil
	}
	if req.ReplyTo != "" {
		replyTopic = req.ReplyTo
	}
	timeLocation, err := util.ParseTimeLocation(req.TimeLocation, time.UTC)
	if err != nil {
		rsp.Reason = err.Error()
		return nil
	}

	var buffer = &bytes.Buffer{}
	var output spec.OutputStream
	switch req.Compress {
	case "gzip":
		output = &stream.WriterOutputStream{Writer: gzip.NewWriter(buffer)}
	default:
		req.Compress = ""
		output = &stream.WriterOutputStream{Writer: buffer}
	}

	encoder := codec.NewEncoder(req.Format,
		opts.OutputStream(output),
		opts.Timeformat(req.Timeformat),
		opts.Precision(req.Precision),
		opts.Rownum(req.Rownum),
		opts.Heading(req.Heading),
		opts.TimeLocation(timeLocation),
		opts.Delimiter(","),
		opts.BoxStyle("default"),
		opts.BoxSeparateColumns(true),
		opts.BoxDrawBorder(true),
		opts.RowsFlatten(req.RowsFlatten),
		opts.RowsArray(req.RowsArray),
		opts.Transpose(req.Transpose),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := svr.getTrustConnection(ctx, "sys")
	if err != nil {
		rsp.Reason = err.Error()
		return nil
	}
	defer conn.Close()

	queryCtx := &do.QueryContext{
		Conn: conn,
		Ctx:  ctx,
		OnFetchStart: func(cols api.Columns) {
			rsp.ContentType = encoder.ContentType()
			codec.SetEncoderColumns(encoder, cols)
			encoder.Open()
		},
		OnFetch: func(nrow int64, values []any) bool {
			err := encoder.AddRow(values)
			if err != nil {
				// report error to client?
				svr.log.Error("render", err.Error())
				return false
			}
			return true
		},
		OnFetchEnd: func() {
			encoder.Close()
			rsp.Success, rsp.Reason = true, "success"
			rsp.Content = buffer.Bytes()
		},
		OnExecuted: func(userMessage string, rowsAffected int64) {
			rsp.Success, rsp.Reason = true, userMessage
			rsp.Elapse = time.Since(tick).String()
		},
	}

	if _, err := do.Query(queryCtx, req.SqlText); err != nil {
		svr.log.Error("query fail", err.Error())
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
	}

	return nil
}

func (svr *mqttd) handleWrite(peer mqtt.Peer, topic string, payload []byte) error {
	tick := time.Now()
	var replyQoS = byte(0)
	var replyTopic string
	var rsp = &msg.WriteResponse{Reason: "not specified"}

	defer func() {
		if peer == nil || replyTopic == "" {
			return
		}
		rsp.Elapse = time.Since(tick).String()
		buff, _ := json.Marshal(rsp)
		peer.Publish(replyTopic, replyQoS, buff)
	}()

	peerLog := peer.GetLog()

	writePath := strings.ToUpper(strings.TrimPrefix(topic, "write/"))
	wp, err := util.ParseWritePath(writePath)
	if err != nil {
		peerLog.Warn(topic, err.Error())
		return nil
	}
	if wp.Format == "" {
		wp.Format = "json"
	}

	switch wp.Format {
	case "json":
	case "csv":
	default:
		rsp.Reason = fmt.Sprintf("%s unsupported format %q", topic, wp.Format)
		peerLog.Warnf(rsp.Reason)
		return nil
	}
	switch wp.Compress {
	case "": // no compression
	case "-": // no compression
	case "gzip": // gzip compression
	default: // others
		rsp.Reason = fmt.Sprintf("%s unsupproted compress %q", topic, wp.Compress)
		peerLog.Warnf(rsp.Reason)
		return nil
	}

	if wp.Table == "" {
		rsp.Reason = "table is not specified"
		peerLog.Warn(topic, rsp.Reason)
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, dbUser, tableName := do.TokenizeFullTableName(wp.Table)
	conn, err := svr.getTrustConnection(ctx, dbUser)
	if err != nil {
		rsp.Reason = err.Error()
		peerLog.Warn(topic, rsp.Reason)
		return nil
	}
	defer conn.Close()

	exists, err := do.ExistsTable(ctx, conn, wp.Table)
	if err != nil {
		rsp.Reason = err.Error()
		peerLog.Warn(topic, rsp.Reason)
		return nil
	}
	if !exists {
		peerLog.Warnf("%s Table %q does not exist", topic, wp.Table)
		return nil
	}

	var desc *do.TableDescription
	if desc0, err := do.Describe(ctx, conn, wp.Table, false); err != nil {
		rsp.Reason = err.Error()
		peerLog.Warn(topic, rsp.Reason)
		return nil
	} else {
		desc = desc0.(*do.TableDescription)
	}

	var instream spec.InputStream
	if wp.Compress == "gzip" {
		gr, err := gzip.NewReader(bytes.NewBuffer(payload))
		defer func() {
			if gr != nil {
				err = gr.Close()
				if err != nil {
					rsp.Reason = fmt.Sprintf("fail to close decompressor, %s", err.Error())
					peerLog.Warn("----", rsp.Reason)
				}
			}
		}()
		if err != nil {
			rsp.Reason = fmt.Sprintf("fail to unzip, %s", err.Error())
			peerLog.Warn("----", rsp.Reason)
			return nil
		}
		instream = &stream.ReaderInputStream{Reader: gr}
	} else {
		instream = &stream.ReaderInputStream{Reader: bytes.NewReader(payload)}
	}

	codecOpts := []opts.Option{
		opts.InputStream(instream),
		opts.Timeformat("ns"),
		opts.TimeLocation(time.UTC),
		opts.TableName(wp.Table),
		opts.Delimiter(","),
		opts.Heading(false),
	}

	var recno int
	var insertQuery string
	var columnNames []string
	var columnTypes []string

	if wp.Format == "json" {
		bs, err := io.ReadAll(instream)
		if err != nil {
			rsp.Reason = err.Error()
			peerLog.Warn("----", rsp.Reason)
			return nil
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
					peerLog.Warn("----", rsp.Reason)
					return nil
				}
				columnTypes = append(columnTypes, _type)
			}
			valueHolder := strings.Join(_hold, ",")
			insertQuery = fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", tableName, strings.Join(columnNames, ","), valueHolder)
		}
		instream = &stream.ReaderInputStream{Reader: bytes.NewBuffer(bs)}
	}

	if len(columnNames) == 0 {
		columnNames = desc.Columns.Columns().Names()
		columnTypes = desc.Columns.Columns().Types()
	}

	codecOpts = append(codecOpts,
		opts.InputStream(instream),
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
		peerLog.Errorf("----", rsp.Reason)
		return nil
	}

	for {
		vals, err := decoder.NextRow()
		if err != nil {
			if err != io.EOF {
				rsp.Reason = err.Error()
				peerLog.Warn(topic, err.Error())
				return nil
			}
			break
		}
		recno++

		if result := conn.Exec(ctx, insertQuery, vals...); result.Err() != nil {
			rsp.Reason = result.Err().Error()
			peerLog.Warn(topic, result.Err().Error())
			return nil
		}
	}

	rsp.Success, rsp.Reason = true, fmt.Sprintf("success, %d record(s) inserted", recno)
	return nil
}

func (svr *mqttd) handleAppend(peer mqtt.Peer, topic string, payload []byte) error {
	peerLog := peer.GetLog()

	writePath := strings.ToUpper(strings.TrimPrefix(topic, "append/"))
	wp, err := util.ParseWritePath(writePath)
	if err != nil {
		peerLog.Warn(topic, err.Error())
		return nil
	}

	if wp.Format == "" {
		wp.Format = "json"
	}

	switch wp.Format {
	case "json":
	case "csv":
	default:
		peerLog.Warnf("---- unsupported format '%s'", wp.Format)
		return nil
	}
	switch wp.Compress {
	case "": // no compression
	case "-": // no compression
	case "gzip": // gzip compression
	default: // others
		peerLog.Warnf("---- unsupproted compression '%s", wp.Compress)
		return nil
	}

	var appenderSet []*AppenderWrapper
	var appender api.Appender
	var peerId = peer.Id()

	if val, exists := svr.appenders.Get(peerId); exists {
		appenderSet = val.([]*AppenderWrapper)
		for _, a := range appenderSet {
			if a.appender.TableName() == wp.Table {
				appender = a.appender
				break
			}
		}
	}

	if appender == nil {
		ctx, ctxCancel := context.WithCancel(context.Background())
		tableNameFields := strings.SplitN(wp.Table, ".", 2)
		tableUser := "SYS"
		if len(tableNameFields) == 2 {
			tableUser = strings.ToUpper(tableNameFields[0])
			wp.Table = strings.ToUpper(tableNameFields[1])
		} else {
			wp.Table = strings.ToUpper(wp.Table)
		}

		if conn, err := svr.getTrustConnection(ctx, tableUser); err != nil {
			ctxCancel()
			return err
		} else {
			appender, err = conn.Appender(ctx, wp.Table)
			if err != nil {
				ctxCancel()
				peerLog.Errorf("---- fail to create appender, %s", err.Error())
				return nil
			}
			aw := &AppenderWrapper{
				conn:      conn,
				appender:  appender,
				ctx:       ctx,
				ctxCancel: ctxCancel,
			}
			if len(appenderSet) == 0 {
				appenderSet = []*AppenderWrapper{}
			}
			appenderSet = append(appenderSet, aw)
			svr.appenders.Set(peerId, appenderSet)
		}
	}

	var instream spec.InputStream

	if wp.Compress == "gzip" {
		gr, err := gzip.NewReader(bytes.NewBuffer(payload))
		defer func() {
			if gr != nil {
				err = gr.Close()
				if err != nil {
					peerLog.Warnf("---- fail to close decompressor, %s", err.Error())
				}
			}
		}()
		if err != nil {
			peerLog.Warnf("---- fail to gunzip, %s", err.Error())
			return nil
		}
		instream = &stream.ReaderInputStream{Reader: gr}
	} else {
		instream = &stream.ReaderInputStream{Reader: bytes.NewReader(payload)}
	}

	cols, _ := api.AppenderColumns(appender)
	colNames := cols.Names()
	colTypes := cols.Types()
	if api.AppenderTableType(appender) == api.LogTableType && colNames[0] == "_ARRIVAL_TIME" {
		colNames = colNames[1:]
		colTypes = colTypes[1:]
	}
	codecOpts := []opts.Option{
		opts.InputStream(instream),
		opts.Timeformat("ns"),
		opts.TimeLocation(time.UTC),
		opts.TableName(wp.Table),
		opts.Columns(colNames...),
		opts.ColumnTypes(colTypes...),
		opts.Delimiter(","),
		opts.Heading(false),
	}

	decoder := codec.NewDecoder(wp.Format, codecOpts...)

	recno := 0
	for {
		vals, err := decoder.NextRow()
		if err != nil {
			if err != io.EOF {
				peerLog.Warnf("---- append %s, %s", wp.Format, err.Error())
				return nil
			}
			break
		}
		err = appender.Append(vals...)
		if err != nil {
			peerLog.Errorf("---- append %s, %s", wp.Format, err.Error())
			break
		}
		recno++
	}
	peerLog.Debugf("---- appended %d record(s), %s", recno, topic)
	return nil
}

func (svr *mqttd) handleTql(peer mqtt.Peer, topic string, payload []byte) error {
	peerLog := peer.GetLog()

	if svr.tqlLoader == nil {
		peerLog.Error("tql is disabled.")
		return nil
	}

	rawQuery := strings.SplitN(strings.TrimPrefix(topic, "tql/"), "?", 2)
	if len(rawQuery) == 0 {
		peerLog.Warn(topic, "no tql path")
		return nil
	}
	path := rawQuery[0]
	if !strings.HasSuffix(path, ".tql") {
		peerLog.Warn(topic, "no tql found:", path)
		return nil
	}
	var params url.Values
	if len(path) == 2 {
		vs, err := url.ParseQuery(rawQuery[1])
		if err != nil {
			peerLog.Warn(topic, "tql invalid query:", rawQuery[1])
			return nil
		}
		params = vs
	}

	script, err := svr.tqlLoader.Load(path)
	if err != nil {
		peerLog.Warn(topic, "tql load fail", path, err.Error())
		return nil
	}

	task := tql.NewTaskContext(context.TODO())
	task.SetDatabase(svr.db)
	task.SetInputReader(bytes.NewBuffer(payload))
	task.SetOutputWriter(io.Discard)
	task.SetParams(params)
	if err := task.CompileScript(script); err != nil {
		svr.log.Error("tql parse fail", path, err.Error())
		return nil
	}

	result := task.Execute()
	if result == nil {
		svr.log.Error("tql execute error", path)
	} else if result.Err != nil {
		svr.log.Error("tql execute fail", path, result.Err.Error())
	}
	return nil
}
