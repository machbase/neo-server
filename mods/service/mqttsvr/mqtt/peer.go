package mqtt

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	logging "github.com/machbase/neo-logging"
	"github.com/machbase/neo-server/mods/service/mqttsvr/mqtt/packet4"
	"github.com/machbase/neo-server/mods/service/mqttsvr/mqtt/packet5"
	"github.com/machbase/neo-server/mods/util/glob"
)

type Peer interface {
	Id() string

	HasAuth() bool

	ClientId() string // mqtt client-identifier
	Username() string
	X509CommonName() string // common name from x.509 certificate
	RemoteAddrString() string

	GetLog() logging.Log
	LogLevel() logging.Level
	SetLogLevel(level logging.Level)

	SetMaxMessageSizeLimit(limit int)

	Start()
	Close()

	SetValue(string, string)
	Value(string) (string, bool)

	Stat(stat *PeerStat)

	Publish(topic string, QoS byte, payload []byte)
}

type AuthCode int

const (
	// auth success
	AuthSuccess AuthCode = iota
	// Server unavailable
	AuthFail
	// Bad User Name or Password
	AuthError
	//  Not authorized
	AuthDenied
)

type PeerStat struct {
	CreateTime     time.Time
	CertCommonName string
	ReadCount      uint64
	ReadBytes      uint64
	WriteCount     uint64
	WriteBytes     uint64
}

type peer struct {
	// bytes alignment in the struct is important
	// https://github.com/machbase/neo/issues/22
	// https://pkg.go.dev/sync/atomic#pkg-note-BUG
	netstat peerNetStat

	name string
	log  logging.Log

	conn     Connection
	server   Server
	delegate ServerDelegate

	hasAuth  bool
	clientId string
	username string

	x509CommonName string

	mutex           sync.Mutex
	nextMessageId   uint32
	keepAlive       uint16
	protocolVersion byte
	lastPacketId    uint16

	maximumQOS        byte
	topicAliasMaximum uint16
	serverKeepAlive   uint16

	cretime time.Time

	maxMessageSizeLimit int

	topicAliases     map[uint16]string
	allowedPublish   []string
	allowedSubscribe []string

	values map[string]string
}

type peerNetStat struct {
	readCount  uint64
	readBytes  uint64
	writeCount uint64
	writeBytes uint64
}

func NewPeer(svr Server, conn Connection) Peer {
	var commonName = ""
	var peerLogger logging.Log

	if conn.IsSecure() {
		commonName = conn.CommonName()
	}
	addrId := conn.RemoteAddrString()

	if len(commonName) > 0 {
		peerLogger = logging.GetLog(fmt.Sprintf("%s %s", commonName, strings.Repeat("-", 12)))
	} else {
		peerLogger = logging.GetLog(addrId)
	}
	//peerLogger.SetLevel(logging.LevelAll)

	if conn != nil && conn.IsSecure() {
		commonName = conn.CommonName()
	}

	p := &peer{
		name:              addrId,
		server:            svr,
		delegate:          svr.Delegate(),
		conn:              conn,
		log:               peerLogger,
		cretime:           time.Now(),
		nextMessageId:     1,
		keepAlive:         40,
		maximumQOS:        1,
		topicAliasMaximum: 15,
		serverKeepAlive:   70,
		x509CommonName:    commonName,
		values:            make(map[string]string),
	}
	return p
}

func (p *peer) Start() {
	p.mutex.Lock()
	if p.conn == nil {
		p.mutex.Unlock()
		return
	}

	p.conn.SetReadDeadline(time.Duration(30) * time.Second)

	ver, pkt, nlen, err := accept(p.conn.Reader())
	if err != nil {
		p.server.LogReject(p.conn.RemoteAddrString(), err)
		p.conn.Close()
		p.conn = nil
		p.mutex.Unlock()
		return
	}
	p.incRead(uint64(nlen))
	p.mutex.Unlock()

	certHash := p.conn.RemoteCertHash()
	if len(certHash) > 8 {
		p.log.Infof("Sess CREATE %s cert-hash:%s", p.conn.RemoteAddrString(), certHash[0:8])
	} else {
		p.log.Infof("Sess CREATE")
	}

	p.protocolVersion = ver

	p.server.RegisterPeer(p)

	p.recv(pkt)

	go p.loopReader()
}

