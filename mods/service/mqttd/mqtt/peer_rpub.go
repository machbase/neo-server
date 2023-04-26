package mqtt

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/service/mqttd/mqtt/packet4"
	"github.com/machbase/neo-server/mods/service/mqttd/mqtt/packet5"
)

func (p *peer) RecvPublish5(msg *packet5.Publish) {
	var topic string
	topicAlias := msg.Properties.TopicAlias
	if topicAlias != nil && *topicAlias != 0 {
		if len(msg.Topic) == 0 {
			// read pre-defined topic alias
			v, ok := p.GetTopicAlias(*topicAlias)
			if ok {
				topic = v
			} else {
				// TODO:  Error - topic alias does not exist
				topic = ""
			}
		} else {
			// update topic alias as alias-topicname
			topic = msg.Topic
			p.SetTopicAlias(*topicAlias, topic)
		}
	} else {
		topic = msg.Topic
	}

	switch msg.QoS {
	case 0:
		if p.CanPublish(topic) {
			p.handleRecvPublish(msg.Topic, msg.Payload)
		} else {
			p.log.Warnf("peer published to '%s', not allowed", msg.Topic)
			p.Close()
			return
		}
	case 1:
		var reasonCode byte = packet5.PubackSuccess
		if p.UpdateLastPacketId(msg.PacketID) {
			if p.CanPublish(topic) {
				p.handleRecvPublish(topic, msg.Payload)
			} else {
				reasonCode = packet5.PubackNotAuthorized
			}
		} else {
			p.log.Debugf("Duplicated mid:%d d:%t r:%t", msg.PacketID, msg.Duplicate, msg.Retain)
		}
		puback := &packet5.Puback{}
		puback.PacketID = msg.PacketID
		puback.ReasonCode = reasonCode
		cp := packet5.NewControlPacket(packet5.PUBACK)
		cp.Content = puback
		p.send(cp)
	default:
		disconn := &packet5.Disconnect{}
		disconn.ReasonCode = packet5.DisconnectQoSNotSupported // QoS not supported
		cp := packet5.NewControlPacket(packet5.DISCONNECT)
		cp.Content = disconn
		p.send(cp)
		p.Close()
	}
}

func (p *peer) RecvPublish4(msg *packet4.PublishPacket) {
	switch msg.Qos {
	case 0:
		if p.CanPublish(msg.TopicName) {
			p.handleRecvPublish(msg.TopicName, msg.Payload)
		} else {
			p.log.Warnf("peer published to '%s', not allowed", msg.TopicName)
			p.Close()
			return
		}
	case 1:
		if p.UpdateLastPacketId(msg.MessageID) {
			if p.CanPublish(msg.TopicName) {
				p.handleRecvPublish(msg.TopicName, msg.Payload)
			} else {
				p.log.Warnf("peer published to '%s', not allowed", msg.TopicName)
				p.Close()
				return
			}
		} else {
			p.log.Debugf("Duplicated mid:%d d:%t r:%t", msg.MessageID, msg.Dup, msg.Retain)
		}
		puback := packet4.NewControlPacket(packet4.PUBACK).(*packet4.PubackPacket)
		puback.MessageID = msg.MessageID
		p.send(puback)
	default:
		// unsupported QoS
		disconn := packet4.NewControlPacket(packet4.DISCONNECT).(*packet4.DisconnectPacket)
		p.send(disconn)
		p.Close()
	}
}

func (p *peer) handleRecvPublish(topic string, payload []byte) {
	begin := time.Now()
	defer p.server.Metrics().RecvPubTimer.UpdateSince(begin)

	err := p.delegate.OnMessage(&EvtMessage{
		PeerId:     p.Id(),
		CommonName: p.x509CommonName,
		ClientId:   p.clientId,
		Topic:      topic,
		Raw:        payload,
	})

	logLevel := logging.LevelTrace
	if err != nil {
		logLevel = logging.LevelWarn
	}

	if p.log.LogEnabled(logLevel) {
		msg := ""
		if err != nil {
			msg = fmt.Sprintf("error: %s", err.Error())
		}
		p.log.Logf(logLevel, "Dump topic:%s len:%d %s", topic, len(payload), msg)
		lines := strings.Split(hex.Dump(payload), "\n")
		for _, l := range lines {
			if len(l) > 0 {
				p.log.Logf(logLevel, "Dump %s", l)
			}
		}
	}
}

func (p *peer) RecvPuback5(msg *packet5.Puback) {
	p.handleRecvPuback(msg.PacketID)
}

func (p *peer) RecvPuback4(msg *packet4.PubackPacket) {
	p.handleRecvPuback(msg.MessageID)
}

func (p *peer) handleRecvPuback(mid uint16) {
}
