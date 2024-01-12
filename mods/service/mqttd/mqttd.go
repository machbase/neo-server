package mqttd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/service/allowance"
	"github.com/machbase/neo-server/mods/service/mqttd/mqtt"
	"github.com/machbase/neo-server/mods/service/security"
	"github.com/machbase/neo-server/mods/tql"
	spi "github.com/machbase/neo-spi"
	cmap "github.com/orcaman/concurrent-map"
)

type Service interface {
	Start() error
	Stop()
}

type Option func(s *mqttd)

func New(db spi.Database, options ...Option) (Service, error) {
	svr := &mqttd{
		log:       logging.GetLog("mqttd"),
		db:        db,
		appenders: cmap.New(),
	}
	for _, opt := range options {
		opt(svr)
	}
	return svr, nil
}

func OptionListenAddress(addr ...string) Option {
	return func(s *mqttd) {
		s.listenAddresses = append(s.listenAddresses, addr...)
	}
}

func OptionHandler(prefix string, handler HandlerType) Option {
	return func(s *mqttd) {
		s.handlers = append(s.handlers, &HandlerConfig{Prefix: prefix, Handler: handler})
	}
}

func OptionMaxMessageSizeLimit(limit int) Option {
	return func(s *mqttd) {
		s.maxMessageSizeLimit = limit
	}
}

func OptionPeerDefaultLogLevel(lvl logging.Level) Option {
	return func(s *mqttd) {
		s.mqttd.SetPeerDefaultLogLevel(lvl)
	}
}

func OptionAuthServer(authSvc security.AuthServer, enabled bool) Option {
	return func(s *mqttd) {
		s.authServer = authSvc
		s.enableTokenAuth = enabled
		if enabled {
			s.log.Infof("MQTT token authentication enabled")
		} else {
			s.log.Infof("MQTT token authentication disabled")
		}
	}
}

func OptionTls(serverCertPath string, serverKeyPath string) Option {
	return func(s *mqttd) {
		s.enableTls = true
		s.serverCertPath = serverCertPath
		s.serverKeyPath = serverKeyPath
		s.log.Infof("MQTT TLS enabled")
	}
}

func OptionTqlLoader(loader tql.Loader) Option {
	return func(s *mqttd) {
		s.tqlLoader = loader
	}
}

type HandlerType string

const (
	HandlerMachbase = HandlerType("machbase")
	HandlerInflux   = HandlerType("influx") // influx line protocol
	HandlerVoid     = HandlerType("-")
)

type HandlerConfig struct {
	Prefix  string
	Handler HandlerType
}

type mqttd struct {
	mqttd      mqtt.Server
	db         spi.Database
	dbConn     spi.Conn
	log        logging.Log
	appenders  cmap.ConcurrentMap
	authServer security.AuthServer
	tqlLoader  tql.Loader

	dbCtx       context.Context
	dbCtxCancel context.CancelFunc

	listenAddresses     []string
	handlers            []*HandlerConfig
	Passwords           map[string]string
	maxMessageSizeLimit int
	serverCertPath      string
	serverKeyPath       string
	enableTokenAuth     bool
	enableTls           bool
}

func (svr *mqttd) Start() error {
	if svr.db == nil {
		return errors.New("no database instance")
	}

	svr.dbCtx, svr.dbCtxCancel = context.WithCancel(context.Background())
	if conn, err := svr.db.Connect(svr.dbCtx, mach.WithTrustUser("sys")); err != nil {
		return err
	} else {
		svr.dbConn = conn
	}

	for i, h := range svr.handlers {
		if len(h.Prefix) > 0 {
			svr.handlers[i].Prefix = strings.TrimSuffix(h.Prefix, "/")
		}
	}

	mqttdConf := &mqtt.MqttConfig{
		Name:             "machbase",
		TcpListeners:     []mqtt.TcpListenerConfig{},
		UnixSocketConfig: mqtt.UnixSocketListenerConfig{},
		Allowance: allowance.AllowanceConfig{
			Policy: "NONE",
		},
		MaxMessageSizeLimit: svr.maxMessageSizeLimit,
	}
	for _, addr := range svr.listenAddresses {
		if strings.HasPrefix(addr, "tcp://") {
			tcpConf := mqtt.TcpListenerConfig{
				ListenAddress: strings.TrimPrefix(addr, "tcp://"),
				SoLinger:      0,
				KeepAlive:     10,
				NoDelay:       true,
			}
			if svr.enableTls {
				tcpConf.Tls.Disabled = false
				tcpConf.Tls.LoadSystemCAs = false
				tcpConf.Tls.LoadPrivateCAs = true
				tcpConf.Tls.CertFile = svr.serverCertPath
				tcpConf.Tls.KeyFile = svr.serverKeyPath
			} else {
				tcpConf.Tls.Disabled = true
			}
			mqttdConf.TcpListeners = append(mqttdConf.TcpListeners, tcpConf)
		} else if strings.HasPrefix(addr, "unix://") {
			mqttdConf.UnixSocketConfig = mqtt.UnixSocketListenerConfig{
				Path:       strings.TrimPrefix(addr, "unix://"),
				Permission: 0644,
			}
		}
	}
	svr.mqttd = mqtt.NewServer(mqttdConf, svr)

	err := svr.mqttd.Start()
	if err != nil {
		return err
	}

	return nil
}

