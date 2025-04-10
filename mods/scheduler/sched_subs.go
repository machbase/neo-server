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

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/codec"
	"github.com/machbase/neo-server/v8/mods/codec/opts"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/nats-io/nats.go"
	"github.com/tidwall/gjson"
)

type SubscriberEntry struct {
	BaseEntry
	TaskTql string
	Bridge  string
	Topic   string

	QoS        int    // mqtt only
	QueueName  string // nats only, Queue Group
	StreamName string // nats only, JetStream

	s   *svr
	log logging.Log

	shouldSubscribe bool
	ctx             context.Context
	ctxCancel       context.CancelFunc
	conn            api.Conn
	appender        api.Appender
	subscription    bridge.Subscription

	wd *util.WriteDescriptor
}

var _ Entry = (*SubscriberEntry)(nil)

func NewSubscriberEntry(s *svr, def *model.ScheduleDefinition) (*SubscriberEntry, error) {
	ret := &SubscriberEntry{
		BaseEntry:  BaseEntry{name: def.Name, state: STOP, autoStart: def.AutoStart},
		TaskTql:    def.Task,
		Bridge:     def.Bridge,
		Topic:      def.Topic,
		QoS:        def.QoS,
		QueueName:  def.QueueName,
		StreamName: def.StreamName,
		s:          s,
		log:        logging.GetLog(fmt.Sprintf("subscriber-%s", strings.ToLower(def.Name))),
	}
	return ret, nil
}

func (ent *SubscriberEntry) Start() error {
	ent.state = STARTING
	ent.err = nil
	ent.shouldSubscribe = true
	ent.ctx, ent.ctxCancel = context.WithCancel(context.Background())

	ent.log.Infof("starting, bridge=%s, topic=%s", ent.Bridge, ent.Topic)
	if br0, err := bridge.GetBridge(ent.Bridge); err != nil {
		ent.log.Tracef("get bridge, %s", err.Error())
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
		case *bridge.MqttBridge:
			return ent.startMqtt(br)
		case *bridge.NatsBridge:
			return ent.startNats(br)
		default:
			ent.state = FAILED
			ent.err = fmt.Errorf("%s is not a bridge of subscriber type", br0.String())
			return ent.err
		}
	}
}

func (ent *SubscriberEntry) doMqttOnConnect(br *bridge.MqttBridge) {
	if !ent.shouldSubscribe {
		return
	}
	if subscription, err := br.Subscribe(ent.Topic, byte(ent.QoS), ent.doMqttTask); err != nil {
		ent.state = FAILED
		ent.err = err
	} else {
		if subscription == nil {
			ent.state = FAILED
			ent.err = fmt.Errorf("fail to subscribe %s %s", br.String(), ent.Topic)
		} else {
			ent.subscription = subscription
			ent.state = RUNNING
			ent.err = nil
		}
	}
}

