package testsuite

import (
	"context"
	_ "embed"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machcli"
	"github.com/machbase/neo-server/v8/api/machrpc"
	"github.com/machbase/neo-server/v8/api/machsvr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/test/bufconn"
)

type TestCase func(*testing.T, api.Database, context.Context)

func TestAll(t *testing.T, db api.Database, tests ...func(*testing.T)) {
	tt := []TestCase{
		UserAuth,
		Ping,
		License,
		DescribeTable,
		InsertAndQuery,
		InsertMeta,
		AppendTag,
		AppendTagNotExist,
		AppendTagPartial,
		ShowTables,
		ExistsTable,
		Indexes,
		Explain,     // machcli does not support explain
		ExplainFull, // machcli does not support explain
		Columns,
		LogTableExec,
		LogTableAppend,
		TagTableAppend,
		WatchLogTable,
		DemoUser,
	}

	ctx := context.TODO()
	db_name := strings.TrimPrefix(fmt.Sprintf("%T", db), "*")
	db_name = strings.SplitN(db_name, ".", 2)[0]
	for _, tc := range tt {
		name := runtime.FuncForPC(reflect.ValueOf(tc).Pointer()).Name()
		name = strings.TrimPrefix(name, "github.com/machbase/neo-server/v8/api/testsuite.")
		name = fmt.Sprintf("%s_%s", db_name, name)
		t.Run(name, func(t *testing.T) { tc(t, db, ctx) })
	}

	for _, tc := range tests {
		name := runtime.FuncForPC(reflect.ValueOf(tc).Pointer()).Name()
		name = filepath.Base(name)
		t.Run(name, tc)
	}
}

func DropTestTables(db api.Database) error {
	ctx := context.TODO()
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	if err != nil {
		return err
	}
	if r := conn.Exec(ctx, "DROP TABLE tag_data"); r.Err() != nil {
		return r.Err()
	}
	if r := conn.Exec(ctx, "DROP TABLE tag_simple"); r.Err() != nil {
		return r.Err()
	}
	if r := conn.Exec(ctx, "DROP TABLE log_data"); r.Err() != nil {
		return r.Err()
	}
	return conn.Close()
}

func CreateTestTables(db api.Database) error {
	// create test tables
	ctx := context.TODO()
	conn, _ := db.Connect(ctx, api.WithPassword("sys", "manager"))
	defer conn.Close()

	result := conn.Exec(ctx, api.SqlTidy(`
		create tag table if not exists tag_data(
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
		return err
	}

	result = conn.Exec(ctx, api.SqlTidy(`
		create tag table if not exists tag_simple(
			name            varchar(100) primary key, 
			time            datetime basetime, 
			value           double
		)
	`))
	if err := result.Err(); err != nil {
		return err
	}

	result = conn.Exec(ctx, api.SqlTidy(`
		create table if not exists log_data(
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
		return err
	}
	return nil
}

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
	grpcServerWg    sync.WaitGroup
	grpcListener    *bufconn.Listener
	grpcClientConn  *grpc.ClientConn
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
	if db, err := machsvr.NewDatabase(machsvr.DatabaseOption{MaxOpenConn: -1, MaxOpenQuery: -1}); err != nil {
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

	ctx := context.TODO()

	// trace_log_level
	conn, err := s.machsvrDatabase.Connect(ctx, api.WithTrustUser("sys"))
	if err != nil {
		panic(err)
	}
	result := conn.Exec(ctx, "alter system set trace_log_level=1023")
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

	s.grpcServerWg.Add(1)
	go func() {
		s.grpcServer.Serve(s.grpcListener)
		s.grpcServerWg.Done()
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
	if err := s.machcliDatabase.Close(); err != nil {
		panic(err)
	}
	if err := s.grpcClientConn.Close(); err != nil {
		panic(err)
	}
	if err := s.grpcListener.Close(); err != nil {
		panic(err)
	}
	s.grpcServer.GracefulStop()
	s.grpcServerWg.Wait()
	if err := s.machsvrDatabase.Shutdown(); err != nil {
		panic(err)
	}

	machsvr.Finalize()

	if err := os.RemoveAll(s.machsvrDataDir); err != nil {
		panic(err)
	}
}

func (s *Server) CreateTestTables() error {
	return CreateTestTables(s.machsvrDatabase)
}

func (s *Server) DropTestTables() error {
	return DropTestTables(s.machsvrDatabase)
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
	if machsvr_db, err := machsvr.NewDatabase(machsvr.DatabaseOption{MaxOpenConn: 10}); err != nil {
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
