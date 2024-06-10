package scheduler

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/bridge"
	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/model"
	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/tql"
	"github.com/machbase/neo-server/mods/util"
)

type SubscriberEntry struct {
	BaseEntry
	TaskTql string
	Bridge  string
	Topic   string
	QoS     int
	Queue   string

	s   *svr
	log logging.Log

	didOnConnectSubscriber bool
	shouldSubscribe        bool
	ctx                    context.Context
	ctxCancel              context.CancelFunc
	conn                   api.Conn
	appender               api.Appender
	subscriptionToken      any

	wd *util.WriteDescriptor
}

var _ Entry = &SubscriberEntry{}

func NewSubscriberEntry(s *svr, def *model.ScheduleDefinition) (*SubscriberEntry, error) {
	ret := &SubscriberEntry{
		BaseEntry: BaseEntry{name: def.Name, state: STOP, autoStart: def.AutoStart},
		TaskTql:   def.Task,
		Bridge:    def.Bridge,
		Topic:     def.Topic,
		QoS:       def.QoS,
		Queue:     def.Queue,
		s:         s,
		log:       logging.GetLog(fmt.Sprintf("subscriber-%s", strings.ToLower(def.Name))),
	}

	return ret, nil
}

func (ent *SubscriberEntry) Start() error {
	ent.state = STARTING
	ent.err = nil
	ent.shouldSubscribe = true
	ent.ctx, ent.ctxCancel = context.WithCancel(context.Background())

	if ent.didOnConnectSubscriber {
		return nil
	}
	if br0, err := bridge.GetBridge(ent.Bridge); err != nil {
		ent.state = FAILED
		ent.err = err
		return err
	} else {
		if wd, err := util.NewWriteDescriptor(ent.TaskTql); err != nil {
			return err
		} else {
			ent.wd = wd
		}
		switch br := br0.(type) {
		case bridge.MqttBridge:
			return ent.startMqtt(br)
		case bridge.NatsBridge:
			return ent.startNats(br)
		default:
			ent.state = FAILED
			ent.err = fmt.Errorf("%s is not a bridge of subscriber type", br0.String())
			return ent.err
		}
	}
}

func (ent *SubscriberEntry) startMqtt(br bridge.MqttBridge) error {
	if ent.Topic == "" {
		ent.state = FAILED
		ent.err = fmt.Errorf("empty topic is not allowed, subscribe to %s", br.String())
		return ent.err
	}

	ent.didOnConnectSubscriber = true
	br.OnConnect(func(bridge any) {
		if !ent.shouldSubscribe {
			return
		}
		if ok, err := br.Subscribe(ent.Topic, byte(ent.QoS), ent.doMqttTask); err != nil {
			ent.state = FAILED
			ent.err = err
		} else {
			if !ok {
				ent.state = FAILED
				ent.err = fmt.Errorf("fail to subscribe %s %s", br.String(), ent.Topic)
			} else {
				ent.state = RUNNING
			}
		}
	})
	br.OnDisconnect(func(bridge any) {
		if ent.shouldSubscribe {
			ent.state = STARTING
		} else {
			ent.state = STOP
		}
	})

	return nil
}

func (ent *SubscriberEntry) startNats(br bridge.NatsBridge) error {
	if ent.Topic == "" {
		ent.state = FAILED
		ent.err = fmt.Errorf("empty topic is not allowed, subscribe to %s", br.String())
		return ent.err
	}
	var token any
	if ent.Queue == "" {
		if t, err := br.Subscribe(ent.Topic, ent.doNatsTask); err != nil {
			ent.state = FAILED
			ent.err = err
		} else {
			token = t
		}
	} else {
		if t, err := br.QueueSubscribe(ent.Topic, ent.Queue, ent.doNatsTask); err != nil {
			ent.state = FAILED
			ent.err = err
		} else {
			token = t
		}
	}
	if token == nil {
		if ent.err == nil {
			ent.state = FAILED
			ent.err = fmt.Errorf("fail to subscribe %s %s", br.String(), ent.Topic)
		}
	} else {
		ent.subscriptionToken = token
		ent.state = RUNNING
	}
	return nil
}

