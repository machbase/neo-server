package mqtt

import (
	"time"

	"github.com/machbase/neo-server/mods/service/mqttsvr/mqtt/packet4"
	"github.com/machbase/neo-server/mods/service/mqttsvr/mqtt/packet5"
)

func (p *peer) RecvConnect5(msg *packet5.Connect) {
	connack := &packet5.Connack{}
	connack.SessionPresent = false
	connack.Properties = &packet5.Properties{
		MaximumQOS:        &p.maximumQOS,
		TopicAliasMaximum: &p.topicAliasMaximum,
		ServerKeepAlive:   &p.serverKeepAlive,
	}

	begin := time.Now()
	authCode, result, err := p.delegate.OnConnect(&EvtConnect{
		PeerId:          p.Id(),
		ProtocolVersion: msg.ProtocolVersion,
		CommonName:      p.x509CommonName,
		ClientId:        msg.ClientID,
		Username:        msg.Username,
		Password:        msg.Password,
		KeepAlive:       msg.KeepAlive,
		CleanStart:      msg.CleanStart,
		CertHash:        p.conn.RemoteCertHash(),
	})
	p.server.Metrics().AuthTimer.Update(time.Since(begin))
	if err != nil {
		p.log.Errorf("auth error %s", err.Error())
	}

	p.SetProtocolVersion(msg.ProtocolVersion)
	if authCode == AuthSuccess {
		p.SetAuth(msg.ClientID, msg.Username, true)
		p.SetKeepAlive(msg.KeepAlive)
		if result != nil {
			p.AllowPublish(result.AllowedPublishTopicPatterns...)
			p.AllowSubscribe(result.AllowedSubscribeTopicPatterns...)
		}
	}

	switch authCode {
	case AuthSuccess:
		connack.ReasonCode = 0x00
		p.server.Metrics().AuthSuccessCounter.Inc(1)
	case AuthDenied:
		connack.ReasonCode = 0x87 // Not authorized
		p.server.Metrics().AuthDeniedCounter.Inc(1)
	case AuthFail:
		connack.ReasonCode = 0x88 // Server unavailable
		p.server.Metrics().AuthFailCounter.Inc(1)
	case AuthError:
		connack.ReasonCode = 0x86 // Bad User Name or Password
		p.server.Metrics().AuthErrorCounter.Inc(1)
	default:
		connack.ReasonCode = 0x87 // Not authorized
		p.server.Metrics().AuthDeniedCounter.Inc(1)
	}

	cp := packet5.NewControlPacket(packet5.CONNACK)
	cp.Content = connack
	p.sendCallback(cp, func(nlen int64, err error) {
		if connack.ReasonCode != 0 || err != nil {
			p.Close()
			return
		}
	})
}

func (p *peer) RecvConnect4(msg *packet4.ConnectPacket) {
	connack := packet4.NewControlPacket(packet4.CONNACK).(*packet4.ConnackPacket)
	connack.SessionPresent = false

	begin := time.Now()
	authCode, result, err := p.delegate.OnConnect(&EvtConnect{
		PeerId:          p.Id(),
		ProtocolVersion: msg.ProtocolVersion,
		CommonName:      p.x509CommonName,
		ClientId:        msg.ClientIdentifier,
		Username:        msg.Username,
		Password:        msg.Password,
		KeepAlive:       msg.Keepalive,
		CleanStart:      msg.CleanSession,
		CertHash:        p.conn.RemoteCertHash(),
	})
	p.server.Metrics().AuthTimer.Update(time.Since(begin))
	if err != nil {
		p.log.Errorf("auth error %s", err.Error())
	}

	p.SetProtocolVersion(msg.ProtocolVersion)
	if authCode == AuthSuccess {
		p.SetAuth(msg.ClientIdentifier, msg.Username, true)
		p.SetKeepAlive(msg.Keepalive)
		if result != nil {
			p.AllowPublish(result.AllowedPublishTopicPatterns...)
			p.AllowSubscribe(result.AllowedSubscribeTopicPatterns...)
		}
	}

	switch authCode {
	case AuthSuccess:
		connack.ReturnCode = packet4.Accepted
		p.server.Metrics().AuthSuccessCounter.Inc(1)
	case AuthDenied:
		connack.ReturnCode = packet4.ErrRefusedNotAuthorized
		p.server.Metrics().AuthDeniedCounter.Inc(1)
	case AuthFail:
		connack.ReturnCode = packet4.ErrRefusedServerUnavailable
		p.server.Metrics().AuthFailCounter.Inc(1)
	case AuthError:
		connack.ReturnCode = packet4.ErrRefusedBadUsernameOrPassword
		p.server.Metrics().AuthErrorCounter.Inc(1)
	default:
		connack.ReturnCode = packet4.ErrRefusedNotAuthorized
		p.server.Metrics().AuthDeniedCounter.Inc(1)
	}

	p.sendCallback(connack, func(nlen int64, err error) {
		if connack.ReturnCode != 0 || err != nil {
			p.Close()
			return
		}
	})
}
