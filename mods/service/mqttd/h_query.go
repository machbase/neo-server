package mqttd

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"time"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/util"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

func (s *mqttd) handleQuery(cl *mqtt.Client, pk packets.Packet) {
	tick := time.Now()
	req := &msg.QueryRequest{Format: "json", Timeformat: "ns", TimeLocation: "UTC", Precision: -1, Heading: true}
	rsp := &msg.QueryResponse{Reason: "not specified"}
	replyTopic := s.defaultReplyTopic
	defer func() {
		rsp.Elapse = time.Since(tick).String()
		replyPayload := rsp.Content
		if len(replyPayload) == 0 {
			buff, _ := json.Marshal(rsp)
			replyPayload = buff
		}
		qos := pk.FixedHeader.Qos
		packetId := uint16(0)
		if qos > 0 {
			packetId = pk.PacketID
		}
		reply := packets.Packet{
			TopicName:       replyTopic,
			Origin:          cl.ID,
			Payload:         replyPayload,
			ProtocolVersion: cl.Properties.ProtocolVersion,
			PacketID:        packetId, // if qos==0, packet id must be 0
			FixedHeader:     packets.FixedHeader{Remaining: len(replyPayload), Type: packets.Publish, Qos: qos},
			Created:         time.Now().Unix(),
			Expiry:          time.Now().Unix() + s.broker.Options.Capabilities.MaximumMessageExpiryInterval,
		}
		code := reply.PublishValidate(s.broker.Options.Capabilities.TopicAliasMaximum)
		if code != packets.CodeSuccess {
			s.log.Error("publish validate", code.String())
			return
		}
		if err := cl.WritePacket(reply); err != nil {
			s.log.Error("write reply", err.Error())
		}
	}()

	err := json.Unmarshal(pk.Payload, req)
	if err != nil {
		rsp.Reason = err.Error()
		return
	}
	if req.ReplyTo != "" {
		replyTopic = req.ReplyTo
	}
	timeLocation, err := util.ParseTimeLocation(req.TimeLocation, time.UTC)
	if err != nil {
		rsp.Reason = err.Error()
		return
	}

	var buffer = &bytes.Buffer{}
	var output spec.OutputStream
	switch req.Compress {
	case "gzip":
		output = &stream.WriterOutputStream{Writer: gzip.NewWriter(buffer)}
	default:
		req.Compress = ""
		output = &stream.WriterOutputStream{Writer: buffer}
	}

	encoder := codec.NewEncoder(req.Format,
		opts.OutputStream(output),
		opts.Timeformat(req.Timeformat),
		opts.Precision(req.Precision),
		opts.Rownum(req.Rownum),
		opts.Heading(req.Heading),
		opts.TimeLocation(timeLocation),
		opts.Delimiter(","),
		opts.BoxStyle("default"),
		opts.BoxSeparateColumns(true),
		opts.BoxDrawBorder(true),
		opts.RowsFlatten(req.RowsFlatten),
		opts.RowsArray(req.RowsArray),
		opts.Transpose(req.Transpose),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := s.db.Connect(ctx, api.WithTrustUser("sys"))
	if err != nil {
		rsp.Reason = err.Error()
		return
	}
	defer conn.Close()

	query := &api.Query{
		Begin: func(q *api.Query) {
			cols := q.Columns()
			rsp.ContentType = encoder.ContentType()
			codec.SetEncoderColumns(encoder, cols)
			encoder.Open()
		},
		Next: func(q *api.Query, nrow int64) bool {
			values, err := q.Columns().MakeBuffer()
			if err != nil {
				s.log.Error("buffer", err.Error())
				return false
			}
			if err = q.Scan(values...); err != nil {
				s.log.Error("scan", err.Error())
				return false
			}
			if err = encoder.AddRow(values); err != nil {
				// report error to client?
				s.log.Error("render", err.Error())
				return false
			}
			return true
		},
		End: func(q *api.Query) {
			if q.IsFetch() {
				encoder.Close()
				rsp.Success, rsp.Reason = true, "success"
				rsp.Content = buffer.Bytes()
			} else {
				rsp.Success, rsp.Reason = true, q.UserMessage()
				rsp.Elapse = time.Since(tick).String()
			}
		},
	}

	if err := query.Execute(ctx, conn, req.SqlText); err != nil {
		s.log.Error("query fail", err.Error())
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
	}
}
