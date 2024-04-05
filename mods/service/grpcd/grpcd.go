package grpcd

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-client/machrpc"
	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/api/bridge"
	"github.com/machbase/neo-server/api/mgmt"
	"github.com/machbase/neo-server/api/schedule"
	"github.com/machbase/neo-server/mods/leak"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/model"
	"github.com/machbase/neo-server/mods/service/internal/netutil"
	"github.com/machbase/neo-server/mods/service/security"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Service interface {
	Start() error
	Stop()
}

// Factory
func New(db *mach.Database, options ...Option) (Service, error) {
	s := &grpcd{
		log:            logging.GetLog("grpcd"),
		db:             db,
		maxRecvMsgSize: 4 * 1024 * 1024,
		maxSendMsgSize: 4 * 1024 * 1024,
	}

	for _, opt := range options {
		opt(s)
	}

	return s, nil
}

type Option func(svr *grpcd)

// ListenAddresses
func OptionListenAddress(addrs ...string) Option {
	return func(s *grpcd) {
		s.listenAddresses = append(s.listenAddresses, addrs...)
	}
}

// MaxRecvMsgSize
func OptionMaxRecvMsgSize(size int) Option {
	return func(s *grpcd) {
		s.maxRecvMsgSize = size
	}
}

// MaxSendMsgSize
func OptionMaxSendMsgSize(size int) Option {
	return func(s *grpcd) {
		s.maxSendMsgSize = size
	}
}

func OptionTlsCreds(keyPath string, certPath string) Option {
	return func(s *grpcd) {
		s.keyPath = keyPath
		s.certPath = certPath
	}
}

// mgmt implements
func OptionManagementServer(handler mgmt.ManagementServer) Option {
	return func(s *grpcd) {
		s.mgmtImpl = handler
	}
}

// bridge implements
func OptionBridgeServer(handler any) Option {
	return func(s *grpcd) {
		if o, ok := handler.(bridge.ManagementServer); ok {
			s.bridgeMgmtImpl = o
		}
		if o, ok := handler.(bridge.RuntimeServer); ok {
			s.bridgeRuntimeImpl = o
		}
	}
}

// schedule
func OptionScheduleServer(handler schedule.ManagementServer) Option {
	return func(s *grpcd) {
		s.schedMgmtImpl = handler
	}
}

func OptionLeakDetector(detector *leak.Detector) Option {
	return func(s *grpcd) {
		s.leakDetector = detector
	}
}

func OptionAuthServer(authServer security.AuthServer) Option {
	return func(s *grpcd) {
		s.authServer = authServer
	}
}

func OptionServicePortsFunc(portz func(svc string) ([]*model.ServicePort, error)) Option {
	return func(s *grpcd) {
		s.servicePortsFunc = portz
	}
}

func OptionServerInfoFunc(fn func() (*machrpc.ServerInfo, error)) Option {
	return func(s *grpcd) {
		s.serverInfoFunc = fn
	}
}

func OptionServerSessionsFunc(fn func(statz, session bool) (*machrpc.Statz, []*machrpc.Session, error)) Option {
	return func(s *grpcd) {
		s.serverSessionsFunc = fn
	}
}

func OptionServerKillSessionFunc(fn func(id string) error) Option {
	return func(s *grpcd) {
		s.serverKillSessionFunc = fn
	}
}

type grpcd struct {
	machrpc.UnimplementedMachbaseServer

	log logging.Log

	authServer security.AuthServer

	db           *mach.Database
	sessions     map[string]*connParole
	sessionsLock sync.Mutex

	listenAddresses []string
	maxRecvMsgSize  int
	maxSendMsgSize  int
	keyPath         string
	certPath        string

	leakDetector          *leak.Detector
	mgmtImpl              mgmt.ManagementServer
	bridgeMgmtImpl        bridge.ManagementServer
	bridgeRuntimeImpl     bridge.RuntimeServer
	schedMgmtImpl         schedule.ManagementServer
	servicePortsFunc      func(svc string) ([]*model.ServicePort, error)
	serverInfoFunc        func() (*machrpc.ServerInfo, error)
	serverSessionsFunc    func(statz, session bool) (*machrpc.Statz, []*machrpc.Session, error)
	serverKillSessionFunc func(id string) error

	rpcServer  *grpc.Server
	mgmtServer *grpc.Server

	mgmtServerInsecure *grpc.Server
}

type connParole struct {
	rawConn *mach.Conn
	handle  string
	cretime time.Time
}

