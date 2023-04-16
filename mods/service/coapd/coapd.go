package coapd

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/logging"
	spi "github.com/machbase/neo-spi"
	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/mux"
	"github.com/plgd-dev/go-coap/v3/net"
	"github.com/plgd-dev/go-coap/v3/options"
	"github.com/plgd-dev/go-coap/v3/tcp"
	tcpServer "github.com/plgd-dev/go-coap/v3/tcp/server"
	"github.com/plgd-dev/go-coap/v3/udp"
	udpServer "github.com/plgd-dev/go-coap/v3/udp/server"
)

type Service interface {
	Start() error
	Stop()
}

type Option func(s *coapd)

// Factory
func New(db spi.Database, options ...Option) (Service, error) {
	s := &coapd{
		log: logging.GetLog("coapd"),
		db:  db,
	}

	for _, opt := range options {
		opt(s)
	}
	return s, nil
}

// ListenAddresses
func OptionListenAddress(addrs ...string) Option {
	return func(s *coapd) {
		s.listenAddresses = append(s.listenAddresses, addrs...)
	}
}

// LogWriter
func OptionLogWriter(w io.Writer) Option {
	return func(s *coapd) {
		s.logWriter = w
	}
}

type coapd struct {
	log logging.Log
	db  spi.Database

	listenAddresses []string
	logWriter       io.Writer

	tcpListeners []*net.TCPListener
	tcpServers   []*tcpServer.Server
	udpListeners []*net.UDPConn
	udpServers   []*udpServer.Server
}

func (svr *coapd) Start() error {
	r := mux.NewRouter()
	r.Use(svr.logging)
	if err := r.Handle("/a", mux.HandlerFunc(svr.handle)); err != nil {
		return err
	}

	for _, addr := range svr.listenAddresses {
		network := "udp"
		var tlsCfg *tls.Config
		if strings.HasPrefix(addr, "tcp://") {
			network = "tcp"
			addr = strings.TrimPrefix(addr, "tcp://")
		} else if strings.HasPrefix(addr, "tls://") {
			network = "tcp"
			addr = strings.TrimPrefix(addr, "tls://")
		} else if strings.HasPrefix(addr, "udp://") {
			network = "udp"
			addr = strings.TrimPrefix(addr, "udp://")
		} else if strings.HasPrefix(addr, "dtls://") {
			network = "udp"
			addr = strings.TrimPrefix(addr, "dtls://")
		}

		switch network {
		case "tcp":
			if tlsCfg == nil {
				lsnr, err := net.NewTCPListener(network, addr)
				if err != nil {
					return err
				}
				svr.tcpListeners = append(svr.tcpListeners, lsnr)
				s := tcp.NewServer(options.WithMux(r))
				svr.tcpServers = append(svr.tcpServers, s)
				svr.LogPrint("CoAP Listen", network, addr)
				go s.Serve(lsnr)
			}
		case "udp":
			if tlsCfg == nil {
				lsnr, err := net.NewListenUDP(network, addr)
				if err != nil {
					return err
				}
				svr.udpListeners = append(svr.udpListeners, lsnr)
				s := udp.NewServer(options.WithMux(r))
				svr.udpServers = append(svr.udpServers, s)
				svr.LogPrint("CoAP Listen", network, addr)
				go s.Serve(lsnr)
			}
		}
	}
	return nil
}

func (svr *coapd) Stop() {
	for _, s := range svr.tcpServers {
		s.Stop()
	}
	for _, s := range svr.tcpListeners {
		s.Close()
	}
	for _, s := range svr.udpServers {
		s.Stop()
	}
	for _, s := range svr.udpListeners {
		s.Close()
	}
}

func (svr *coapd) LogPrint(strs ...any) {
	if svr.logWriter == nil {
		return
	}
	svr.logWriter.Write([]byte(fmt.Sprintln(strs...)))
}

func (svr *coapd) logging(next mux.Handler) mux.Handler {
	return mux.HandlerFunc(func(w mux.ResponseWriter, r *mux.Message) {
		if svr.logWriter != nil {
			svr.logWriter.Write([]byte(fmt.Sprintf("ClientAddress %v, %v\n", w.Conn().RemoteAddr(), r.String())))
		}
		next.ServeCOAP(w, r)
	})
}

func (svr *coapd) handle(w mux.ResponseWriter, req *mux.Message) {
	obs, err := req.Options().Observe()
	switch {
	case req.Code() == codes.GET && err == nil && obs == 0:
		go periodicTransmitter(w.Conn(), req.Token())
	case req.Code() == codes.GET:
		err := sendResponse(w.Conn(), req.Token(), time.Now(), -1)
		if err != nil {
			svr.LogPrint("Error on transmitter:", err)
		}
	}
}

func sendResponse(cc mux.Conn, token []byte, subded time.Time, obs int64) error {
	m := cc.AcquireMessage(cc.Context())
	defer cc.ReleaseMessage(m)
	m.SetCode(codes.Content)
	m.SetToken(token)
	m.SetBody(bytes.NewReader([]byte(fmt.Sprintf("Been running for %v", time.Since(subded)))))
	m.SetContentFormat(message.TextPlain)
	if obs >= 0 {
		m.SetObserve(uint32(obs))
	}
	return cc.WriteMessage(m)
}

func periodicTransmitter(cc mux.Conn, token []byte) {
	subded := time.Now()

	for obs := int64(2); ; obs++ {
		err := sendResponse(cc, token, subded, obs)
		if err != nil {
			return
		}
		time.Sleep(time.Second)
	}
}
