package mqtt

import (
	"fmt"
	"strings"
	"sync"

	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/service/allowance"
	"github.com/machbase/neo-server/mods/service/security"

	cmap "github.com/orcaman/concurrent-map"
)

type Evt interface {
	GetPeerId() string
	GetCommonName() string
	GetClientId() string
}

func (e *EvtConnect) GetPeerId() string     { return e.PeerId }
func (e *EvtConnect) GetCommonName() string { return e.CommonName }
func (e *EvtConnect) GetClientId() string   { return e.ClientId }

func (e *EvtDisconnect) GetPeerId() string     { return e.PeerId }
func (e *EvtDisconnect) GetCommonName() string { return e.CommonName }
func (e *EvtDisconnect) GetClientId() string   { return e.ClientId }

func (e *EvtMessage) GetPeerId() string     { return e.PeerId }
func (e *EvtMessage) GetCommonName() string { return e.CommonName }
func (e *EvtMessage) GetClientId() string   { return e.ClientId }

type EvtConnect struct {
	PeerId          string
	CommonName      string
	ClientId        string
	ProtocolVersion byte
	Username        string
	Password        []byte
	KeepAlive       uint16
	CleanStart      bool
	CertHash        string
}

type ConnectResult struct {
	AllowedPublishTopicPatterns   []string
	AllowedSubscribeTopicPatterns []string
}

type EvtDisconnect struct {
	PeerId             string
	CommonName         string
	ClientId           string
	ByRemotePeer       bool
	ByKeepAliveTimeout bool
}

type EvtMessage struct {
	PeerId     string
	CommonName string
	ClientId   string
	Topic      string
	Raw        []byte
}

type ServerDelegate interface {
	OnConnect(evt *EvtConnect) (AuthCode, *ConnectResult, error)
	OnDisconnect(evt *EvtDisconnect)
	OnMessage(evt *EvtMessage) error
}

type Server interface {
	Start() error
	Stop()

	ListenAddresses() []string

	Listeners() []Listener

	RegisterPeer(p Peer)
	UnregisterPeer(p Peer)

	SetDelegate(d ServerDelegate)
	Delegate() ServerDelegate

	GetPeer(peerId string) (Peer, bool)
	CountPeers() int64
	IteratePeers(cb func(p Peer) bool)

	GetOtpGenerator(key string) (security.Generator, error)

	Metrics() *ServerMetrics
	LogReject(remoteAddr string, cause error)
}

type server struct {
	Server

	conf MqttConfig
	log  logging.Log

	allowance allowance.Allowance

	lsnrs []Listener

	acceptChan chan any
	quitChan   chan any

	closeWait    sync.WaitGroup
	closing      bool
	closingMutex sync.Mutex

	delegate ServerDelegate

	peers cmap.ConcurrentMap // map string(remoteAddress) - Peer

	OtpPrefixes OtpPrefixes
	metrics     *ServerMetrics
}

func NewServer(cfg *MqttConfig, delegate ServerDelegate) Server {
	return &server{
		peers:      cmap.New(),
		conf:       *cfg,
		delegate:   delegate,
		acceptChan: make(chan any, 256),
		quitChan:   make(chan any, 1),
	}
}

func (s *server) Start() error {
	s.log = logging.GetLog("mqtt")
	s.allowance = allowance.NewAllowanceFromConfig(&s.conf.Allowance)
	s.metrics = NewServerMetrics(s)

	// start tcp listeners
	for _, tcpConf := range s.conf.TcpListeners {
		if len(tcpConf.ListenAddress) > 0 {
			lsnr, err := NewTcpListener(&tcpConf, s.acceptChan)
			if err != nil {
				return fmt.Errorf("fail to listen: %s; %s", tcpConf.ListenAddress, err)
			}
			s.lsnrs = append(s.lsnrs, lsnr)
			lsnr.SetAllowance(s.allowance)
			err = lsnr.Start()
			if err != nil {
				return err
			}
		}
	}

	// start unix socket listener
	if len(s.conf.UnixSocketConfig.Path) > 0 {
		lsnr, err := NewUnixSocketListener(&s.conf.UnixSocketConfig, s.acceptChan)
		if err != nil {
			return fmt.Errorf("fail to listen: %s; %s", s.conf.UnixSocketConfig.Path, err)
		}
		s.lsnrs = append(s.lsnrs, lsnr)
		lsnr.SetAllowance(s.allowance)
		err = lsnr.Start()
		if err != nil {
			return err
		}
	}

	//// loop for receiving notification of accept/reject
	s.closeWait.Add(1)
	go func() {
		defer s.closeWait.Done()

		rejectLog := logging.GetLog("mqtt-reject")
		for {
			select {
			case msg := <-s.acceptChan:
				switch cn := msg.(type) {
				case Connection:
					// create and start new peer
					p := NewPeer(s, cn)
					p.SetMaxMessageSizeLimit(s.conf.MaxMessageSizeLimit)
					go p.Start()
					s.metrics.ConnAllowed.Inc(1)

				case RejectConnection:
					if cn.Error() != nil {
						rejectLog.Tracef("reject connect from %s:%d %s, %s", cn.RemoteHost(), cn.RemotePort(), cn.Reason(), cn.Error())
					} else {
						rejectLog.Tracef("reject connect from %s:%d %s", cn.RemoteHost(), cn.RemotePort(), cn.Reason())
					}
					s.metrics.ConnDenied.Inc(1)
				}

			case <-s.quitChan:
				for _, l := range s.lsnrs {
					l.Stop()
				}
				close(s.acceptChan)
				s.CloseAllPeers()
				return
			}
		}
	}()

	return nil
}