func (p *peer) Close() {
	p.mutex.Lock()
	if p.conn == nil { // already closed
		p.mutex.Unlock()
		return
	}
	p.conn.Close()
	p.conn = nil
	p.mutex.Unlock()

	p.server.UnregisterPeer(p)

	age := time.Since(p.cretime).Round(time.Second)

	p.log.Infof("Sess DESTROY age:%s", age)
}

func (p *peer) RemoteAddrString() string {
	return p.conn.RemoteAddrString()
}

func (p *peer) GetLog() logging.Log {
	return p.log
}

func (p *peer) SetLogLevel(level logging.Level) {
	p.log.SetLevel(level)
}

func (p *peer) LogLevel() logging.Level {
	return p.log.Level()

}

func (p *peer) SetMaxMessageSizeLimit(limit int) {
	p.maxMessageSizeLimit = limit
}

func (p *peer) Id() string {
	return p.name
}

func (p *peer) Username() string {
	return p.username
}

func (p *peer) ClientId() string {
	return p.clientId
}

func (p *peer) X509CommonName() string {
	return p.x509CommonName
}

func (p *peer) HasAuth() bool {
	return p.hasAuth
}

func (p *peer) SetClientId(clientId string) {
	p.clientId = clientId
	if len(p.clientId) > 0 {
		p.log = logging.GetLog(fmt.Sprintf("%s %s", p.x509CommonName, p.clientId))
	}
}

func (p *peer) SetAuth(clientId, username string, passAuth bool) {
	p.hasAuth = passAuth
	p.clientId = clientId
	p.username = username
}

func (p *peer) AllowPublish(topicPatterns ...string) {
	p.allowedPublish = append(p.allowedPublish, topicPatterns...)
}

func (p *peer) AllowSubscribe(topicPatterns ...string) {
	p.allowedSubscribe = append(p.allowedSubscribe, topicPatterns...)
}

func (p *peer) CanPublish(topicname string) bool {
	for _, p := range p.allowedPublish {
		if flag, _ := glob.Match(p, topicname); flag {
			return true
		}
	}
	return false
}

func (p *peer) CanSubscribe(topicname string) bool {
	for _, p := range p.allowedSubscribe {
		if flag, _ := glob.Match(p, topicname); flag {
			return true
		}
	}
	return false
}

func (p *peer) SetTopicAlias(alias uint16, topicname string) {
	if p.topicAliases == nil {
		p.topicAliases = make(map[uint16]string)
	}
	p.topicAliases[alias] = topicname
}

func (p *peer) GetTopicAlias(alias uint16) (string, bool) {
	if p.topicAliases == nil {
		return "", false
	}
	v, ok := p.topicAliases[alias]
	return v, ok
}

func (p *peer) NextMessageId() uint16 {
	var mid uint16
	n := atomic.AddUint32(&p.nextMessageId, 1)
	if n > math.MaxUint16 {
		mid = uint16(n % math.MaxUint16)
	} else {
		mid = uint16(n)
	}
	return mid
}

// returns
//
//	false: if the packetId has been processed
//	true: the packet has arrived first time, need to process
func (p *peer) UpdateLastPacketId(packetId uint16) bool {
	if p.lastPacketId == packetId {
		return false
	}

	p.lastPacketId = packetId
	return true
}

func (p *peer) SetKeepAlive(keepalive uint16) {
	p.keepAlive = keepalive
}

func (p *peer) KeepAlive() uint16 {
	return p.keepAlive
}

func (p *peer) SetProtocolVersion(ver byte) {
	p.protocolVersion = ver
}

func (p *peer) ProtocolVersion() byte {
	return p.protocolVersion
}

