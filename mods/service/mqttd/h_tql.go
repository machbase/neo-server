package mqttd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/machbase/neo-server/mods/tql"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

func (s *mqttd) handleTql(cl *mqtt.Client, pk packets.Packet) {
	if s.tqlLoader == nil {
		s.log.Error("tql is not enabled.")
		return
	}

	rawQuery := strings.SplitN(strings.TrimPrefix(pk.TopicName, "db/tql/"), "?", 2)
	if len(rawQuery) == 0 {
		s.log.Warn(cl.Net.Remote, "no tql path", pk.TopicName)
		return
	}
	path := rawQuery[0]
	if !strings.HasSuffix(path, ".tql") {
		s.log.Warn(cl.Net.Remote, "invalid tql path", path)
		return
	}
	var params url.Values
	if len(path) == 2 {
		vs, err := url.ParseQuery(rawQuery[1])
		if err != nil {
			s.log.Warn(cl.Net.Remote, "tql invalid query:", rawQuery[1])
			return
		}
		params = vs
	}

	wr := msg.WriteRequest{}
	dec := json.NewDecoder(bytes.NewBuffer(pk.Payload))
	// ignore json decoder error, the payload json can be non-full-document json.
	dec.Decode(&wr)

	script, err := s.tqlLoader.Load(path)
	if err != nil {
		s.log.Warn(cl.Net.Remote, "tql load fail", path, err.Error())
		return
	}

	buf := &bytes.Buffer{}
	task := tql.NewTaskContext(context.TODO())
	task.SetDatabase(s.db)
	task.SetInputReader(bytes.NewBuffer(pk.Payload))
	task.SetOutputWriter(buf)
	task.SetParams(params)
	if err := task.CompileScript(script); err != nil {
		s.log.Error("tql parse fail", path, err.Error())
		return
	}

	result := task.Execute()
	if result == nil {
		s.log.Error("tql execute error", path)
	} else if result.Err != nil {
		s.log.Error("tql execute fail", path, result.Err.Error())
	}
	if wr.ReplyTo != "" {
		replyPayload := buf.Bytes()
		reply := packets.Packet{
			TopicName:       wr.ReplyTo,
			Origin:          cl.ID,
			Payload:         replyPayload,
			ProtocolVersion: cl.Properties.ProtocolVersion,
			PacketID:        0, // if qos==0, packet id must be 0
			FixedHeader:     packets.FixedHeader{Remaining: len(replyPayload), Type: packets.Publish, Qos: byte(0)},
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
	}
}