func (ent *SubscriberEntry) Stop() error {
	ent.state = STOPPING
	ent.err = nil
	ent.shouldSubscribe = false
	defer func() {
		if ent.appender != nil {
			ent.appender.Close()
		}
		if ent.conn != nil {
			ent.conn.Close()
		}
		if ent.ctxCancel != nil {
			ent.ctxCancel()
		}
	}()

	if br0, err := bridge.GetBridge(ent.Bridge); err != nil {
		ent.state = FAILED
		ent.err = err
		return err
	} else {
		var ok bool
		var err error
		switch br := br0.(type) {
		case bridge.MqttBridge:
			ok, err = br.Unsubscribe(ent.Topic)
		case bridge.NatsBridge:
			ok, err = br.Unsubscribe(ent.subscriptionToken)
		default:
			ent.state = FAILED
			ent.err = fmt.Errorf("%s is not a bridge of subscriber type", br0.String())
			return ent.err
		}
		if err != nil {
			ent.state = FAILED
			ent.err = err
			return err
		} else {
			if !ok {
				ent.state = FAILED
				ent.err = fmt.Errorf("fail to unsubscribe %s %s", br0.String(), ent.Topic)
				return ent.err
			} else {
				ent.state = STOP
				return nil
			}
		}
	}
}

func (ent *SubscriberEntry) doMqttTask(topic string, payload []byte, msgId int, dup bool, retain bool) {
	tick := time.Now()
	rsp := &msg.WriteResponse{Reason: "not specified"}
	ent.log.Trace(ent.name, ent.TaskTql, "topic =", topic, "msgid =", msgId, "dup =", dup, "retain =", retain)
	defer func() {
		if ent.err != nil {
			ent.log.Warn(ent.name, ent.TaskTql, ent.state.String(), ent.err.Error(), time.Since(tick).String())
		} else {
			ent.log.Trace(ent.name, ent.TaskTql, ent.state.String(), time.Since(tick).String())
		}
	}()
	if ent.wd.IsTqlDestination() {
		sc, err := ent.s.tqlLoader.Load(ent.TaskTql)
		if err != nil {
			ent.err = err
			ent.state = FAILED
			ent.Stop()
			return
		}
		params := map[string][]string{}
		params["TOPIC"] = []string{topic}
		params["MSGID"] = []string{fmt.Sprintf("%d", msgId)}
		params["DUP"] = []string{fmt.Sprintf("%t", dup)}
		params["RETAIN"] = []string{fmt.Sprintf("%t", retain)}
		task := tql.NewTaskContext(context.TODO())
		task.SetDatabase(ent.s.db)
		task.SetInputReader(bytes.NewBuffer(payload))
		task.SetOutputWriterJson(io.Discard, true)
		task.SetParams(params)
		if err := task.CompileScript(sc); err != nil {
			ent.err = err
			ent.state = FAILED
			ent.Stop()
			return
		}
		if result := task.Execute(); result == nil || result.Err != nil {
			ent.err = err
			ent.state = FAILED
			ent.Stop()
		}
	} else {
		if ent.wd.Method == "append" {
			ent.doAppend(payload, rsp)
		} else {
			ent.doInsert(payload, rsp)
		}
	}
}

func (ent *SubscriberEntry) doNatsTask(topic string, payload []byte, header map[string][]string, replier func([]byte)) {
	tick := time.Now()
	rsp := &msg.WriteResponse{Reason: "not specified"}

	defer func() {
		rsp.Elapse = time.Since(tick).String()
		if ent.err != nil {
			rsp.Reason = ent.err.Error()
			ent.log.Warn(ent.name, ent.TaskTql, ent.state.String(), ent.err.Error(), rsp.Elapse)
		} else {
			ent.log.Trace(ent.name, ent.TaskTql, ent.state.String(), rsp.Reason, rsp.Elapse)
		}
		if replier != nil {
			buff, _ := json.Marshal(rsp)
			replier(buff)
		}
	}()
	if ent.wd.IsTqlDestination() {
		sc, err := ent.s.tqlLoader.Load(ent.TaskTql)
		if err != nil {
			ent.err = err
			ent.state = FAILED
			ent.Stop()
			return
		}
		task := tql.NewTaskContext(context.TODO())
		task.SetDatabase(ent.s.db)
		task.SetInputReader(bytes.NewBuffer(payload))
		task.SetOutputWriterJson(io.Discard, true)
		params := map[string][]string{}
		for k, v := range header {
			params[k] = v
		}
		task.SetParams(params)
		if err := task.CompileScript(sc); err != nil {
			ent.err = err
			ent.state = FAILED
			ent.Stop()
			return
		}
		if result := task.Execute(); result == nil || result.Err != nil {
			ent.err = err
			ent.state = FAILED
			ent.Stop()
		} else {
			rsp.Success, rsp.Reason = true, "success"
		}
	} else {
		if ent.wd.Method == "append" {
			ent.doAppend(payload, rsp)
		} else {
			ent.doInsert(payload, rsp)
		}
	}
}