func (p *peer) loopReader() {
	var pkt any
	var err error
	var nlen int
	for {
		pkt, nlen, err = p.readPacket()
		if pkt != nil {
			p.incRead(uint64(nlen))
		}
		if err != nil {
			timeout := false
			byRemotePeer := false
			if ne, ok := err.(net.Error); ok {
				if ne.Timeout() {
					timeout = true
				} else if ne.Temporary() {
					continue
				}
			}
			if errors.Is(err, syscall.ECONNRESET) {
				byRemotePeer = true
			} else if errors.Is(err, io.EOF) {
				byRemotePeer = true
			}
			// create pseudo Disconnect event
			p.handleDisconnect(byRemotePeer, timeout)
			p.log.Debugf("---- loopReader exit by %s", err)
			p.Close()
			return
		}
		if pkt != nil {
			p.recv(pkt)
		}
	}
}

func (p *peer) readPacket() (any, int, error) {
	p.mutex.Lock()
	if p.conn == nil {
		p.mutex.Unlock()
		return nil, 0, errors.New("closed connection")
	}
	if p.keepAlive > 0 {
		readDeadline := (p.keepAlive * 3) / 2
		p.conn.SetReadDeadline(time.Duration(readDeadline) * time.Second) // keepalive * 1.5
	} else {
		p.conn.SetReadDeadline(0)
	}
	reader := p.conn.Reader()
	p.mutex.Unlock()

	if p.protocolVersion == 5 {
		return packet5.ReadPacket(reader, p.maxMessageSizeLimit)
	} else {
		return packet4.ReadPacket(reader, p.maxMessageSizeLimit)
	}
}

// pkt should be *packets.ControlPacket or packet4.ControlPacket
func (p *peer) writePacket(pkt any) (int64, error) {
	p.mutex.Lock()
	if p.conn == nil {
		p.mutex.Unlock()
		return 0, errors.New("closed connection")
	}
	writer := p.conn.Writer()
	//writer := bufio.NewWriter(p.conn.Writer())
	p.mutex.Unlock()

	var nlen int64
	var err error

	if msg, ok := pkt.(*packet5.ControlPacket); ok {
		nlen, err = msg.WriteTo(writer)
	} else if msg, ok := pkt.(packet4.ControlPacket); ok {
		nlen, err = msg.Write(writer)
	} else {
		nlen, err = 0, fmt.Errorf("unknown writing packet %v", pkt)
	}

	if err == nil {
		p.incWrite(uint64(nlen))
	}

	// if err == nil {
	// 	writer.Flush()
	// }

	return nlen, err
}

func (p *peer) writePacketAsync(pkt any, cb func(int64, error)) {
	go func() {
		nlen, err := p.writePacket(pkt)
		if cb != nil {
			cb(nlen, err)
		}
	}()
}