func (svr *mqttd) Stop() {
	if svr.mqttd != nil {
		svr.mqttd.Stop()
	}
	if svr.dbConn != nil {
		svr.dbConn.Close()
		svr.dbCtxCancel()
	}
}

func (svr *mqttd) getTrustConnection(ctx context.Context) (spi.Conn, error) {
	return svr.db.Connect(ctx, mach.WithTrustUser("sys"))
}

func (svr *mqttd) SetAuthServer(authServer security.AuthServer) {
	svr.authServer = authServer
}

func (svr *mqttd) OnConnect(evt *mqtt.EvtConnect) (mqtt.AuthCode, *mqtt.ConnectResult, error) {
	if svr.enableTokenAuth {
		if svr.authServer == nil {
			return mqtt.AuthDenied, nil, nil
		}
		clientId := evt.ClientId
		username := evt.Username // contains token
		svr.log.Tracef("MQTT auth '%s' token '%s'", clientId, username)
		if !strings.HasPrefix(username, clientId) {
			return mqtt.AuthError, nil, nil
		}
		pass, err := svr.authServer.ValidateClientToken(string(username))
		if err != nil {
			return mqtt.AuthDenied, nil, err
		}
		if !pass {
			return mqtt.AuthError, nil, nil
		}
	}
	if svr.enableTls {
		if svr.authServer == nil {
			return mqtt.AuthDenied, nil, nil
		}
		clientId := evt.ClientId
		certHash := evt.CertHash
		svr.log.Tracef("MQTT auth '%s' cert %s", clientId, certHash)
		pass, err := svr.authServer.ValidateClientCertificate(clientId, certHash)
		if err != nil {
			return mqtt.AuthDenied, nil, err
		}
		if !pass {
			return mqtt.AuthError, nil, nil
		}
	}

	pubTopic := []string{}
	for _, h := range svr.handlers {
		pubTopic = append(pubTopic, h.Prefix)
		pubTopic = append(pubTopic, fmt.Sprintf("%s/*", h.Prefix))
	}
	result := &mqtt.ConnectResult{
		AllowedPublishTopicPatterns:   pubTopic,
		AllowedSubscribeTopicPatterns: []string{"*"},
	}
	return mqtt.AuthSuccess, result, nil
}

func (svr *mqttd) OnDisconnect(evt *mqtt.EvtDisconnect) {
	svr.appenders.RemoveCb(evt.PeerId, func(key string, v interface{}, exists bool) bool {
		if !exists {
			return false
		}
		appenders := v.([]spi.Appender)
		for _, ap := range appenders {
			ap.Close()
		}
		return true
	})
}

func (svr *mqttd) OnMessage(evt *mqtt.EvtMessage) error {
	handler := HandlerType("machbase")
	prefix := ""
	for _, h := range svr.handlers {
		if strings.HasPrefix(evt.Topic, h.Prefix) {
			prefix = h.Prefix
			handler = h.Handler
			break
		}
	}

	switch handler {
	case HandlerInflux:
		svr.onLineprotocol(evt, prefix)
	case HandlerMachbase:
		return svr.onMachbase(evt, prefix)
	default:
		svr.log.Warnf("unhandled message topic:'%s'", evt.Topic)
	}
	return nil
}
