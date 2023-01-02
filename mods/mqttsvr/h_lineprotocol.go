package mqttsvr

import (
	"bytes"
	"strings"
	"time"

	"github.com/influxdata/line-protocol/v2/lineprotocol"
	"github.com/machbase/cemlib/mqtt"
	"github.com/machbase/dbms-mach-go/server/msg"
)

func (svr *Server) onLineprotocol(evt *mqtt.EvtMessage, prefix string) {
	dbName := strings.TrimPrefix(evt.Topic, prefix+"/")
	dec := lineprotocol.NewDecoder(bytes.NewBuffer(evt.Raw))
	if dec == nil {
		svr.log.Warnf("lineprotocol decoder fail")
		return
	}
	precision := lineprotocol.Nanosecond
	for dec.Next() {
		m, err := dec.Measurement()
		if err != nil {
			svr.log.Warnf("measurement error: %s", err.Error())
			return
		}
		measurement := string(m)
		tags := make(map[string]string)
		fields := make(map[string]any)

		for {
			key, val, err := dec.NextTag()
			if err != nil {
				svr.log.Warnf("tag error: %s", err.Error())
				return
			}
			if key == nil {
				break
			}
			tags[string(key)] = string(val)
		}

		for {
			key, val, err := dec.NextField()
			if err != nil {
				svr.log.Warnf("field error: %s", err.Error())
				return
			}
			if key == nil {
				break
			}
			fields[string(key)] = val.Interface()
		}

		ts, err := dec.Time(precision, time.Time{})
		if err != nil {
			svr.log.Warnf("time error: %s", err.Error())
			return
		}
		if ts.IsZero() {
			svr.log.Warnf("timestamp error: %s", err.Error())
			return
		}

		err = msg.WriteLineProtocol(svr.db, dbName, measurement, fields, tags, ts)
		if err != nil {
			svr.log.Warnf("lineprotocol fail: %s", err.Error())
		}
	}
}
