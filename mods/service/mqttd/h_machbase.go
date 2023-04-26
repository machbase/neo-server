package mqttd

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/service/mqttd/mqtt"
	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/transcoder"
	spi "github.com/machbase/neo-spi"
)

func (svr *mqttd) onMachbase(evt *mqtt.EvtMessage, prefix string) error {
	topic := evt.Topic
	topic = strings.TrimPrefix(topic, prefix+"/")
	peer, ok := svr.mqttd.GetPeer(evt.PeerId)

	if topic == "query" {
		reply := func(msg any) {
			if ok {
				buff, err := json.Marshal(msg)
				if err != nil {
					return
				}
				peer.Publish(prefix+"/reply", 1, buff)
			}
		}
		return svr.handleQuery(peer, evt.Raw, reply)
	} else if strings.HasPrefix(topic, "write") {
		return svr.handleWrite(peer, topic, evt.Raw)
	} else if strings.HasPrefix(topic, "append/") {
		return svr.handleAppend(peer, topic, evt.Raw)
	} else {
		peer.GetLog().Warnf("---- invalid topic '%s'", evt.Topic)
	}
	return nil
}

func (svr *mqttd) handleQuery(peer mqtt.Peer, payload []byte, reply func(msg any)) error {
	tick := time.Now()
	req := &msg.QueryRequest{}
	rsp := &msg.QueryResponse{Reason: "not specified"}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
		reply(rsp)
	}()

	err := json.Unmarshal(payload, req)
	if err != nil {
		rsp.Reason = err.Error()
		return nil
	}
	Query(svr.db, req, rsp)
	return nil
}

func (svr *mqttd) handleWrite(peer mqtt.Peer, topic string, payload []byte) error {
	req := &msg.WriteRequest{}
	rsp := &msg.WriteResponse{Reason: "not specified"}

	err := json.Unmarshal(payload, req)
	if err != nil {
		rsp.Reason = err.Error()
		return nil
	}
	if len(req.Table) == 0 {
		req.Table = strings.TrimPrefix(topic, "write/")
	}

	if len(req.Table) == 0 {
		rsp.Reason = "table is not specified"
		return nil
	}
	Write(svr.db, req, rsp)
	return nil
}

func (svr *mqttd) handleAppend(peer mqtt.Peer, topic string, payload []byte) error {
	peerLog := peer.GetLog()

	table := strings.ToUpper(strings.TrimPrefix(topic, "append/"))
	if len(table) == 0 {
		return nil
	}

	toks := strings.Split(table, ":")
	var compress = ""
	var transname = ""

	var format = "json"
	if len(toks) >= 1 {
		table = toks[0]
	}
	if len(toks) >= 2 {
		format = strings.ToLower(toks[1])
	}
	if len(toks) == 3 {
		compress = strings.ToLower(toks[2])
	} else if len(toks) == 4 {
		transname = strings.ToLower(toks[2])
		compress = strings.ToLower(toks[3])
	}

	switch format {
	case "json":
	case "csv":
	default:
		peerLog.Warnf("---- unsupported format '%s'", format)
		return nil
	}
	switch compress {
	case "": // no compression
	case "-": // no compression
	case "gzip": // gzip compression
	default: // others
		peerLog.Warnf("---- unsupproted compression '%s", compress)
		return nil
	}

	var err error
	var appenderSet []spi.Appender
	var appender spi.Appender
	var peerId = peer.Id()

	val, exists := svr.appenders.Get(peerId)
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
			peerLog.Errorf("---- fail to create appender, %s", err.Error())
			return nil
		}
		if len(appenderSet) == 0 {
			appenderSet = []spi.Appender{}
		}
		appenderSet = append(appenderSet, appender)
		svr.appenders.Set(peerId, appenderSet)
	}

	var instream spi.InputStream

	if compress == "gzip" {
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
	builder := codec.NewDecoderBuilder(format).
		SetInputStream(instream).
		SetColumns(cols).
		SetTimeFormat("ns").
		SetTimeLocation(time.UTC).
		SetCsvDelimieter(",").
		SetCsvHeading(false)

	if len(transname) > 0 {
		opts := []transcoder.Option{}
		if exepath, err := os.Executable(); err == nil {
			opts = append(opts, transcoder.OptionPath(filepath.Dir(exepath)))
		}
		trans := transcoder.New(transname, opts...)
		builder.SetTranscoder(trans)
	}

	decoder := builder.Build()

	recno := 0
	for {
		vals, err := decoder.NextRow()
		if err != nil {
			if err != io.EOF {
				peerLog.Warnf("---- append %s, %s", format, err.Error())
				return nil
			}
			break
		}
		err = appender.Append(vals...)
		if err != nil {
			peerLog.Errorf("---- append %s, %s", format, err.Error())
			break
		}
		recno++
	}
	peerLog.Debugf("---- appended %d record(s), %s", recno, topic)
	return nil
}