func (ent *SubscriberEntry) startMqtt(br *bridge.MqttBridge) error {
	if ent.Topic == "" {
		ent.state = FAILED
		ent.err = fmt.Errorf("empty topic is not allowed, subscribe to %s", br.String())
		return ent.err
	}
	if br.IsConnected() {
		ent.log.Tracef("bridge %s is already connected, renew subscription", br.String())
		ent.doMqttOnConnect(br)
		return ent.err
	}
	br.OnConnect(func(bridge any) {
		ent.doMqttOnConnect(br)
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

func (ent *SubscriberEntry) startNats(br *bridge.NatsBridge) error {
	if ent.Topic == "" {
		ent.state = FAILED
		ent.err = fmt.Errorf("empty topic is not allowed, subscribe to %s", br.String())
		return ent.err
	}
	subscribeOptions := []bridge.NatsSubscribeOption{}
	if ent.QueueName != "" {
		subscribeOptions = append(subscribeOptions, bridge.NatsQueueGroup(ent.QueueName))
	}
	if ent.StreamName != "" {
		subscribeOptions = append(subscribeOptions, bridge.NatsStreamName(ent.StreamName))
	}
	if sub, err := br.Subscribe(ent.Topic, ent.doNatsTask, subscribeOptions...); err != nil {
		ent.state = FAILED
		ent.err = err
	} else {
		if sub == nil {
			ent.state = FAILED
			ent.err = fmt.Errorf("fail to subscribe %s %s", br.String(), ent.Topic)
		} else {
			ent.subscription = sub
			ent.state = RUNNING
		}
	}
	return nil
}

func (ent *SubscriberEntry) Stop() error {
	ent.log.Infof("stopping, bridge=%s, topic=%s", ent.Bridge, ent.Topic)

	ent.state = STOPPING
	ent.err = nil
	ent.shouldSubscribe = false
	defer func() {
		if ent.appender != nil {
			ent.appender.Close()
			ent.appender = nil
		}
		if ent.conn != nil {
			ent.conn.Close()
			ent.conn = nil
		}
		if ent.ctxCancel != nil {
			ent.ctxCancel()
			ent.ctxCancel = nil
		}
	}()

	if ent.subscription == nil {
		ent.state = STOP
		return nil
	}

	if br0, err := bridge.GetBridge(ent.Bridge); err != nil {
		ent.state = FAILED
		ent.err = err
		return err
	} else {
		var err error
		switch br0.(type) {
		case *bridge.MqttBridge:
			err = ent.subscription.Unsubscribe()
		case *bridge.NatsBridge:
			err = ent.subscription.Unsubscribe()
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
			ent.state = STOP
			return nil
		}
	}
}

type Reason struct {
	Success bool
	Reason  string
	Elapse  string
}

func (ent *SubscriberEntry) doMqttTask(topic string, payload []byte, msgId int, dup bool, retain bool) {
	tick := time.Now()
	rsp := &Reason{Reason: "not specified"}

	defer func() {
		if ent.err != nil {
			ent.log.Warn(ent.name, ent.TaskTql, ent.state.String(), ent.err.Error(), time.Since(tick).String())
		} else {
			ent.log.Trace(ent.name, ent.TaskTql, ent.state.String(), time.Since(tick).String())
		}
	}()
	if ent.wd.IsTqlDestination() {
		params := map[string][]string{}
		params["TOPIC"] = []string{topic}
		params["MSGID"] = []string{fmt.Sprintf("%d", msgId)}
		params["DUP"] = []string{fmt.Sprintf("%t", dup)}
		params["RETAIN"] = []string{fmt.Sprintf("%t", retain)}
		ent.doTql(payload, params, rsp)
	} else {
		if ent.wd.Method == "append" {
			ent.doAppend(payload, rsp)
		} else {
			ent.doInsert(payload, rsp)
		}
	}
}

func (ent *SubscriberEntry) doNatsTask(natsMsg *nats.Msg) {
	tick := time.Now()
	rsp := &Reason{Reason: "not specified"}

	defer func() {
		rsp.Elapse = time.Since(tick).String()
		if ent.err != nil {
			rsp.Reason = ent.err.Error()
			ent.log.Warn(ent.name, ent.TaskTql, ent.state.String(), ent.err.Error(), rsp.Elapse)
		} else {
			ent.log.Trace(ent.name, ent.TaskTql, ent.state.String(), rsp.Reason, rsp.Elapse)
		}
		if natsMsg.Reply != "" {
			buff, _ := json.Marshal(rsp)
			natsMsg.Respond(buff)
		} else {
			natsMsg.Ack()
		}
	}()
	if ent.wd.IsTqlDestination() {
		ent.doTql(natsMsg.Data, natsMsg.Header, rsp)
	} else {
		if ent.wd.Method == "append" {
			ent.doAppend(natsMsg.Data, rsp)
		} else {
			ent.doInsert(natsMsg.Data, rsp)
		}
	}
}

func (ent *SubscriberEntry) doTql(payload []byte, header map[string][]string, rsp *Reason) {
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
	task.SetLogWriter(ent.log)
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
		if result != nil && result.Err != nil {
			ent.err = result.Err
		}
		ent.state = FAILED
		ent.Stop()
	} else {
		rsp.Success, rsp.Reason = true, "success"
	}
}

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

func (ent *SubscriberEntry) doInsert(payload []byte, rsp *Reason) {
	if ent.conn == nil {
		if conn, err := ent.s.db.Connect(ent.ctx, api.WithTrustUser("sys")); err != nil {
			rsp.Reason = fmt.Sprintf("%s %s %s", ent.name, ent.TaskTql, err.Error())
			ent.log.Warn(ent.TaskTql, err.Error())
			return
		} else {
			ent.conn = conn
		}
	}

	exists, err := api.ExistsTable(ent.ctx, ent.conn, ent.wd.Table)
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

	var desc *api.TableDescription
	if desc0, err := api.DescribeTable(ent.ctx, ent.conn, ent.wd.Table, false); err != nil {
		rsp.Reason = fmt.Sprintf("%s %s", ent.TaskTql, err.Error())
		ent.log.Warnf(ent.TaskTql, err.Error())
		return
	} else {
		desc = desc0
	}

	var inputStream io.Reader
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
		inputStream = gr
	} else {
		inputStream = bytes.NewReader(payload)
	}

	codecOpts := []opts.Option{
		opts.InputStream(inputStream),
		opts.Timeformat(ent.wd.Timeformat),
		opts.TimeLocation(ent.wd.TimeLocation),
		opts.TableName(ent.wd.Table),
		opts.Delimiter(ent.wd.Delimiter),
		opts.Heading(ent.wd.Heading),
	}

	var rownum uint64
	var insertQuery string
	var columnNames []string
	var columnTypes []api.DataType

	if ent.wd.Format == "json" {
		bs, err := io.ReadAll(inputStream)
		if err != nil {
			rsp.Reason = err.Error()
			ent.log.Warn(err.Error())
			return
		}
		/// the json of payload can have 3 types of forms.
		// 1. Array of Array: [[field1, field2],[field1,field]]
		// 2. Array : [field1, field2]
		// 3. Full document:  {data:{rows:[[field1, field2],[field1,field2]]}}

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
					rsp.Reason = fmt.Sprintf("%s column %q not found in the table %q", ent.name, colName, ent.wd.Table)
					ent.log.Warnf("column %q not found in the table %q", colName, ent.wd.Table)
					return
				}
				columnTypes = append(columnTypes, _type.DataType())
			}
			valueHolder := strings.Join(_hold, ",")
			insertQuery = fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", ent.wd.Table, strings.Join(columnNames, ","), valueHolder)
		}
		inputStream = bytes.NewBuffer(bs)
	}

	if len(columnNames) == 0 {
		columnNames = desc.Columns.Names()
		columnTypes = desc.Columns.DataTypes()
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
		insertQuery = fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", ent.wd.Table, columnsHolder, valueHolder)
	}

	decoder := codec.NewDecoder(ent.wd.Format, codecOpts...)

	if decoder == nil {
		rsp.Reason = fmt.Sprintf("%s codec %q not found", ent.name, ent.wd.Format)
		ent.log.Errorf("codec %q not found", ent.wd.Format)
		return
	}

	for {
		vals, _, err := decoder.NextRow()
		if err != nil {
			if err != io.EOF {
				rsp.Reason = fmt.Sprintf("%s %s", ent.TaskTql, err.Error())
				ent.log.Warn(ent.TaskTql, err.Error())
				return
			}
			break
		}
		rownum++

		if result := ent.conn.Exec(ent.ctx, insertQuery, vals...); result.Err() != nil {
			ent.log.Warn(ent.name, ent.TaskTql, result.Err().Error())
			return
		}
	}
	ent.subscription.AddInserted(rownum)
	records := "record"
	if rownum > 1 {
		records = "records"
	}
	rsp.Success, rsp.Reason = true, fmt.Sprintf("%s %s inserted", util.HumanizeNumber(rownum), records)
}

