package testsuite

import (
	"context"
	_ "embed"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/api/machcli"
	"github.com/machbase/neo-server/api/machrpc"
	"github.com/machbase/neo-server/api/machsvr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/test/bufconn"
)

//go:embed testsuite.conf
var defaultConfig []byte

var defaultLock = &sync.Mutex{}

type Server struct {
	machsvrConfig   []byte
	machsvrDatabase *machsvr.Database
	machsvrDataDir  string
	machsvrPort     int
	machcliDatabase *machcli.Database
	grpcServer      *grpc.Server
	grpcListener    *bufconn.Listener
	grpcClientConn  *grpc.ClientConn
	onceStart       sync.Once
	onceStop        sync.Once
}

func NewServer(dataPath string) *Server {
	defaultLock.Lock()
	defer defaultLock.Unlock()

	ret := &Server{
		machsvrConfig:  defaultConfig,
		machsvrDataDir: dataPath,
	}
	return ret
}

func (s *Server) checkListenPort() {
	time.Sleep(time.Millisecond * time.Duration(3000*rand.Float32()))
	var lsnr net.Listener
	for {
		if l, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
			continue
		} else {
			lsnr = l
			s.machsvrPort = l.Addr().(*net.TCPAddr).Port
			break
		}
	}
	lsnr.Close()
}

func (s *Server) StartServer(m *testing.M) {
	s.onceStart.Do(func() {
		s.startServer()
	})
}

func (s *Server) startServer() {
	// prepare
	homePath, err := filepath.Abs(filepath.Join(s.machsvrDataDir, "machbase"))
	if err != nil {
		panic(err)
	}
	confPath := filepath.Join(homePath, "conf", "machbase.conf")

	os.RemoveAll(homePath)
	os.MkdirAll(homePath, 0755)
	os.MkdirAll(filepath.Join(homePath, "conf"), 0755)
	os.MkdirAll(filepath.Join(homePath, "trc"), 0755)
	os.MkdirAll(filepath.Join(homePath, "dbs"), 0755)
	os.WriteFile(confPath, defaultConfig, 0644)

	// available port
	s.checkListenPort()
	if err := machsvr.Initialize(homePath, s.machsvrPort, machsvr.OPT_SIGHANDLER_OFF); err != nil {
		panic(err)
	}

	if !machsvr.ExistsDatabase() {
		if err := machsvr.CreateDatabase(); err != nil {
			panic(err)
		}
	}

	// setup
	if db, err := machsvr.NewDatabase(); err != nil {
		panic(err)
	} else {
		s.machsvrDatabase = db
	}

	if err := s.machsvrDatabase.Startup(); err != nil {
		// why this happens?
		//
		// MACH-ERR 3208 Server thread error: 3046 - Communication module error (rc=21): [mmpInitialize].
		panic(err)
	}

	// create test tables
	ctx := context.TODO()
	conn, _ := s.machsvrDatabase.Connect(ctx, api.WithTrustUser("sys"))
	result := conn.Exec(ctx, api.SqlTidy(`
		create tag table tag_data(
			name            varchar(100) primary key, 
			time            datetime basetime, 
			value           double summarized,
			short_value     short,
			ushort_value    ushort,
			int_value       integer,
			uint_value 	    uinteger,
			long_value      long,
			ulong_value 	ulong,
			str_value       varchar(400),
			json_value      json,
			ipv4_value      ipv4,
			ipv6_value      ipv6
		)
	`))
	if err := result.Err(); err != nil {
		panic(err)
	}

	result = conn.Exec(ctx, api.SqlTidy(`
		create tag table tag_simple(
			name            varchar(100) primary key, 
			time            datetime basetime, 
			value           double
		)
	`))
	if err := result.Err(); err != nil {
		panic(err)
	}

	result = conn.Exec(ctx, api.SqlTidy(`
		create table log_data(
		    time datetime,
			short_value short,
			ushort_value ushort,
			int_value integer,
			uint_value uinteger,
			long_value long,
			ulong_value ulong,
			double_value double,
			float_value float,
			str_value varchar(400),
			json_value json,
			ipv4_value ipv4,
			ipv6_value ipv6,
			text_value text,
			bin_value binary)
	`))
	if err := result.Err(); err != nil {
		panic(err)
	}

	// trace_log_level
	conn, err = s.machsvrDatabase.Connect(ctx, api.WithTrustUser("sys"))
	if err != nil {
		panic(err)
	}
	result = conn.Exec(ctx, "alter system set trace_log_level=1023")
	if result.Err() != nil {
		panic(result.Err())
	}
	conn.Close()

	// grpc server
	rpcSvr := machsvr.NewRPCServer(s.machsvrDatabase)

	buffer := 101024 * 1024
	s.grpcListener = bufconn.Listen(buffer)

	s.grpcServer = grpc.NewServer()
	machrpc.RegisterMachbaseServer(s.grpcServer, rpcSvr)

	go func() {
		s.grpcServer.Serve(s.grpcListener)
	}()

	resolver.SetDefaultScheme("passthrough")
	rpcConn, err := grpc.NewClient("bufconn",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return s.grpcListener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	} else {
		s.grpcClientConn = rpcConn
	}

	// machcli database
	if db, err := machcli.NewDatabase(&machcli.Config{
		Host: "127.0.0.1",
		Port: s.machsvrPort,
	}); err != nil {
		panic(err)
	} else {
		s.machcliDatabase = db
	}
}

