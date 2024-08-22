package mqtt2

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"strings"
	"time"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/util"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

type AppenderWrapper struct {
	conn      api.Conn
	appender  api.Appender
	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (s *mqtt2) handleAppend(cl *mqtt.Client, pk packets.Packet) {
	writePath := strings.ToUpper(strings.TrimPrefix(pk.TopicName, "db/append/"))
	wp, err := util.ParseWritePath(writePath)
	if err != nil {
		s.log.Warn(cl.Net.Remote, pk.TopicName, err.Error())
		return
	}

	if wp.Format == "" {
		wp.Format = "json"
	}

	switch wp.Format {
	case "json":
	case "csv":
	default:
		s.log.Warn(cl.Net.Remote, "unsupported format:", wp.Format)
		return
	}
	switch wp.Compress {
	case "": // no compression
	case "-": // no compression
	case "gzip": // gzip compression
	default: // others
		s.log.Warn(cl.Net.Remote, "unsupproted compression:", wp.Compress)
		return
	}

	var appenderSet []*AppenderWrapper
	var appender api.Appender
	var peerId = cl.Net.Remote

	if val, exists := s.appenders.Get(peerId); exists {
		appenderSet = val.([]*AppenderWrapper)
		for _, a := range appenderSet {
			if a.appender.TableName() == wp.Table {
				appender = a.appender
				break
			}
		}
	}

	if appender == nil {
		ctx, ctxCancel := context.WithCancel(context.Background())
		tableNameFields := strings.SplitN(wp.Table, ".", 2)
		tableUser := "SYS"
		if len(tableNameFields) == 2 {
			tableUser = strings.ToUpper(tableNameFields[0])
			wp.Table = strings.ToUpper(tableNameFields[1])
		} else {
			wp.Table = strings.ToUpper(wp.Table)
		}

		if conn, err := s.db.Connect(ctx, api.WithTrustUser(tableUser)); err != nil {
			ctxCancel()
			s.log.Warn(cl.Net.Remote, err.Error())
			return
		} else {
			appender, err = conn.Appender(ctx, wp.Table)
			if err != nil {
				ctxCancel()
				s.log.Warn(cl.Net.Remote, "fail to create appender,", err.Error())
				return
			}
			aw := &AppenderWrapper{
				conn:      conn,
				appender:  appender,
				ctx:       ctx,
				ctxCancel: ctxCancel,
			}
			if len(appenderSet) == 0 {
				appenderSet = []*AppenderWrapper{}
			}
			appenderSet = append(appenderSet, aw)
			s.appenders.Set(peerId, appenderSet)
		}
	}

	var instream spec.InputStream

	if wp.Compress == "gzip" {
		gr, err := gzip.NewReader(bytes.NewBuffer(pk.Payload))
		defer func() {
			if gr != nil {
				err = gr.Close()
				if err != nil {
					s.log.Warn(cl.Net.Remote, "fail to close decompressor,", err.Error())
				}
			}
		}()
		if err != nil {
			s.log.Warn(cl.Net.Remote, "fail to gunzip,", err.Error())
			return
		}
		instream = &stream.ReaderInputStream{Reader: gr}
	} else {
		instream = &stream.ReaderInputStream{Reader: bytes.NewReader(pk.Payload)}
	}

	cols, _ := api.AppenderColumns(appender)
	colNames := cols.Names()
	colTypes := cols.Types()
	if api.AppenderTableType(appender) == api.LogTableType && colNames[0] == "_ARRIVAL_TIME" {
		colNames = colNames[1:]
		colTypes = colTypes[1:]
	}
	codecOpts := []opts.Option{
		opts.InputStream(instream),
		opts.Timeformat("ns"),
		opts.TimeLocation(time.UTC),
		opts.TableName(wp.Table),
		opts.Columns(colNames...),
		opts.ColumnTypes(colTypes...),
		opts.Delimiter(","),
		opts.Heading(false),
	}

	decoder := codec.NewDecoder(wp.Format, codecOpts...)

	recno := 0
	for {
		vals, err := decoder.NextRow()
		if err != nil {
			if err != io.EOF {
				s.log.Warn(cl.Net.Remote, "append", wp.Format, err.Error())
				return
			}
			break
		}
		err = appender.Append(vals...)
		if err != nil {
			s.log.Warn(cl.Net.Remote, "append", wp.Format, err.Error())
			break
		}
		recno++
	}
	s.log.Debug(cl.Net.Remote, "appended", recno, "record(s),", wp.Table)
}
