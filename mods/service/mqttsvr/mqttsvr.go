package mqttsvr

import (
	"fmt"
	"strings"

	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/service/allowance"
	"github.com/machbase/neo-server/mods/service/mqttsvr/mqtt"
	"github.com/machbase/neo-server/mods/service/security"
	spi "github.com/machbase/neo-spi"
	cmap "github.com/orcaman/concurrent-map"
)

func New(db spi.Database, conf *Config) *Server {
	svr := &Server{
		conf: conf,
		db:   db,
	}
	mqttdConf := &mqtt.MqttConfig{
		Name:             "machbase",
		TcpListeners:     []mqtt.TcpListenerConfig{},
		UnixSocketConfig: mqtt.UnixSocketListenerConfig{},
		Allowance: allowance.AllowanceConfig{
			Policy: "NONE",
		},
		MaxMessageSizeLimit: conf.MaxMessageSizeLimit,
	}
	for _, c := range conf.Listeners {
		if strings.HasPrefix(c, "tcp://") {
			tcpConf := mqtt.TcpListenerConfig{
				ListenAddress: strings.TrimPrefix(c, "tcp://"),
				SoLinger:      0,
				KeepAlive:     10,
				NoDelay:       true,
			}
			if conf.EnableTls {
				tcpConf.Tls.Disabled = false
				tcpConf.Tls.LoadSystemCAs = false
				tcpConf.Tls.LoadPrivateCAs = true
				tcpConf.Tls.CertFile = conf.ServerCertPath
				tcpConf.Tls.KeyFile = conf.ServerKeyPath
			} else {
				tcpConf.Tls.Disabled = true
			}
			mqttdConf.TcpListeners = append(mqttdConf.TcpListeners, tcpConf)
		} else if strings.HasPrefix(c, "unix://") {
			mqttdConf.UnixSocketConfig = mqtt.UnixSocketListenerConfig{
				Path:       strings.TrimPrefix(c, "unix://"),
				Permission: 0644,
			}
		}
	}
	for i, h := range conf.Handlers {
		if len(h.Prefix) > 0 {
			conf.Handlers[i].Prefix = strings.TrimSuffix(h.Prefix, "/")
		}
	}
	svr.mqttd = mqtt.NewServer(mqttdConf, svr)
	return svr
}

type Config struct {
	Listeners []string
	Handlers  []HandlerConfig
	Passwords map[string]string

	MaxMessageSizeLimit int

	ServerCertPath string
	ServerKeyPath  string

	EnableTokenAuth bool
	EnableTls       bool
}

type HandlerConfig struct {
	Prefix  string
	Handler string
}

type Server struct {
	conf  *Config
	mqttd mqtt.Server
	db    spi.Database
	log   logging.Log

	appenders cmap.ConcurrentMap

	authServer security.AuthServer // injection point
}

func (svr *Server) Start() error {
	svr.log = logging.GetLog("mqttsvr")
	svr.appenders = cmap.New()

	err := svr.mqttd.Start()
	if err != nil {
		return err
	}

	return nil
}

func (svr *Server) Stop() {
	if svr.mqttd != nil {
		svr.mqttd.Stop()
	}
}

func (svr *Server) SetAuthServer(authServer security.AuthServer) {
	svr.authServer = authServer
}

func (svr *Server) OnConnect(evt *mqtt.EvtConnect) (mqtt.AuthCode, *mqtt.ConnectResult, error) {
	if svr.conf.EnableTokenAuth {
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
	if svr.conf.EnableTls {
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

	peer, ok := svr.mqttd.GetPeer(evt.PeerId)
	if ok {
		peer.SetLogLevel(logging.LevelDebug)
	}

	pubTopic := []string{}
	for _, h := range svr.conf.Handlers {
		pubTopic = append(pubTopic, h.Prefix)
		pubTopic = append(pubTopic, fmt.Sprintf("%s/*", h.Prefix))
	}
	result := &mqtt.ConnectResult{
		AllowedPublishTopicPatterns:   pubTopic,
		AllowedSubscribeTopicPatterns: []string{"*"},
	}
	return mqtt.AuthSuccess, result, nil
}

func (svr *Server) OnDisconnect(evt *mqtt.EvtDisconnect) {
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

func (svr *Server) OnMessage(evt *mqtt.EvtMessage) error {
	handler := "machbase"
	prefix := ""
	for _, h := range svr.conf.Handlers {
		if strings.HasPrefix(evt.Topic, h.Prefix) {
			prefix = h.Prefix
			handler = h.Handler
			break
		}
	}

	switch handler {
	case "influx":
		svr.onLineprotocol(evt, prefix)
	case "machbase":
		return svr.onMachbase(evt, prefix)
	default:
		svr.log.Warnf("unhandled message topic:'%s'", evt.Topic)
	}
	return nil
}
