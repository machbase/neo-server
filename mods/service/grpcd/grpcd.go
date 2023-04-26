package grpcd

import (
	"runtime"
	"strings"

	"github.com/machbase/neo-grpc/machrpc"
	"github.com/machbase/neo-grpc/mgmt"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/service/internal/netutil"
	spi "github.com/machbase/neo-spi"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

type Service interface {
	Start() error
	Stop()
}

// Factory
func New(db spi.Database, options ...Option) (Service, error) {
	s := &grpcd{
		log:            logging.GetLog("grpcd"),
		db:             db,
		maxRecvMsgSize: 4 * 1024 * 1024,
		maxSendMsgSize: 4 * 1024 * 1024,
		ctxMap:         cmap.New(),
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

// mgmt implements
func OptionManagementServer(handler mgmt.ManagementServer) Option {
	return func(s *grpcd) {
		s.mgmtImpl = handler
	}
}

type grpcd struct {
	machrpc.MachbaseServer

	log logging.Log
	db  spi.Database

	listenAddresses []string
	maxRecvMsgSize  int
	maxSendMsgSize  int

	mgmtImpl mgmt.ManagementServer

	ctxMap     cmap.ConcurrentMap
	rpcServer  *grpc.Server
	mgmtServer *grpc.Server
}

func (svr *grpcd) Start() error {
	grpcOptions := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(int(svr.maxRecvMsgSize)),
		grpc.MaxSendMsgSize(int(svr.maxSendMsgSize)),
		grpc.StatsHandler(svr),
	}

	// create grpc server
	svr.rpcServer = grpc.NewServer(grpcOptions...)
	svr.mgmtServer = grpc.NewServer(grpcOptions...)

	// rpcServer is serving only db service
	machrpc.RegisterMachbaseServer(svr.rpcServer, svr)
	// mgmtServer is serving general db service + mgmt service
	machrpc.RegisterMachbaseServer(svr.mgmtServer, svr)

	if svr.mgmtImpl != nil {
		mgmt.RegisterManagementServer(svr.mgmtServer, svr.mgmtImpl)
	}

	//listeners
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

		if strings.HasPrefix(listen, "unix://") || strings.HasPrefix(listen, "tcp://127.0.0.1:") {
			// only gRPC via Unix Socket and loopback is allowed to perform mgmt service
			go svr.mgmtServer.Serve(lsnr)
		} else {
			go svr.rpcServer.Serve(lsnr)
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
}
