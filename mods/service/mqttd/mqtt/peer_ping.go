package mqtt

import (
	"github.com/machbase/neo-server/mods/service/mqttd/mqtt/packet4"
	"github.com/machbase/neo-server/mods/service/mqttd/mqtt/packet5"
)

func (p *peer) RecvPingreq5(msg *packet5.Pingreq) {
	pong := &packet5.Pingresp{}
	cp := packet5.NewControlPacket(packet5.PINGRESP)
	cp.Content = pong
	p.send(cp)
}

func (p *peer) RecvPingreq4(msg *packet4.PingreqPacket) {
	pong := packet4.NewControlPacket(packet4.PINGRESP).(*packet4.PingrespPacket)
	p.send(pong)
}
