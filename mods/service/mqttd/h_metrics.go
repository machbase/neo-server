package mqttd

import (
	"bytes"
	"context"
	"strings"
	"time"

	"github.com/influxdata/line-protocol/v2/lineprotocol"
	"github.com/machbase/neo-server/api"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

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
	precision := lineprotocol.Nanosecond

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

		ts, err := dec.Time(precision, time.Time{})
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