func (s *server) Stop() {
	s.closingMutex.Lock()
	if s.closing {
		s.closingMutex.Unlock()
		return
	}
	s.closing = true
	s.closingMutex.Unlock()

	s.StopAllListeners()

	s.quitChan <- "quit"
	s.closeWait.Wait()

	close(s.quitChan)
}

func (s *server) Listeners() []Listener {
	return s.lsnrs
}

func (s *server) StopAllListeners() int {
	var cnt int
	for _, l := range s.lsnrs {
		if err := l.Stop(); err == nil {
			cnt += 1
		}
	}

	// clear all listers
	s.lsnrs = make([]Listener, 0)
	return cnt
}

func (s *server) ListenAddresses() []string {
	addrs := make([]string, 0)
	for _, c := range s.conf.TcpListeners {
		addrs = append(addrs, c.ListenAddress)
	}
	return addrs
}

func (s *server) SetDelegate(d ServerDelegate) {
	s.delegate = d
}

func (s *server) Delegate() ServerDelegate {
	return s.delegate
}

func (s *server) UnregisterPeer(peer Peer) {
	peerId := peer.Id()
	clientId := peer.ClientId()
	if len(clientId) == 0 {
		clientId = strings.Repeat("-", 12)
	}
	commonName := peer.X509CommonName()
	if len(commonName) == 0 {
		commonName = strings.Repeat("-", 20)
	}

	if _, ok := s.peers.Get(peerId); ok {
		s.peers.Remove(peerId)
	}
	s.Metrics().ChannelCounter.Dec(1)
	// s.log.Tracef("%s %s Sess unregister peer %s", commonName, clientId, peerId)
}

func (s *server) RegisterPeer(peer Peer) {
	peerId := peer.Id()
	clientId := peer.ClientId()
	if len(clientId) == 0 {
		clientId = strings.Repeat("-", 12)
	}
	commonName := peer.X509CommonName()
	if len(commonName) == 0 {
		commonName = strings.Repeat("-", 20)
	}

	if old, ok := s.peers.Get(peerId); ok {
		// never happen
		// 동일 remote address로 복수의 Peer가 존재할 수 없다.
		// already existing peer with same remote address
		s.log.Warnf("%s %s Sess caution, registering same remote address %s", commonName, old.(Peer).ClientId(), peerId)
	}
	s.peers.Set(peerId, peer)
	s.Metrics().ChannelCounter.Inc(1)

	// s.log.Tracef("%s %s Sess register peer %s", commonName, clientId, peerId)
}

func (s *server) LogReject(remoteAddr string, cause error) {
	isHealthChecker := false
	for _, p := range s.conf.HealthCheckAddrs {
		// "tcp/10.20.1.252:"
		if strings.HasPrefix(remoteAddr, p) {
			isHealthChecker = true
			break
		}
	}

	if !isHealthChecker {
		// ignore health check conneciton
		s.log.Info("connection rejected", remoteAddr, cause)
	}
}

func (s *server) CountPeers() int64 {
	return int64(s.peers.Count())
}

func (s *server) IteratePeers(cb func(p Peer) bool) {
	for item := range s.peers.IterBuffered() {
		if p, ok := item.Val.(Peer); ok {
			if !cb(p) {
				return
			}
		}
	}
}

func (s *server) GetPeer(peerId string) (Peer, bool) {
	if p, ok := s.peers.Get(peerId); ok && p != nil {
		return p.(Peer), ok
	} else {
		return nil, false
	}
}

func (s *server) CloseAllPeers() {
	for _, p := range s.peers.Items() {
		p.(Peer).Close()
	}
}

func (s *server) OnConfigChange(newConf *MqttConfig) {
	//// allowance
	newAllowance := allowance.NewAllowanceFromConfig(&newConf.Allowance)
	s.allowance = newAllowance
	for _, l := range s.lsnrs {
		l.SetAllowance(s.allowance)
	}
}

func (s *server) Metrics() *ServerMetrics {
	return s.metrics
}

/*
func (s *server) Ingest(m *Message) {
	if m == nil {
		return
	}

	if s.eventLoop != nil {
		s.eventLoop.Ingest(m)
	}
}
*/

func (s *server) GetOtpGenerator(key string) (security.Generator, error) {
	period := 60
	if s.OtpPrefixes != nil && len(key) > 3 {
		key, _ = s.OtpPrefixes.Match(key)
	}
	return security.NewGenerator([]byte(key), period, []int{-1, -2, 1}, security.GeneratorHex12)
}
