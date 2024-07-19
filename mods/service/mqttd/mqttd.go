package mqttd

import (
	"context"
	"errors"
	"strings"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/service/allowance"
	"github.com/machbase/neo-server/mods/service/mqttd/mqtt"
	"github.com/machbase/neo-server/mods/service/security"
	"github.com/machbase/neo-server/mods/tql"
	cmap "github.com/orcaman/concurrent-map"
)

type Service interface {
	Start() error
	Stop()
}

type Option func(s *mqttd)

func New(db api.Database, options ...Option) (Service, error) {
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

type AppenderWrapper struct {
	conn      api.Conn
	appender  api.Appender
	ctx       context.Context
	ctxCancel context.CancelFunc
}

type HandlerType string

const (
	HandlerMachbase = HandlerType("machbase")
	HandlerInflux   = HandlerType("influx") // influx line protocol
	HandlerVoid     = HandlerType("-")
)

type mqttd struct {
	mqttd      mqtt.Server
	db         api.Database
	log        logging.Log
	appenders  cmap.ConcurrentMap
	authServer security.AuthServer
	tqlLoader  tql.Loader

	listenAddresses     []string
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
	for _, k := range svr.appenders.Keys() {
		svr.appenders.RemoveCb(k, func(key string, v interface{}, exists bool) bool {
			if as, ok := v.([]*AppenderWrapper); ok {
				for _, aw := range as {
					aw.conn.Close()
					aw.ctxCancel()
				}
			}
			return true
		})
	}
}

func (svr *mqttd) getTrustConnection(ctx context.Context, user string) (api.Conn, error) {
	return svr.db.Connect(ctx, api.WithTrustUser(user))
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

	pubTopic := []string{
		"db",
		"db/*",
		"metrics",
		"metrics/*",
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
		appenders := v.([]*AppenderWrapper)
		for _, aw := range appenders {
			aw.appender.Close()
			aw.conn.Close()
			aw.ctxCancel()
		}
		return true
	})
}

func (svr *mqttd) OnMessage(evt *mqtt.EvtMessage) error {
	if strings.HasPrefix(evt.Topic, "db/") {
		return svr.onMachbase(evt)
	} else if strings.HasPrefix(evt.Topic, "metrics/") {
		svr.onLineprotocol(evt)
	} else {
		svr.log.Warnf("unhandled message topic:'%s'", evt.Topic)
	}
	return nil
}
