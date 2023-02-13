package mqttsvr

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/machbase/cemlib/mqtt"
	"github.com/machbase/neo-server/mods/msg"
	spi "github.com/machbase/neo-spi"
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
		var appenderSet []spi.Appender
		var appender spi.Appender

		val, exists := svr.appenders.Get(evt.PeerId)
		if exists {
			appenderSet = val.([]spi.Appender)
			for _, a := range appenderSet {
				if a.TableName() == table {
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
				appenderSet = []spi.Appender{}
			}
			appenderSet = append(appenderSet, appender)
			svr.appenders.Set(evt.PeerId, appenderSet)
		}

		result := gjson.ParseBytes(evt.Raw)

		head := result.Get("0")
		if head.IsArray() {
			// if payload contains multiple tuples
			cols, err := appender.Columns()
			if err != nil {
				svr.log.Errorf("fail to get appender columns, %s", err.Error())
				return nil
			}
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
			cols, err := appender.Columns()
			if err != nil {
				svr.log.Errorf("fail to get appender columns, %s", err.Error())
				return nil
			}
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

func convAppendColumns(fields []gjson.Result, cols spi.Columns, tableType spi.TableType) ([]any, error) {
	fieldsOffset := 0
	colsNum := len(cols)
	vals := []any{}
	switch tableType {
	case spi.LogTableType:
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
		case spi.ColumnTypeNameInt16:
			vals = append(vals, v.Int())
		case spi.ColumnTypeNameInt32:
			vals = append(vals, v.Int())
		case spi.ColumnTypeNameInt64:
			vals = append(vals, v.Int())
		case spi.ColumnTypeNameString:
			vals = append(vals, v.Str)
		case spi.ColumnTypeNameDatetime:
			vals = append(vals, v.Int())
		case spi.ColumnTypeNameFloat:
			vals = append(vals, v.Float())
		case spi.ColumnTypeNameDouble:
			vals = append(vals, v.Float())
		case spi.ColumnTypeNameIPv4:
			vals = append(vals, v.Str)
		case spi.ColumnTypeNameIPv6:
			vals = append(vals, v.Str)
		case spi.ColumnTypeNameBinary:
			return nil, errors.New("append fail, binary column is not supproted via JSON payload")
		}
	}

	return vals, nil
}
