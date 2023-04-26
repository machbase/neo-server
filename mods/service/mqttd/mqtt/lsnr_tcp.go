package mqtt

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/service/allowance"
	"github.com/pkg/errors"
)

type tcpListener struct {
	isTls      bool
	raw        net.Listener
	address    string
	acceptChan chan<- any
	tcpConfig  *TcpListenerConfig
	tlsConfig  *TlsListenerConfig
	allowance  allowance.Allowance

	log       logging.Log
	alive     bool
	name      string
	closeWait sync.WaitGroup
}

func newTcpListener(cfg *TcpListenerConfig, acceptChan chan<- any) (*tcpListener, error) {
	tlsConfig := &cfg.Tls
	if cfg.Tls.Disabled {
		tlsConfig = nil
	}

	lsnr := &tcpListener{
		address:    cfg.ListenAddress,
		acceptChan: acceptChan,
		tcpConfig:  cfg,
		tlsConfig:  tlsConfig,
		alive:      false,
		name:       "mqtt-tcp",
		closeWait:  sync.WaitGroup{},
	}
	lsnr.log = logging.GetLog(lsnr.name)

	if err := lsnr.buildRawListener(); err != nil {
		return nil, err
	}
	return lsnr, nil
}

func (l *tcpListener) buildRawListener() error {
	if rt, err := buildTlsConfig(l.tlsConfig, l.tcpConfig); err != nil {
		return err
	} else {
		if rt == nil {
			l.raw, err = net.Listen("tcp", l.address)
			l.isTls = false
			return err
		} else {
			l.raw, err = tls.Listen("tcp", l.address, rt)
			l.isTls = true
			return err
		}
	}
}

func buildTlsConfig(tlsCfg *TlsListenerConfig, tcpCfg *TcpListenerConfig) (*tls.Config, error) {
	if tcpCfg == nil || tlsCfg == nil {
		return nil, nil
	}

	if len(tlsCfg.CertFile) == 0 || len(tlsCfg.KeyFile) == 0 {
		return nil, nil
	}

	cert, err := tls.LoadX509KeyPair(tlsCfg.CertFile, tlsCfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("fail to load x509 pair %s %s; %s", tlsCfg.CertFile, tlsCfg.KeyFile, err)
	}

	var rootCAs *x509.CertPool
	if tlsCfg.LoadSystemCAs {
		rootCAs, _ = x509.SystemCertPool()
	}
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	if tlsCfg.LoadPrivateCAs {
		// append root ca
		ca, err := os.ReadFile(tlsCfg.CertFile)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("fail to load ca key: %s\n", tlsCfg.CertFile))
		}
		if ok := rootCAs.AppendCertsFromPEM(ca); !ok {
			return nil, errors.Wrap(err, fmt.Sprintf("fail to add ca key: %s\n", tlsCfg.CertFile))
		}
	}

	rt := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		ClientAuth:         tls.RequireAndVerifyClientCert,
		ClientCAs:          rootCAs,
		GetConfigForClient: configTlsConn(tcpCfg),
	}
	return rt, nil
}

func configTlsConn(tcpConfig *TcpListenerConfig) func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
	return func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
		if conn, ok := hello.Conn.(*net.TCPConn); ok {
			tcpConfig.configure(conn)
		}
		return nil, nil
	}
}

func (l *tcpListener) Start() error {
	if l.alive {
		return nil
	}

	if l.raw == nil {
		if err := l.buildRawListener(); err != nil {
			return err
		}
	}
	l.alive = true
	l.closeWait.Add(1)
	go l.runTcpListener()
	return nil
}

func (l *tcpListener) Stop() error {
	if !l.alive {
		return nil
	}

	l.alive = false
	if l.raw != nil {
		if err := l.raw.Close(); err != nil {
			return err
		}
		l.raw = nil
	}
	l.closeWait.Wait()
	return nil
}

func (l *tcpListener) SetAllowance(allowance allowance.Allowance) {
	l.allowance = allowance
}

func (l *tcpListener) Name() string {
	return l.name
}

func (l *tcpListener) IsAlive() bool {
	return l.alive
}

// blocks; the caller typically invokes it in a go statement.
func (l *tcpListener) runTcpListener() {
	listenAddr := l.raw.Addr()
	l.log.Infof("MQTT Listen tcp://%s", listenAddr)
	defer func() {
		l.log.Tracef("Stop listener %s", listenAddr)
		l.closeWait.Done()
	}()

	for {
		conn, err := l.raw.Accept()
		if err != nil {
			if !l.alive {
				return
			}
			if ne, ok := err.(net.Error); ok {
				if ne.Temporary() {
					l.log.Warnf("accept temporary failed: %s", err)
					continue
				}
			}
			l.log.Errorf("socket failed: %s", err)
			return
		}
		// else {
		// 	 l.log.Printf("accept: %s", conn.RemoteAddr())
		// }

		switch incoming := conn.(type) {
		case *net.TCPConn:
			go l.incomingTcpConn(incoming)
		case *tls.Conn:
			go l.incomingTlsConn(incoming)
		default:
			l.log.Errorf("unsupported connection type: %+v", incoming)
			incoming.Close()
		}
	}
}

func (l *tcpListener) incomingTcpConn(conn *net.TCPConn) {
	l.tcpConfig.configure(conn)

	if !l.checkAllowance(conn) {
		return
	}
	l.acceptChan <- NewTcpConnection(conn)
}

func (l *tcpListener) incomingTlsConn(conn *tls.Conn) {
	if !l.checkAllowance(conn) {
		return
	}

	var cancel context.CancelFunc = nil

	ctx := context.Background()
	if l.tlsConfig.HandshakeTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(l.tlsConfig.HandshakeTimeout))
	}

	if err := conn.HandshakeContext(ctx); err != nil {
		remoteAddr := conn.RemoteAddr()
		l.log.Tracef("%s %s %s handshake error, %s", strings.Repeat("-", 20), strings.Repeat("-", 12), remoteAddr, err)
		conn.Close()
		if cancel != nil {
			cancel()
		}
		l.notifyRejectConn(remoteAddr, RejectSslHandshake, err)
		return
	}
	if cancel != nil {
		cancel()
	}

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 || state.PeerCertificates[0] == nil {
		remoteAddr := conn.RemoteAddr()
		l.log.Tracef("%s %s %s invalid client cert", strings.Repeat("-", 20), strings.Repeat("-", 12), remoteAddr)
		conn.Close()
		l.notifyRejectConn(remoteAddr, RejectInvalidCertificate, nil)
		return
	}
	clientCert := state.PeerCertificates[0]

	l.acceptChan <- NewTlsConnection(conn, clientCert)
}

func (l *tcpListener) notifyRejectConn(addr net.Addr, reason RejectReason, reasonErr error) {
	host, portStr, err := net.SplitHostPort(addr.String())
	if err != nil {
		host = addr.String()
		portStr = "0"
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		port = 0
	}
	rj := NewRejectConnection(host, port, reason, reasonErr)
	l.acceptChan <- rj
}

func (l *tcpListener) checkAllowance(conn net.Conn) bool {
	if l.allowance != nil {
		remoteAddr := conn.RemoteAddr()
		host, _, _ := net.SplitHostPort(remoteAddr.String())
		if !l.allowance.Allow(host) {
			conn.Close()
			l.notifyRejectConn(remoteAddr, RejectByAllowancePolicy, nil)
			return false
		}
	}
	return true
}