func (svr *grpcd) Start() error {
	grpcOptions := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(int(svr.maxRecvMsgSize)),
		grpc.MaxSendMsgSize(int(svr.maxSendMsgSize)),
		grpc.StatsHandler(svr),
	}

	if svr.db == nil {
		return errors.New("no database instance")
	}

	svr.sessions = map[string]*connParole{}

	// create grpc server insecure
	svr.mgmtServerInsecure = grpc.NewServer(grpcOptions...)

	// creds
	tlsCreds, err := svr.loadTlsCreds()
	if err != nil {
		return err
	}
	if tlsCreds != nil {
		grpcOptions = append(grpcOptions, grpc.Creds(tlsCreds))
		svr.log.Infof("gRPC TLS enabled")
	}

	// create grpc server
	svr.rpcServer = grpc.NewServer(grpcOptions...)
	svr.mgmtServer = grpc.NewServer(grpcOptions...)

	// rpcServer is serving the db service
	machrpc.RegisterMachbaseServer(svr.rpcServer, svr)

	// mgmtServer is serving general db service + mgmt service
	machrpc.RegisterMachbaseServer(svr.mgmtServer, svr)
	machrpc.RegisterMachbaseServer(svr.mgmtServerInsecure, svr)
	if svr.mgmtImpl != nil {
		mgmt.RegisterManagementServer(svr.mgmtServer, svr.mgmtImpl)
		mgmt.RegisterManagementServer(svr.mgmtServerInsecure, svr.mgmtImpl)
	}

	// mgmtServer serves bridge management service
	if svr.bridgeMgmtImpl != nil {
		bridge.RegisterManagementServer(svr.mgmtServer, svr.bridgeMgmtImpl)
		bridge.RegisterManagementServer(svr.mgmtServerInsecure, svr.bridgeMgmtImpl)
	}

	// rpcServer can serve bridge runtime service
	if svr.bridgeRuntimeImpl != nil {
		bridge.RegisterRuntimeServer(svr.rpcServer, svr.bridgeRuntimeImpl)
		bridge.RegisterRuntimeServer(svr.mgmtServer, svr.bridgeRuntimeImpl)
		bridge.RegisterRuntimeServer(svr.mgmtServerInsecure, svr.bridgeRuntimeImpl)
	}

	// schedServer management service
	if svr.schedMgmtImpl != nil {
		schedule.RegisterManagementServer(svr.mgmtServer, svr.schedMgmtImpl)
		schedule.RegisterManagementServer(svr.mgmtServerInsecure, svr.schedMgmtImpl)
	}

	// listeners
	for _, listen := range svr.listenAddresses {
		if runtime.GOOS == "windows" && strings.HasPrefix(listen, "unix://") {
			// s.log.Debugf("gRPC unable %s on Windows", listen)
			continue
		}
		lsnr, err := netutil.MakeListener(listen)
		if err != nil {
			return errors.Wrap(err, "cannot start with failed listener")
		}
		svr.log.Infof("gRPC Listen %s", listen)

		if runtime.GOOS == "windows" {
			// windows require mgmt service to shutdown process from neow
			go svr.mgmtServer.Serve(lsnr)
		} else {
			if strings.HasPrefix(listen, "unix://") {
				// only gRPC via Unix Socket and loopback is allowed to perform mgmt service
				go svr.mgmtServerInsecure.Serve(lsnr)
			} else if strings.HasPrefix(listen, "tcp://127.0.0.1:") {
				// only gRPC via Unix Socket and loopback is allowed to perform mgmt service
				go svr.mgmtServer.Serve(lsnr)
			} else {
				go svr.rpcServer.Serve(lsnr)
			}
		}
	}
	return nil
}

func (svr *grpcd) Stop() {
	if svr.rpcServer != nil {
		svr.rpcServer.Stop()
	}
	if svr.mgmtServer != nil {
		svr.mgmtServer.Stop()
	}
	if svr.mgmtServerInsecure != nil {
		svr.mgmtServerInsecure.Stop()
	}
}

func (svr *grpcd) loadTlsCreds() (credentials.TransportCredentials, error) {
	if len(svr.certPath) == 0 && len(svr.keyPath) == 0 {
		return nil, nil
	}

	cert, err := tls.LoadX509KeyPair(svr.certPath, svr.keyPath)
	if err != nil {
		return nil, err
	}

	caContent, _ := os.ReadFile(svr.certPath)
	block, _ := pem.Decode(caContent)
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, errors.Wrap(err, "fail to load server CA cert")
	}
	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		// VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		// 	// here, we can see peer's cert
		// 	return nil
		// },
		ClientCAs:          caPool,
		InsecureSkipVerify: true,
	}
	return credentials.NewTLS(tlsConfig), nil
}

func (svr *grpcd) getSession(handle string) (*connParole, bool) {
	svr.sessionsLock.Lock()
	ret, ok := svr.sessions[handle]
	svr.sessionsLock.Unlock()
	return ret, ok
}

func (svr *grpcd) setSession(handle string, conn *connParole) {
	svr.sessionsLock.Lock()
	svr.sessions[handle] = conn
	svr.sessionsLock.Unlock()
}

func (svr *grpcd) removeSession(handle string) {
	svr.sessionsLock.Lock()
	delete(svr.sessions, handle)
	svr.sessionsLock.Unlock()
}
