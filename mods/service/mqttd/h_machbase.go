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

	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/service/mqttd/mqtt"
	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/tql"
	"github.com/machbase/neo-server/mods/util"
	spi "github.com/machbase/neo-spi"
)

func (svr *mqttd) onMachbase(evt *mqtt.EvtMessage, prefix string) error {
	topic := evt.Topic
	topic = strings.TrimPrefix(topic, prefix+"/")
	peer, ok := svr.mqttd.GetPeer(evt.PeerId)
	if !ok {
		peer = nil
	}

	if topic == "query" {
		return svr.handleQuery(peer, evt.Raw, prefix+"/reply", 1)
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
		peer.Publish(replyTopic, replyQoS, rsp.Content)
	}()

	err := json.Unmarshal(payload, req)
	if err != nil {
		rsp.Reason = err.Error()
		return nil
	}
	if req.ReplyTo != "" {
		replyTopic = req.ReplyTo
	}
	var timeLocation = parseTimeLocation(req.TimeLocation, time.UTC)

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

	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		rsp.Reason = err.Error()
		return nil
	}
	defer conn.Close()

	queryCtx := &do.QueryContext{
		Conn: conn,
		Ctx:  ctx,
		OnFetchStart: func(cols spi.Columns) {
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
		peerLog.Warnf("%s unsupported format %q", topic, wp.Format)
		return nil
	}
	switch wp.Compress {
	case "": // no compression
	case "-": // no compression
	case "gzip": // gzip compression
	default: // others
		peerLog.Warnf("%s unsupproted compress %q", topic, wp.Compress)
		return nil
	}

	if wp.Table == "" {
		peerLog.Warn(topic, "table is not specified")
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		peerLog.Warn(topic, err.Error())
		return nil
	}
	defer conn.Close()

	exists, err := do.ExistsTable(ctx, conn, wp.Table)
	if err != nil {
		peerLog.Warnf(topic, err.Error())
		return nil
	}
	if !exists {
		peerLog.Warnf("%s Table %q does not exist", topic, wp.Table)
		return nil
	}

	var desc *do.TableDescription
	if desc0, err := do.Describe(ctx, conn, wp.Table, false); err != nil {
		peerLog.Warnf(topic, err.Error())
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

	codecOpts := []opts.Option{
		opts.InputStream(instream),
		opts.Timeformat("ns"),
		opts.TimeLocation(time.UTC),
		opts.TableName(wp.Table),
		opts.Columns(desc.Columns.Columns().Names()...),
		opts.ColumnTypes(desc.Columns.Columns().Types()...),
		opts.Delimiter(","),
		opts.Heading(false),
	}

	decoder := codec.NewDecoder(wp.Format, codecOpts...)
	lineno := 0
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
	insertQuery := fmt.Sprintf("insert into %s (%s) values(%s)", wp.Table, columnsHolder, valueHolder)

	for {
		vals, err := decoder.NextRow()
		if err != nil {
			if err != io.EOF {
				peerLog.Warnf(topic, err.Error())
				return nil
			}
			break
		}
		lineno++

		if result := conn.Exec(ctx, insertQuery, vals...); result.Err() != nil {
			peerLog.Warn(topic, result.Err().Error())
			return nil
		}
	}
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
	var appender spi.Appender
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
		if conn, err := svr.db.Connect(ctx, mach.WithTrustUser("sys")); err != nil {
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

	cols, _ := appender.Columns()
	codecOpts := []opts.Option{
		opts.InputStream(instream),
		opts.Timeformat("ns"),
		opts.TimeLocation(time.UTC),
		opts.TableName(wp.Table),
		opts.Columns(cols.Names()...),
		opts.ColumnTypes(cols.Types()...),
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

func parseTimeLocation(str string, def *time.Location) *time.Location {
	if str == "" {
		return def
	}

	tz := strings.ToLower(str)
	if tz == "local" {
		return time.Local
	} else if tz == "utc" {
		return time.UTC
	} else {
		if loc, err := util.GetTimeLocation(str); err != nil {
			loc, err := time.LoadLocation(str)
			if err != nil {
				return def
			}
			return loc
		} else {
			return loc
		}
	}
}