func (s *Server) StopServer(m *testing.M) {
	s.onceStop.Do(func() {
		s.stopServer()
	})
}

func (s *Server) stopServer() {
	ctx := context.TODO()

	if err := s.machcliDatabase.Close(); err != nil {
		panic(err)
	}
	if err := s.grpcClientConn.Close(); err != nil {
		panic(err)
	}
	if err := s.grpcListener.Close(); err != nil {
		panic(err)
	}
	s.grpcServer.Stop()

	conn, err := s.machsvrDatabase.Connect(ctx, api.WithTrustUser("sys"))
	if err != nil {
		panic(err)
	}
	conn.Exec(ctx, "EXEC table_flush(tag_data)")
	conn.Exec(ctx, "EXEC table_flush(tag_simple)")
	conn.Exec(ctx, "EXEC table_flush(log_data)")
	result := conn.Exec(ctx, `drop table tag_data`)
	if err := result.Err(); err != nil {
		if err.Error() != "MACH-ERR 2031 Resource busy (TAG_DATA)." {
			panic(err)
		}
	}

	result = conn.Exec(ctx, `drop table log_data`)
	if err := result.Err(); err != nil {
		if err.Error() != "MACH-ERR 2031 Resource busy (LOG_DATA)." {
			panic(err)
		}
	}
	conn.Close()

	if err := s.machsvrDatabase.Shutdown(); err != nil {
		panic(err)
	}

	machsvr.Finalize()

	if err := os.RemoveAll(s.machsvrDataDir); err != nil {
		panic(err)
	}
}

func (s *Server) ClientConn() *grpc.ClientConn {
	return s.grpcClientConn
}

type TestingT interface {
	Log(args ...any)
	Fatal(args ...any)
	Fail()
	Fatalf(format string, args ...any)
}

func Database_machsvr(t TestingT) api.Database {
	var db api.Database
	if machsvr_db, err := machsvr.NewDatabase(); err != nil {
		t.Log("Error", err.Error())
		t.Fail()
	} else {
		db = machsvr_db
	}
	return db
}

func (s *Server) DatabaseSVR() api.Database {
	return s.machsvrDatabase
}

func (s *Server) DatabaseRPC() api.Database {
	rpcClient := machrpc.NewMachbaseClient(s.grpcClientConn)
	return machrpc.NewClientWithRPCClient(rpcClient)
}

func (s *Server) DatabaseCLI() api.Database {
	return s.machcliDatabase
}
