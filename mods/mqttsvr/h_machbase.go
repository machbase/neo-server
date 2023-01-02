package mqttsvr

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/machbase/cemlib/mqtt"
	mach "github.com/machbase/dbms-mach-go"
	"github.com/machbase/dbms-mach-go/server/msg"
	"github.com/tidwall/gjson"
)

func (svr *Server) onMachbase(evt *mqtt.EvtMessage, prefix string) error {
	tick := time.Now()
	topic := evt.Topic
	topic = strings.TrimPrefix(topic, prefix+"/")

	reply := func(msg any) {
		peer, ok := svr.mqttd.GetPeer(evt.PeerId)
		if ok {
			buff, err := json.Marshal(msg)
			if err != nil {
				return
			}
			peer.Publish(prefix+"/reply", 1, buff)
		}
	}
	if topic == "query" {
		////////////////////////
		// query
		req := &msg.QueryRequest{}
		rsp := &msg.QueryResponse{Reason: "not specified"}
		err := json.Unmarshal(evt.Raw, req)
		if err != nil {
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			reply(rsp)
			return nil
		}
		msg.Query(svr.db, req, rsp)
		rsp.Elapse = time.Since(tick).String()
		reply(rsp)
	} else if strings.HasPrefix(topic, "write") {
		////////////////////////
		// write
		req := &msg.WriteRequest{}
		rsp := &msg.WriteResponse{Reason: "not specified"}
		err := json.Unmarshal(evt.Raw, req)
		if err != nil {
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			reply(rsp)
			return nil
		}
		if len(req.Table) == 0 {
			req.Table = strings.TrimPrefix(topic, "write/")
		}

		if len(req.Table) == 0 {
			rsp.Reason = "table is not specified"
			rsp.Elapse = time.Since(tick).String()
			reply(rsp)
			return nil
		}
		msg.Write(svr.db, req, rsp)
		rsp.Elapse = time.Since(tick).String()
		reply(rsp)
	} else if strings.HasPrefix(topic, "append/") {
		////////////////////////
		// append
		table := strings.ToUpper(strings.TrimPrefix(topic, "append/"))
		if len(table) == 0 {
			return nil
		}

		var err error
		var appenderSet []*mach.Appender
		var appender *mach.Appender

		val, exists := svr.appenders.Get(evt.PeerId)
		if exists {
			appenderSet = val.([]*mach.Appender)
			for _, a := range appenderSet {
				if a.Table() == table {
					appender = a
					break
				}
			}
		}
		if appender == nil {
			appender, err = svr.db.Appender(table)
			if err != nil {
				svr.log.Errorf("fail to create appender, %s", err.Error())
				return nil
			}
			if len(appenderSet) == 0 {
				appenderSet = []*mach.Appender{}
			}
			appenderSet = append(appenderSet, appender)
			svr.appenders.Set(evt.PeerId, appenderSet)
		}

		result := gjson.ParseBytes(evt.Raw)

		head := result.Get("0")
		if head.IsArray() {
			// if payload contains multiple tuples
			result.ForEach(func(key, value gjson.Result) bool {
				vals := []any{}
				value.ForEach(func(key, value gjson.Result) bool {
					vals = append(vals, value.Value())
					return true
				})
				err = appender.Append(vals...)
				if err != nil {
					svr.log.Warnf("append fail %s", err.Error())
					return false
				}
				return true
			})
		} else {
			// a single tuple
			vals := []any{}
			result.ForEach(func(key, value gjson.Result) bool {
				vals = append(vals, value.Value())
				return true
			})
			err = appender.Append(vals...)
			if err != nil {
				svr.log.Warnf("append fail %s", err.Error())
			}
		}
	}
	return nil
}
