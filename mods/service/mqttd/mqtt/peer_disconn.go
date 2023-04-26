package mqtt

import (
	"github.com/machbase/neo-server/mods/service/mqttd/mqtt/packet4"
	"github.com/machbase/neo-server/mods/service/mqttd/mqtt/packet5"
)

func (p *peer) RecvDisconnect5(msg *packet5.Disconnect) {
	p.Close()
	//// do not call handleDisconnect here, otherwise it produces redundance DC events
	//// this will be called from peer.loopReader() by io.EOF
	//p.handleDisconnect(true, false)
}

func (p *peer) RecvDisconnect4(msg *packet4.DisconnectPacket) {
	p.Close()
	//// do not call handleDisconnect here, otherwise it produces redundance DC events
	//// this will be called from peer.loopReader() by io.EOF
	//p.handleDisconnect(true, false)
}

func (p *peer) handleDisconnect(byRemotePeer bool, keepAliveTimeout bool) {
	p.delegate.OnDisconnect(&EvtDisconnect{
		PeerId:             p.Id(),
		CommonName:         p.x509CommonName,
		ClientId:           p.clientId,
		ByRemotePeer:       byRemotePeer,
		ByKeepAliveTimeout: keepAliveTimeout,
	})
}