func (ent *SubscriberEntry) doAppend(payload []byte, rsp *Reason) {
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
		if appender, err := ent.conn.Appender(ent.ctx, ent.wd.Table); err != nil {
			rsp.Reason = fmt.Sprintf("%s %s fail to create appender, %s", ent.name, ent.TaskTql, err.Error())
			ent.log.Warn(ent.TaskTql, err.Error())
		} else {
			ent.appender = appender
		}
	}

	var inputStream io.Reader
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
		inputStream = gr
	} else {
		inputStream = bytes.NewReader(payload)
	}

	cols, _ := ent.appender.Columns()
	colNames := cols.Names()
	colTypes := cols.DataTypes()
	if ent.appender.TableType() == api.TableTypeLog && colNames[0] == "_ARRIVAL_TIME" {
		colNames = colNames[1:]
		colTypes = colTypes[1:]
	}
	codecOpts := []opts.Option{
		opts.InputStream(inputStream),
		opts.Timeformat(ent.wd.Timeformat),
		opts.TimeLocation(ent.wd.TimeLocation),
		opts.TableName(ent.wd.Table),
		opts.Columns(colNames...),
		opts.ColumnTypes(colTypes...),
		opts.Delimiter(ent.wd.Delimiter),
		opts.Heading(ent.wd.Heading),
	}

	decoder := codec.NewDecoder(ent.wd.Format, codecOpts...)

	if decoder == nil {
		rsp.Reason = fmt.Sprintf("%s codec %q not found", ent.name, ent.wd.Format)
		ent.log.Errorf("codec %q not found", ent.wd.Format)
		return
	}

	rownum := uint64(0)
	for {
		var values []any
		if vals, _, err := decoder.NextRow(); err != nil {
			if err != io.EOF {
				rsp.Reason = fmt.Sprintf("append %s, %s", ent.wd.Format, err.Error())
				ent.log.Warnf("append %s, %s", ent.wd.Format, err.Error())
				return
			}
			break
		} else {
			values = vals
		}
		if err := ent.appender.Append(values...); err != nil {
			rsp.Reason = fmt.Sprintf("append %s, %s on the %d'th record", ent.wd.Format, err.Error(), rownum+1)
			ent.log.Warnf("append %s, %s on the %d'th record", ent.wd.Format, err.Error(), rownum+1)
			break
		}
		rownum++
	}
	ent.subscription.AddAppended(rownum)
	records := "record"
	if rownum > 1 {
		records = "records"
	}
	rsp.Success, rsp.Reason = true, fmt.Sprintf("%s %s appended", util.HumanizeNumber(rownum), records)
}