// pkt: *packets.ControlPacket or packet4.ControlPacket
func (p *peer) recv(pkt any) {
	if p.protocolVersion == 5 {
		switch msg := pkt.(*packet5.ControlPacket).Content.(type) {
		case *packet5.Connect:
			p.SetClientId(msg.ClientID)
			p.log.Debugf("Recv CONNECT     v:%d keepAlive:%d clean:%t", msg.ProtocolVersion, msg.KeepAlive, msg.CleanStart)
			p.RecvConnect5(msg)
		case *packet5.Disconnect:
			p.log.Debugf("Recv DISCONNECT")
			p.RecvDisconnect5(msg)
		case *packet5.Publish:
			p.log.Debugf("Recv PUBLISH     mid:%d d:%t q:%d r:%t topic:%s len:%d", msg.PacketID, msg.Duplicate, msg.QoS, msg.Retain, msg.Topic, len(msg.Payload))
			p.RecvPublish5(msg)
		case *packet5.Puback:
			p.log.Debugf("Recv PUBACK      mid:%d", msg.PacketID)
			p.RecvPuback5(msg)
		case *packet5.Subscribe:
			p.log.Debugf("Recv SUBSCRIBE   mid:%d %s", msg.PacketID, msg.Subscriptions)
			p.RecvSubscribe5(msg)
		case *packet5.Unsubscribe:
			p.log.Debugf("Recv UNSUBSCRIBE mid:%d %s", msg.PacketID, msg.Topics)
			p.RecvUnsubscribe5(msg)
		case *packet5.Pingreq:
			p.log.Debugf("Recv PINGREQ")
			p.RecvPingreq5(msg)
		default:
			p.log.Warnf("Recv %v", msg)
		}
	} else {
		switch msg := pkt.(packet4.ControlPacket).(type) {
		case *packet4.ConnectPacket:
			p.SetClientId(msg.ClientIdentifier)
			p.log.Debugf("Recv CONNECT     v:%d keepAlive:%d clean:%t", msg.ProtocolVersion, msg.Keepalive, msg.CleanSession)
			p.RecvConnect4(msg)
		case *packet4.DisconnectPacket:
			p.log.Debugf("Recv DISCONNECT")
			p.RecvDisconnect4(msg)
		case *packet4.PublishPacket:
			p.log.Debugf("Recv PUBLISH     mid:%d d:%t q:%d r:%t topic:%s len:%d", msg.MessageID, msg.Dup, msg.Qos, msg.Retain, msg.TopicName, len(msg.Payload))
			p.RecvPublish4(msg)
		case *packet4.PubackPacket:
			p.log.Debugf("Recv PUBACK      mid:%d", msg.MessageID)
			p.RecvPuback4(msg)
		case *packet4.SubscribePacket:
			p.log.Debugf("Recv SUBSCRIBE   mid:%d %s", msg.MessageID, msg.Topics)
			p.RecvSubscribe4(msg)
		case *packet4.UnsubscribePacket:
			p.log.Debugf("Recv UNSUBSCRIBE mid:%d %s", msg.MessageID, msg.Topics)
			p.RecvUnsubscribe4(msg)
		case *packet4.PingreqPacket:
			p.log.Debugf("Recv PINGREQ")
			p.RecvPingreq4(msg)
		default:
			p.log.Warnf("Recv %s", msg.String())
		}
	}
}

// pkt: *packets.ControlPacket or packet4.ControlPacket
func (p *peer) send(pkt any) {
	p.sendCallback(pkt, nil)
}

func (p *peer) sendCallback(pkt any, cb func(nlen int64, err error)) {
	p.writePacketAsync(pkt, func(nlen int64, err error) {
		logStr := ""

		if p.protocolVersion == 5 {
			switch msg := pkt.(*packet5.ControlPacket).Content.(type) {
			case *packet5.Connack:
				logStr = fmt.Sprintf("Send CONNACK     %s", msg.Reason())
			case *packet5.Publish:
				logStr = fmt.Sprintf("Send PUBLISH     mid:%d d:%t q:%d r:%t topic:%s len:%d", msg.PacketID, msg.Duplicate, msg.QoS, msg.Retain, msg.Topic, len(msg.Payload))
			case *packet5.Puback:
				logStr = fmt.Sprintf("Send PUBACK      mid:%d", msg.PacketID)
			case *packet5.Suback:
				logStr = fmt.Sprintf("Send SUBACK      mid:%d codes:%+v", msg.PacketID, msg.Reasons)
			case *packet5.Unsuback:
				logStr = fmt.Sprintf("Send UNSUBACK    mid:%d", msg.PacketID)
			case *packet5.Pingresp:
				logStr = "Send PINGRESP"
			default:
				logStr = fmt.Sprintf("Send %v", msg)
			}
		} else {
			switch msg := pkt.(packet4.ControlPacket).(type) {
			case *packet4.ConnackPacket:
				logStr = fmt.Sprintf("Send CONNACK     %s", packet4.ConnackReturnCodes[msg.ReturnCode])
			case *packet4.PublishPacket:
				logStr = fmt.Sprintf("Send PUBLISH     mid:%d d:%t q:%d r:%t topic:%s len:%d", msg.MessageID, msg.Dup, msg.Qos, msg.Retain, msg.TopicName, len(msg.Payload))
			case *packet4.PubackPacket:
				logStr = fmt.Sprintf("Send PUBACK      mid:%d", msg.MessageID)
			case *packet4.SubackPacket:
				logStr = fmt.Sprintf("Send SUBACK      mid:%d codes:%+v", msg.MessageID, msg.ReturnCodes)
			case *packet4.UnsubackPacket:
				logStr = fmt.Sprintf("Send UNSUBACK    mid:%d", msg.MessageID)
			case *packet4.PingrespPacket:
				logStr = "Send PINGRESP"
			default:
				logStr = fmt.Sprintf("Send %s", msg.String())
			}
		}

		if err != nil {
			p.log.Warnf("%s error:%s", logStr, err)
		} else {
			p.log.Debugf(logStr)
		}

		if cb != nil {
			cb(nlen, err)
		}
	})
}

