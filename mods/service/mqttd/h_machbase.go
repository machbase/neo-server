package mqttd

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/service/mqttd/mqtt"
	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/transcoder"
	"github.com/machbase/neo-server/mods/util"
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
	} else if strings.HasPrefix(topic, "tql/") && svr.tqlLoader != nil {
		return svr.handleTql(peer, topic, evt.Raw)
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

	var appenderSet []spi.Appender
	var appender spi.Appender
	var peerId = peer.Id()

	val, exists := svr.appenders.Get(peerId)
	if exists {
		appenderSet = val.([]spi.Appender)
		for _, a := range appenderSet {
			if a.TableName() == wp.Table {
				appender = a
				break
			}
		}
	}
	if appender == nil {
		appender, err = svr.db.Appender(wp.Table)
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
	codecOpts := []codec.Option{
		codec.InputStream(instream),
		codec.Timeformat("ns"),
		codec.TimeLocation(time.UTC),
		codec.Table(wp.Table),
		codec.Columns(cols.Names(), cols.Types()),
		codec.Delimiter(","),
		codec.Heading(false),
	}

	if len(wp.Transform) > 0 {
		opts := []transcoder.Option{}
		if exepath, err := os.Executable(); err == nil {
			opts = append(opts, transcoder.OptionPath(filepath.Dir(exepath)))
		}
		opts = append(opts, transcoder.OptionPname("mqtt"))
		trans := transcoder.New(wp.Transform, opts...)
		codecOpts = append(codecOpts, codec.Transcoder(trans))
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

	tql, err := script.Parse(bytes.NewBuffer(payload), params, io.Discard, false)
	if err != nil {
		svr.log.Error("tql parse fail", path, err.Error())
		return nil
	}

	if err := tql.Execute(context.TODO(), svr.db); err != nil {
		svr.log.Error("tql execute fail", path, err.Error())
		return nil
	}
	return nil
}
