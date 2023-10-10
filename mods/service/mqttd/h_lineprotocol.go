package mqttd

import (
	"bytes"
	"context"
	"strings"
	"time"

	"github.com/influxdata/line-protocol/v2/lineprotocol"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/service/mqttd/mqtt"
)

func (svr *mqttd) onLineprotocol(evt *mqtt.EvtMessage, prefix string) {
	dbName := strings.TrimPrefix(evt.Topic, prefix+"/")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var desc *do.TableDescription
	if desc0, err := do.Describe(ctx, svr.dbConn, dbName, false); err != nil {
		svr.log.Warnf("column error: %s", err.Error())
		return
	} else {
		desc = desc0.(*do.TableDescription)
	}

	precision := lineprotocol.Nanosecond

	dec := lineprotocol.NewDecoder(bytes.NewBuffer(evt.Raw))
	if dec == nil {
		svr.log.Warnf("lineprotocol decoder fail")
		return
	}
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
			tags[strings.ToUpper(string(key))] = string(val)
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

		result := do.WriteLineProtocol(ctx, svr.dbConn, dbName, desc.Columns, measurement, fields, tags, ts)
		if result.Err() != nil {
			svr.log.Warnf("lineprotocol fail: %s", result.Err().Error())
		}
	}
}