func (p *peer) egress(sendMsg any) {
	switch msg := sendMsg.(type) {
	case *packet4.PublishPacket:
		p.send(msg)
		//p.Infof("Egrs Publish %s mid:%d d:%t r:%t len:%d", msg.TopicName, msg.MessageID, msg.Dup, msg.Retain, len(msg.Payload))
		if p.log.TraceEnabled() {
			dump := strings.Split(hex.Dump(msg.Payload), "\n")
			for _, line := range dump {
				p.log.Tracef("Dump %s", line)
			}
		}
	case *packet5.Publish:
		p.send(msg)
		//p.Infof("Egrs Publish %s mid:%d d:%t r:%t len:%d", msg.Topic, msg.PacketID, msg.Duplicate, msg.Retain, len(msg.Payload))
		if p.log.TraceEnabled() {
			dump := strings.Split(hex.Dump(msg.Payload), "\n")
			for _, line := range dump {
				p.log.Tracef("Dump %s", line)
			}
		}
	default:
		p.log.Warnf("Egrs XX %+v", msg)
	}
}

func (p *peer) Publish(topic string, QoS byte, payload []byte) {
	begin := time.Now()
	defer p.server.Metrics().SendPubTimer.UpdateSince(begin)

	if p.protocolVersion == 5 {
		pkt := packet5.NewControlPacket(packet5.PUBLISH)
		pub := pkt.Content.(*packet5.Publish)
		pub.Topic = topic
		pub.Payload = payload
		pub.QoS = QoS
		if pub.QoS > 0 {
			pub.PacketID = p.NextMessageId()
		}
		p.egress(pkt)
	} else {
		pub := packet4.NewControlPacket(packet4.PUBLISH).(*packet4.PublishPacket)
		pub.TopicName = topic
		pub.Payload = payload
		pub.Qos = QoS
		if pub.Qos > 0 {
			pub.MessageID = p.NextMessageId()
		}
		p.egress(pub)
	}
}

func (p *peer) incWrite(nbytes uint64) {
	atomic.AddUint64(&p.netstat.writeCount, 1)
	atomic.AddUint64(&p.netstat.writeBytes, nbytes)
}

func (p *peer) incRead(nbytes uint64) {
	atomic.AddUint64(&p.netstat.readCount, 1)
	atomic.AddUint64(&p.netstat.readBytes, nbytes)
}

func (p *peer) Stat(stat *PeerStat) {
	if stat == nil || p == nil {
		return
	}

	stat.CertCommonName = p.x509CommonName
	stat.CreateTime = p.cretime
	stat.ReadBytes = p.netstat.readBytes
	stat.ReadCount = p.netstat.readCount
	stat.WriteBytes = p.netstat.writeBytes
	stat.WriteCount = p.netstat.writeCount
}

func (p *peer) SetValue(key, val string) {
	p.values[key] = val
}

func (p *peer) Value(key string) (string, bool) {
	v, ok := p.values[key]
	return v, ok
}
