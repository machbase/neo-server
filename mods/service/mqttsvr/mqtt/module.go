package mqtt

import (
	"reflect"
	"time"

	"github.com/machbase/booter"
)

func init() {
	RegisterBootFactory()
}

func RegisterBootFactory() {
	defaultConf := MqttConfig{
		TcpListeners: []TcpListenerConfig{
			{
				ListenAddress: "127.0.0.1:1883",
				SoLinger:      0,
				KeepAlive:     20,
				NoDelay:       false,
				Tls: TlsListenerConfig{
					LoadSystemCAs:    false,
					LoadPrivateCAs:   true,
					CertFile:         "",
					KeyFile:          "",
					HandshakeTimeout: 5 * time.Second,
				},
			},
		},
	}

	booter.Register(
		reflect.TypeOf(defaultConf).PkgPath(),
		func() *MqttConfig {
			clone := defaultConf
			return &clone
		},
		func(conf *MqttConfig) (booter.Boot, error) {
			return NewServer(conf, nil), nil
		},
	)
}
