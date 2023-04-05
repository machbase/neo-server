package mqtt

import (
	"github.com/machbase/neo-server/mods/service/mqttsvr/mqtt/packet4"
	"github.com/machbase/neo-server/mods/service/mqttsvr/mqtt/packet5"
)

func (p *peer) RecvUnsubscribe5(msg *packet5.Unsubscribe) {
	if p.UpdateLastPacketId(msg.PacketID) {
		reqs := make([]UnsubReq, 0)
		for _, n := range msg.Topics {
			reqs = append(reqs, UnsubReq{
				TopicName: n,
			})
		}
		p.handleUnsubscribe(reqs)
	} else {
		p.log.Debugf("Duplicated mid:%d", msg.PacketID)
	}

	unsuback := &packet5.Unsuback{}
	unsuback.PacketID = msg.PacketID
	cp := packet5.NewControlPacket(packet5.UNSUBACK)
	cp.Content = unsuback
	p.send(cp)
}

func (p *peer) RecvUnsubscribe4(msg *packet4.UnsubscribePacket) {
	if p.UpdateLastPacketId(msg.MessageID) {
		reqs := make([]UnsubReq, 0)
		for _, n := range msg.Topics {
			reqs = append(reqs, UnsubReq{
				TopicName: n,
			})
		}
		p.handleUnsubscribe(reqs)
	} else {
		p.log.Debugf("Duplicated mid:%d", msg.MessageID)
	}

	unsuback := packet4.NewControlPacket(packet4.UNSUBACK).(*packet4.UnsubackPacket)
	unsuback.MessageID = msg.MessageID
	p.send(unsuback)
}

type UnsubReq struct {
	TopicName string
}

func (p *peer) handleUnsubscribe(req []UnsubReq) {
}
