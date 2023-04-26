package mqtt

import (
	"time"

	"github.com/machbase/neo-server/mods/service/allowance"
)

type MqttConfig struct {
	Name             string
	TcpListeners     []TcpListenerConfig
	UnixSocketConfig UnixSocketListenerConfig
	Allowance        allowance.AllowanceConfig
	HealthCheckAddrs []string

	MaxMessageSizeLimit int
}

type TcpListenerConfig struct {
	ListenAddress string
	SoLinger      int
	KeepAlive     int
	NoDelay       bool
	Tls           TlsListenerConfig
}

type TlsListenerConfig struct {
	Disabled         bool
	LoadSystemCAs    bool          // If true, load system CA pool, if false do not load system CA
	LoadPrivateCAs   bool          // If true, load server's cert into CA pool
	CertFile         string        // PEM file path to server's cert
	KeyFile          string        // PEM file path to server's private key
	HandshakeTimeout time.Duration // SSL handshake timeout
}

type UnixSocketListenerConfig struct {
	Path       string
	Permission int
}
