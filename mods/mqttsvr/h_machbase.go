package mqttsvr

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/machbase/cemlib/mqtt"
	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/mods/msg"
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
			cols := appender.Columns()
			var err error
			result.ForEach(func(key, value gjson.Result) bool {
				fields := value.Array()
				vals, err := convAppendColumns(fields, cols, appender.TableType())
				if err != nil {
					return false
				}
				err = appender.Append(vals...)
				if err != nil {
					svr.log.Warnf("append fail %s %d %s [%+v]", table, appender.TableType(), err.Error(), vals)
					return false
				}
				return true
			})
			return err
		} else {
			// a single tuple
			fields := result.Array()
			cols := appender.Columns()
			vals, err := convAppendColumns(fields, cols, appender.TableType())
			if err != nil {
				return err
			}
			err = appender.Append(vals...)
			if err != nil {
				svr.log.Warnf("append fail %s %d %s [%+v]", table, appender.TableType(), err.Error(), vals)
				return err
			}
			return nil
		}
	}
	return nil
}

func convAppendColumns(fields []gjson.Result, cols []*mach.Column, tableType mach.TableType) ([]any, error) {
	fieldsOffset := 0
	colsNum := len(cols)
	vals := []any{}
	switch tableType {
	case mach.LogTableType:
		if colsNum == len(fields) {
			// num of columns is matched
		} else if colsNum+1 == len(fields) {
			vals = append(vals, fields[0].Int()) // timestamp included
			fieldsOffset = 1
		} else {
			return nil, fmt.Errorf("append fail, received fields not matched columns(%d)", colsNum)
		}

	default:
		if colsNum == len(fields) {
			// num of columns is matched
		} else {
			return nil, fmt.Errorf("append fail, received fields not matched columns(%d)", colsNum)
		}
	}

	for i, v := range fields[fieldsOffset:] {
		switch cols[i].Type {
		case mach.ColumnTypeNameInt16:
			vals = append(vals, v.Int())
		case mach.ColumnTypeNameInt32:
			vals = append(vals, v.Int())
		case mach.ColumnTypeNameInt64:
			vals = append(vals, v.Int())
		case mach.ColumnTypeNameString:
			vals = append(vals, v.Str)
		case mach.ColumnTypeNameDatetime:
			vals = append(vals, v.Int())
		case mach.ColumnTypeNameFloat:
			vals = append(vals, v.Float())
		case mach.ColumnTypeNameDouble:
			vals = append(vals, v.Float())
		case mach.ColumnTypeNameIPv4:
			vals = append(vals, v.Str)
		case mach.ColumnTypeNameIPv6:
			vals = append(vals, v.Str)
		case mach.ColumnTypeNameBinary:
			return nil, errors.New("append fail, binary column is not supproted via JSON payload")
		}
	}

	return vals, nil
}
