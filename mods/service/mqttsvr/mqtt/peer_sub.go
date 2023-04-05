package mqtt

import (
	"github.com/machbase/neo-server/mods/service/mqttsvr/mqtt/packet4"
	"github.com/machbase/neo-server/mods/service/mqttsvr/mqtt/packet5"
)

func (p *peer) RecvSubscribe5(msg *packet5.Subscribe) {
	isDup := !p.UpdateLastPacketId(msg.PacketID)
	if isDup {
		p.log.Debugf("Duplicated mid:%d", msg.PacketID)
	}

	codes := p.handleSubscribe(msg.Subscriptions, isDup)

	suback := &packet5.Suback{}
	suback.PacketID = msg.PacketID
	suback.Reasons = codes
	cp := packet5.NewControlPacket(packet5.SUBACK)
	cp.Content = suback
	p.send(cp)
}

func (p *peer) RecvSubscribe4(msg *packet4.SubscribePacket) {
	isDup := !p.UpdateLastPacketId(msg.MessageID)
	if isDup {
		p.log.Debugf("Duplicated mid:%d", msg.MessageID)
	}

	subs := make(map[string]packet5.SubOptions, 0)
	for i, topic := range msg.Topics {
		subs[topic] = packet5.SubOptions{
			QoS: msg.Qoss[i],
		}
	}

	codes := p.handleSubscribe(subs, isDup)

	suback := packet4.NewControlPacket(packet4.SUBACK).(*packet4.SubackPacket)
	suback.MessageID = msg.MessageID
	suback.ReturnCodes = codes
	p.send(suback)
}

type SubReq struct {
	TopicName string
	QoS       byte
}

func (p *peer) handleSubscribe(subs map[string]packet5.SubOptions, isDup bool) []byte {
	codes := make([]byte, 0)
	for topic, op := range subs {
		if p.CanSubscribe(topic) {
			if op.QoS > 1 {
				codes = append(codes, 1)
			} else {
				codes = append(codes, op.QoS)
			}
		} else { // no-permission to subscribe this topics
			if p.ProtocolVersion() == 5 {
				codes = append(codes, packet5.SubackNotauthorized)
			} else {
				codes = append(codes, 0x80)
			}
		}
	}
	return codes
}
