package machsvr

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/machbase/neo-client/v2/api"
	"github.com/machbase/neo-server/v8/spi"
)

//go:embed test_server.conf
var defaultConfig string

type TestServer struct {
	machsvrDatabase *Database
	machsvrPort     int
	machsvrKey      crypto.PrivateKey
	dataDir         string
}

func (s *TestServer) checkListenPort() {
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

func (s *TestServer) StartServer(dataDir string) {
	// prepare
	homePath, err := filepath.Abs(filepath.Join(dataDir, "machbase"))
	if err != nil {
		panic(err)
	}
	confPath := filepath.Join(homePath, "conf", "machbase.conf")
	s.dataDir = dataDir

	os.RemoveAll(homePath)
	os.MkdirAll(homePath, 0755)
	os.MkdirAll(filepath.Join(homePath, "conf"), 0755)
	os.MkdirAll(filepath.Join(homePath, "trc"), 0755)
	os.MkdirAll(filepath.Join(homePath, "dbs"), 0755)
	os.WriteFile(confPath, []byte(defaultConfig), 0644)

	// available port
	s.checkListenPort()
	if err := Initialize(homePath, s.machsvrPort, OPT_SIGHANDLER_OFF); err != nil {
		panic(err)
	}

	if !ExistsDatabase() {
		if err := CreateDatabase(); err != nil {
			panic(err)
		}
	}

	// setup
	if db, err := NewDatabase(DatabaseOption{MaxOpenConn: -1, MaxOpenQuery: -1}); err != nil {
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
	time.Sleep(time.Millisecond * 2000)

	ctx := context.TODO()

	pair, err := api.GenerateAuthKeyPair()
	if err != nil {
		panic(err)
	}

	privPath, pubPath, err := pair.WriteFiles(homePath, "authkey_test")
	if err != nil {
		panic(err)
	}
	// just to verify the generated key file is valid
	privKey, err := api.LoadPrivateKeyFromFile(privPath)
	if err != nil {
		panic(err)
	}
	s.machsvrKey = privKey

	pubKeyContent, err := os.ReadFile(pubPath)
	if err != nil {
		panic(err)
	}

	// trace_log_level
	conn, err := s.machsvrDatabase.Connect(ctx, WithPassword("sys", "manager"))
	if err != nil {
		panic(err)
	}
	result := conn.Exec(ctx, "alter system set trace_log_level=1023")
	if result.Err() != nil {
		panic(result.Err())
	}
	result = conn.Exec(ctx,
		fmt.Sprintf("alter user sys add auth key (key='%s', valid_before='2100-01-01', comment='test key')",
			string(pubKeyContent)))
	if result.Err() != nil {
		panic(result.Err())
	}
	conn.Close()

	spi.SetDefaultKey("sys", privKey)

	// machgo database
	spi.SetDefaultDSN(map[string]string{
		"host":            "127.0.0.1",
		"port":            fmt.Sprintf("%d", s.machsvrPort),
		"statement_cache": "auto",
		"user":            "sys",
		"auth_key_file":   privPath,
	})
}

func (s *TestServer) StopServer() {
	if err := s.machsvrDatabase.Shutdown(); err != nil {
		panic(err)
	}
	Finalize()
	if err := os.RemoveAll(s.dataDir); err != nil {
		panic(err)
	}
}

func (s *TestServer) MachPort() int {
	return s.machsvrPort
}

func (s *TestServer) MachKey() crypto.PrivateKey {
	return s.machsvrKey
}

func (s *TestServer) MachKeyPEM() (string, error) {
	buff := &bytes.Buffer{}
	// encode private key to PEM format (PKCS#8)
	keyBytes, err := x509.MarshalPKCS8PrivateKey(s.machsvrKey)
	if err != nil {
		return "", err
	}

	err = pem.Encode(buff, &pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})
	if err != nil {
		return "", err
	}
	return buff.String(), nil
}

func (s *TestServer) MachSvr() *Database {
	return s.machsvrDatabase
}
