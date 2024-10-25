package machsvr_test

import (
	"context"
	_ "embed"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/api/machrpc"
	"github.com/machbase/neo-server/api/machsvr"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/test/bufconn"
)

var database *machsvr.Database
var connectOpts []api.ConnectOption
var rpcClient machrpc.MachbaseClient

type Server interface {
	Startup() error
	Shutdown() error
}

func TestMain(m *testing.M) {
	var err error
	var testMode string = "fog"
	var initOption = machsvr.OPT_SIGHANDLER_OFF
	var machPort = 5656

	homePath, err := filepath.Abs(filepath.Join(".", "tmp", "machbase"))
	if err != nil {
		panic(errors.Wrap(err, "abs tmp dir"))
	}
	if err := mkDirIfNotExists(filepath.Join(".", "tmp")); err != nil {
		panic(errors.Wrap(err, "create tmp dir"))
	}
	if err := mkDirIfNotExists(homePath); err != nil {
		panic(errors.Wrap(err, "machbase"))
	}
	if err := mkDirIfNotExists(filepath.Join(homePath, "conf")); err != nil {
		panic(errors.Wrap(err, "machbase conf"))
	}
	if err := mkDirIfNotExists(filepath.Join(homePath, "dbs")); err != nil {
		panic(errors.Wrap(err, "machbase dbs"))
	}
	if err := mkDirIfNotExists(filepath.Join(homePath, "trc")); err != nil {
		panic(errors.Wrap(err, "machbase trc"))
	}

	if len(machbase_conf) == 0 {
		panic("invalid machbase.conf")
	}

	if testMode == "fog" {
		tmp := string(machbase_conf)
		tmp = strings.Replace(tmp, "TAG_CACHE_MAX_MEMORY_SIZE = 33554432", "TAG_CACHE_MAX_MEMORY_SIZE = 536870912", 1)
		machbase_conf = []byte(tmp)
	}

	if runtime.GOOS == "windows" {
		// can not assign other ip address on Windows
		tmp := string(machbase_conf)
		tmp = strings.Replace(tmp, "BIND_IP_ADDRESS = 127.0.0.1", "BIND_IP_ADDRESS = 0.0.0.0", 1)
		machbase_conf = []byte(tmp)
	}

	confPath := filepath.Join(homePath, "conf", "machbase.conf")
	if err = os.WriteFile(confPath, machbase_conf, 0644); err != nil {
		panic(errors.Wrap(err, "machbase.conf"))
	}

	machsvr.Initialize(homePath, machPort, initOption)

	if machsvr.ExistsDatabase() {
		if err = machsvr.DestroyDatabase(); err != nil {
			panic(errors.Wrap(err, "destroy database"))
		}
	}
	if err = machsvr.CreateDatabase(); err != nil {
		panic(errors.Wrap(err, "create database"))
	}

	if db, err := machsvr.NewDatabase(); err != nil {
		panic(err)
	} else {
		database = db
	}
	if database == nil {
		panic("database instance nil")
	}
	if err := database.Startup(); err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	conn, err := database.Connect(ctx, api.WithTrustUser("sys"))
	if err != nil {
		panic(err)
	}
	result := conn.Exec(ctx, "alter system set trace_log_level=1023")
	if result.Err() != nil {
		panic(result.Err())
	}
	conn.Close()

	// grpc Server
	rpcSvr := machsvr.NewRPCServer(database)

	defer func() {
		e := recover()
		if e != nil {
			fmt.Printf("panic: %v\n", e)
		}
	}()

	buffer := 101024 * 1024
	lsnr := bufconn.Listen(buffer)

	baseServer := grpc.NewServer()
	baseServerWg := &sync.WaitGroup{}
	machrpc.RegisterMachbaseServer(baseServer, rpcSvr)
	baseServerWg.Add(1)
	go func() {
		baseServer.Serve(lsnr)
		baseServerWg.Done()
	}()

	resolver.SetDefaultScheme("passthrough")
	rpcConn, err := grpc.NewClient("bufconn",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lsnr.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	rpcClient = machrpc.NewMachbaseClient(rpcConn)

	createLogTable()
	createTagTable()
	createSimpleTagTable()

	m.Run()

	rpcConn.Close()
	if err := lsnr.Close(); err != nil {
		fmt.Printf("error closing listener: %v", err)
	}
	baseServer.Stop()
	baseServerWg.Wait()

	database.Shutdown()
	os.RemoveAll(filepath.Join(".", "tmp"))
}

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func StringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset)-1)]
	}
	return string(b)
}

func randomVarchar() string {
	rangeStart := 0
	rangeEnd := 10
	offset := rangeEnd - rangeStart
	randLength := seededRand.Intn(offset) + rangeStart

	charSet := "aAbBcCdDeEfFgGhHiIjJkKlLmMnNoOpPqQrRsStTuUvVwWxXyYzZ"
	randString := StringWithCharset(randLength, charSet)
	return randString
}

func mkDirIfNotExists(path string) error {
	_, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		if err := os.Mkdir(path, 0755); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func goid() int {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	id, err := strconv.Atoi(idField)
	if err != nil {
		panic(fmt.Sprintf("cannot get goroutine id: %v", err))
	}
	return id
}

func SqlTidy(sqlText string) string {
	lines := strings.Split(sqlText, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimSpace(ln)
	}
	return strings.TrimSpace(strings.Join(lines, " "))
}

//go:embed tc_main_test.conf
var machbase_conf []byte
