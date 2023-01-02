package mqttsvr

import (
	"fmt"
	"strings"

	"github.com/machbase/cemlib/allowance"
	"github.com/machbase/cemlib/logging"
	"github.com/machbase/cemlib/mqtt"
	mach "github.com/machbase/dbms-mach-go"
	cmap "github.com/orcaman/concurrent-map"
)

func New(conf *Config) *Server {
	svr := &Server{
		conf: conf,
		db:   mach.New(),
	}
	mqttdConf := &mqtt.MqttConfig{
		Name:             "machbase",
		TcpListeners:     []mqtt.TcpListenerConfig{},
		UnixSocketConfig: mqtt.UnixSocketListenerConfig{},
		Allowance: allowance.AllowanceConfig{
			Policy: "NONE",
		},
	}
	for _, c := range conf.Listeners {
		if strings.HasPrefix(c, "tcp://") {
			mqttdConf.TcpListeners = append(mqttdConf.TcpListeners, mqtt.TcpListenerConfig{
				ListenAddress: strings.TrimPrefix(c, "tcp://"),
				SoLinger:      0,
				KeepAlive:     10,
				NoDelay:       true,
			})
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
}

type HandlerConfig struct {
	Prefix  string
	Handler string
}

type Server struct {
	conf  *Config
	mqttd mqtt.Server
	db    *mach.Database
	log   logging.Log

	appenders cmap.ConcurrentMap
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

func (svr *Server) OnConnect(evt *mqtt.EvtConnect) (mqtt.AuthCode, *mqtt.ConnectResult, error) {
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
		appenders := v.([]*mach.Appender)
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
