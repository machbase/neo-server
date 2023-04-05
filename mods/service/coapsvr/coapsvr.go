package coapsvr

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"strings"
	"time"

	logging "github.com/machbase/neo-logging"
	spi "github.com/machbase/neo-spi"
	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/mux"
	"github.com/plgd-dev/go-coap/v3/net"
	"github.com/plgd-dev/go-coap/v3/options"
	"github.com/plgd-dev/go-coap/v3/tcp"
	"github.com/plgd-dev/go-coap/v3/udp"
)

func New(db spi.Database, conf *Config) (*Server, error) {
	return &Server{
		conf: conf,
		log:  logging.GetLog("coapsvr"),
		db:   db,
	}, nil
}

type Config struct {
	ListenAddress []string
	LogWriter     io.Writer
}

type Server struct {
	conf *Config
	log  logging.Log
	db   spi.Database

	listeners []io.Closer
	servers   []Stopper
}

type Stopper interface {
	Stop()
}

func (svr *Server) Start() error {
	r := mux.NewRouter()
	r.Use(svr.logging)
	if err := r.Handle("/a", mux.HandlerFunc(svr.handle)); err != nil {
		return err
	}

	for _, addr := range svr.conf.ListenAddress {
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
				svr.listeners = append(svr.listeners, lsnr)
				s := tcp.NewServer(options.WithMux(r))
				svr.servers = append(svr.servers, s)
				svr.LogPrint("CoAP Listen", network, addr)
				go s.Serve(lsnr)
			}
		case "udp":
			if tlsCfg == nil {
				lsnr, err := net.NewListenUDP(network, addr)
				if err != nil {
					return err
				}
				svr.listeners = append(svr.listeners, lsnr)
				s := udp.NewServer(options.WithMux(r))
				svr.servers = append(svr.servers, s)
				svr.LogPrint("CoAP Listen", network, addr)
				go s.Serve(lsnr)
			}
		}
	}
	return nil
}

func (svr *Server) Stop() {
	for _, s := range svr.servers {
		s.Stop()
	}
	for _, s := range svr.listeners {
		s.Close()
	}
}

func (svr *Server) LogPrint(strs ...any) {
	if svr.conf.LogWriter == nil {
		return
	}
	svr.conf.LogWriter.Write([]byte(fmt.Sprintln(strs...)))
}

func (svr *Server) logging(next mux.Handler) mux.Handler {
	return mux.HandlerFunc(func(w mux.ResponseWriter, r *mux.Message) {
		if svr.conf.LogWriter != nil {
			svr.conf.LogWriter.Write([]byte(fmt.Sprintf("ClientAddress %v, %v\n", w.Conn().RemoteAddr(), r.String())))
		}
		next.ServeCOAP(w, r)
	})
}

func (svr *Server) handle(w mux.ResponseWriter, req *mux.Message) {
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
