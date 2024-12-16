package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/machbase/neo-server/v8/api/bridge"
	"github.com/machbase/neo-server/v8/api/machrpc"
	"github.com/machbase/neo-server/v8/api/machsvr"
	"github.com/machbase/neo-server/v8/api/mgmt"
	"github.com/machbase/neo-server/v8/api/schedule"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/stats"
)

// Factory
func NewGrpc(db *machsvr.RPCServer, options ...Option) (*grpcd, error) {
	s := &grpcd{
		log:            logging.GetLog("grpcd"),
		machImpl:       db,
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
func WithGrpcListenAddress(addrs ...string) Option {
	return func(s *grpcd) {
		s.listenAddresses = append(s.listenAddresses, addrs...)
	}
}

// MaxRecvMsgSize
func WithGrpcMaxRecvMsgSize(size int) Option {
	return func(s *grpcd) {
		s.maxRecvMsgSize = size
	}
}

// MaxSendMsgSize
func WithGrpcMaxSendMsgSize(size int) Option {
	return func(s *grpcd) {
		s.maxSendMsgSize = size
	}
}

func WithGrpcTlsCreds(keyPath string, certPath string) Option {
	return func(s *grpcd) {
		s.keyPath = keyPath
		s.certPath = certPath
	}
}

// mgmt implements
func WithGrpcManagementServer(handler mgmt.ManagementServer) Option {
	return func(s *grpcd) {
		s.mgmtImpl = handler
	}
}

// bridge implements
func WithGrpcBridgeServer(handler any) Option {
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
func WithGrpcScheduleServer(handler schedule.ManagementServer) Option {
	return func(s *grpcd) {
		s.schedMgmtImpl = handler
	}
}

func WithGrpcServerInsecure(ignoreInsecure bool) Option {
	return func(s *grpcd) {
		s.ignoreInsecure = ignoreInsecure
	}
}

type grpcd struct {
	log logging.Log

	listenAddresses []string
	maxRecvMsgSize  int
	maxSendMsgSize  int
	keyPath         string
	certPath        string
	ignoreInsecure  bool

	machImpl          *machsvr.RPCServer
	mgmtImpl          mgmt.ManagementServer
	bridgeMgmtImpl    bridge.ManagementServer
	bridgeRuntimeImpl bridge.RuntimeServer
	schedMgmtImpl     schedule.ManagementServer

	rpcServer          *grpc.Server
	rpcServerInsecure  *grpc.Server
	mgmtServer         *grpc.Server
	mgmtServerInsecure *grpc.Server
}

func (svr *grpcd) Start() error {
	grpcOptions := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(int(svr.maxRecvMsgSize)),
		grpc.MaxSendMsgSize(int(svr.maxSendMsgSize)),
		grpc.StatsHandler(svr),
	}

	if svr.machImpl == nil {
		return errors.New("no database instance")
	}

	// create grpc server insecure
	svr.mgmtServerInsecure = grpc.NewServer(grpcOptions...)
	svr.rpcServerInsecure = grpc.NewServer(grpcOptions...)

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
	machrpc.RegisterMachbaseServer(svr.rpcServer, svr.machImpl)
	machrpc.RegisterMachbaseServer(svr.rpcServerInsecure, svr.machImpl)

	// mgmtServer is serving general db service + mgmt service
	machrpc.RegisterMachbaseServer(svr.mgmtServer, svr.machImpl)
	machrpc.RegisterMachbaseServer(svr.mgmtServerInsecure, svr.machImpl)
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
		lsnr, err := util.MakeListener(listen)
		if err != nil {
			return errors.Wrap(err, "cannot start with failed listener")
		}
		svr.log.Infof("gRPC Listen %s", listen)

		if runtime.GOOS == "windows" {
			// windows require mgmt service to shutdown process from neow
			if svr.ignoreInsecure {
				go svr.mgmtServerInsecure.Serve(lsnr)
			} else {
				go svr.mgmtServer.Serve(lsnr)
			}
		} else {
			if strings.HasPrefix(listen, "unix://") {
				// only gRPC via Unix Socket and loopback is allowed to perform mgmt service
				go svr.mgmtServerInsecure.Serve(lsnr)
			} else if strings.HasPrefix(listen, "tcp://127.0.0.1:") {
				// only gRPC via Unix Socket and loopback is allowed to perform mgmt service
				if svr.ignoreInsecure {
					go svr.mgmtServerInsecure.Serve(lsnr)
				} else {
					go svr.mgmtServer.Serve(lsnr)
				}
			} else {
				if svr.ignoreInsecure {
					go svr.rpcServerInsecure.Serve(lsnr)
				} else {
					go svr.rpcServer.Serve(lsnr)
				}
			}
		}
	}
	return nil
}

func (svr *grpcd) Stop() {
	if svr.rpcServer != nil {
		svr.rpcServer.GracefulStop()
	}
	if svr.rpcServerInsecure != nil {
		svr.rpcServerInsecure.GracefulStop()
	}
	if svr.mgmtServer != nil {
		svr.mgmtServer.GracefulStop()
	}
	if svr.mgmtServerInsecure != nil {
		svr.mgmtServerInsecure.GracefulStop()
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

type sessionCtx struct {
	context.Context
	Id     string
	values map[any]any
}

type stringer interface {
	String() string
}

func contextName(c context.Context) string {
	if s, ok := c.(stringer); ok {
		return s.String()
	}
	return reflect.TypeOf(c).String()
}

func (c *sessionCtx) String() string {
	return contextName(c.Context) + "(" + c.Id + ")"
}

func (c *sessionCtx) Value(key any) any {
	if key == contextCtxKey {
		return c
	}
	if v, ok := c.values[key]; ok {
		return v
	}
	return c.Context.Value(key)
}

const contextCtxKey = "machrpc-client-context"

var contextIdSerial int64

//// grpc stat handler

var _ stats.Handler = (*grpcd)(nil)

func (s *grpcd) TagRPC(ctx context.Context, nfo *stats.RPCTagInfo) context.Context {
	return ctx
}

func (s *grpcd) HandleRPC(ctx context.Context, stat stats.RPCStats) {
}

func (s *grpcd) TagConn(ctx context.Context, nfo *stats.ConnTagInfo) context.Context {
	id := strconv.FormatInt(atomic.AddInt64(&contextIdSerial, 1), 10)
	ctx = &sessionCtx{Context: ctx, Id: id}
	return ctx
}

func (s *grpcd) HandleConn(ctx context.Context, stat stats.ConnStats) {
	if _ /*sessCtx*/, ok := ctx.(*sessionCtx); ok {
		switch stat.(type) {
		case *stats.ConnBegin:
			// fmt.Printf("get connBegin: %v\n", sessCtx.Id)
		case *stats.ConnEnd:
			// fmt.Printf("get connEnd: %v\n", sessCtx.Id)
		}
	}
}