func (ent *SubscriberEntry) doInsert(payload []byte, rsp *msg.WriteResponse) {
	if ent.conn == nil {
		if conn, err := ent.s.db.Connect(ent.ctx, api.WithTrustUser("sys")); err != nil {
			rsp.Reason = fmt.Sprintf("%s %s %s", ent.name, ent.TaskTql, err.Error())
			ent.log.Warn(ent.TaskTql, err.Error())
			return
		} else {
			ent.conn = conn
		}
	}

	exists, err := do.ExistsTable(ent.ctx, ent.conn, ent.wd.Table)
	if err != nil {
		rsp.Reason = fmt.Sprintf("%s %s %s", ent.name, ent.TaskTql, err.Error())
		ent.log.Warn(ent.TaskTql, err.Error())
		return
	}
	if !exists {
		rsp.Reason = fmt.Sprintf("%s %s table %q does not exist", ent.name, ent.TaskTql, ent.wd.Table)
		ent.log.Warnf("%s Table %q does not exist", ent.TaskTql, ent.wd.Table)
		return
	}

	var desc *do.TableDescription
	if desc0, err := do.Describe(ent.ctx, ent.conn, ent.wd.Table, false); err != nil {
		rsp.Reason = fmt.Sprintf("%s %s", ent.TaskTql, err.Error())
		ent.log.Warnf(ent.TaskTql, err.Error())
		return
	} else {
		desc = desc0.(*do.TableDescription)
	}

	var instream spec.InputStream
	if ent.wd.Compress == "gzip" {
		gr, err := gzip.NewReader(bytes.NewBuffer(payload))
		defer func() {
			if gr != nil {
				err = gr.Close()
				if err != nil {
					ent.log.Warnf("fail to close decompressor, %s", err.Error())
				}
			}
		}()
		if err != nil {
			rsp.Reason = fmt.Sprintf("fail to decompress, %s", err.Error())
			ent.log.Warn("fail to decompress,", err.Error())
			return
		}
		instream = &stream.ReaderInputStream{Reader: gr}
	} else {
		instream = &stream.ReaderInputStream{Reader: bytes.NewReader(payload)}
	}

	codecOpts := []opts.Option{
		opts.InputStream(instream),
		opts.Timeformat(ent.wd.Timeformat),
		opts.TimeLocation(ent.wd.TimeLocation),
		opts.TableName(ent.wd.Table),
		opts.Delimiter(ent.wd.Delimiter),
		opts.Heading(ent.wd.Heading),
	}

	var recno int
	var insertQuery string
	var columnNames []string
	var columnTypes []string

	if ent.wd.Format == "json" {
		bs, err := io.ReadAll(instream)
		if err != nil {
			rsp.Reason = err.Error()
			ent.log.Warn(err.Error())
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
		// replyTopic := wr.ReplyTo

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
					rsp.Reason = fmt.Sprintf("%s column %q not found in the table %q", ent.name, colName, ent.wd.Table)
					ent.log.Warnf("column %q not found in the table %q", colName, ent.wd.Table)
					return
				}
				columnTypes = append(columnTypes, _type)
			}
			valueHolder := strings.Join(_hold, ",")
			insertQuery = fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", ent.wd.Table, strings.Join(columnNames, ","), valueHolder)
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
		insertQuery = fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", ent.wd.Table, columnsHolder, valueHolder)
	}

	decoder := codec.NewDecoder(ent.wd.Format, codecOpts...)

	if decoder == nil {
		rsp.Reason = fmt.Sprintf("%s codec %q not found", ent.name, ent.wd.Format)
		ent.log.Errorf("codec %q not found", ent.wd.Format)
		return
	}

	for {
		vals, err := decoder.NextRow()
		if err != nil {
			if err != io.EOF {
				rsp.Reason = fmt.Sprintf("%s %s", ent.TaskTql, err.Error())
				ent.log.Warn(ent.TaskTql, err.Error())
				return
			}
			break
		}
		recno++

		if result := ent.conn.Exec(ent.ctx, insertQuery, vals...); result.Err() != nil {
			ent.log.Warn(ent.name, ent.TaskTql, result.Err().Error())
			return
		}
	}
	records := "record"
	if recno > 1 {
		records = "records"
	}
	rsp.Success, rsp.Reason = true, fmt.Sprintf("%d %s inserted", recno, records)
}

func (ent *SubscriberEntry) doAppend(payload []byte, rsp *msg.WriteResponse) {
	if ent.conn == nil {
		if conn, err := ent.s.db.Connect(ent.ctx, api.WithTrustUser("sys")); err != nil {
			rsp.Reason = fmt.Sprintf("%s %s %s", ent.name, ent.TaskTql, err.Error())
			ent.log.Warn(ent.TaskTql, err.Error())
			return
		} else {
			ent.conn = conn
		}
	}

	if ent.appender == nil {
		if appd, err := ent.conn.Appender(ent.ctx, ent.wd.Table); err != nil {
			rsp.Reason = fmt.Sprintf("%s %s fail to create appender, %s", ent.name, ent.TaskTql, err.Error())
			ent.log.Warn(ent.TaskTql, err.Error())
		} else {
			ent.appender = appd
		}
	}

	var instream spec.InputStream
	if ent.wd.Compress == "gzip" {
		gr, err := gzip.NewReader(bytes.NewBuffer(payload))
		defer func() {
			if gr != nil {
				err = gr.Close()
				if err != nil {
					ent.log.Warnf("fail to close decompressor, %s", err.Error())
				}
			}
		}()
		if err != nil {
			rsp.Reason = fmt.Sprintf("fail to decompress, %s", err.Error())
			ent.log.Warn("fail to decompress,", err.Error())
			return
		}
		instream = &stream.ReaderInputStream{Reader: gr}
	} else {
		instream = &stream.ReaderInputStream{Reader: bytes.NewReader(payload)}
	}

	cols, _ := api.AppenderColumns(ent.appender)
	codecOpts := []opts.Option{
		opts.InputStream(instream),
		opts.Timeformat(ent.wd.Timeformat),
		opts.TimeLocation(ent.wd.TimeLocation),
		opts.TableName(ent.wd.Table),
		opts.Columns(cols.Names()...),
		opts.ColumnTypes(cols.Types()...),
		opts.Delimiter(ent.wd.Delimiter),
		opts.Heading(ent.wd.Heading),
	}

	decoder := codec.NewDecoder(ent.wd.Format, codecOpts...)

	if decoder == nil {
		rsp.Reason = fmt.Sprintf("%s codec %q not found", ent.name, ent.wd.Format)
		ent.log.Errorf("codec %q not found", ent.wd.Format)
		return
	}

	recno := 0
	for {
		vals, err := decoder.NextRow()
		if err != nil {
			if err != io.EOF {
				rsp.Reason = fmt.Sprintf("append %s, %s", ent.wd.Format, err.Error())
				ent.log.Warnf("append %s, %s", ent.wd.Format, err.Error())
				return
			}
			break
		}
		err = ent.appender.Append(vals...)
		if err != nil {
			rsp.Reason = fmt.Sprintf("append %s, %s on the %d'th record", ent.wd.Format, err.Error(), recno+1)
			ent.log.Warnf("append %s, %s on the %d'th record", ent.wd.Format, err.Error(), recno+1)
			break
		}
		recno++
	}
	records := "record"
	if recno > 1 {
		records = "records"
	}
	rsp.Success, rsp.Reason = true, fmt.Sprintf("%d %s appended", recno, records)
}
