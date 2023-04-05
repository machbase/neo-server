package server

import (
	"bufio"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/md5"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/booter"
	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-engine/native"
	"github.com/machbase/neo-grpc/machrpc"
	"github.com/machbase/neo-grpc/mgmt"
	logging "github.com/machbase/neo-logging"
	"github.com/machbase/neo-server/mods"
	"github.com/machbase/neo-server/mods/service/ginutil"
	"github.com/machbase/neo-server/mods/service/httpsvr"
	"github.com/machbase/neo-server/mods/service/mqttsvr"
	"github.com/machbase/neo-server/mods/service/rpcsvr"
	"github.com/machbase/neo-server/mods/service/security"
	"github.com/machbase/neo-server/mods/service/sshsvr"
	"github.com/machbase/neo-server/mods/util"
	spi "github.com/machbase/neo-spi"
	"github.com/mbndr/figlet4go"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

func init() {

	booter.Register(
		"machbase.com/neo-server",
		func() *Config {
			return NewConfig()
		},
		func(conf *Config) (booter.Boot, error) {
			return NewServer(conf)
		},
	)

	defaultLogConf := logging.Config{
		Console:                     false,
		Filename:                    "-",
		Append:                      true,
		RotateSchedule:              "@midnight",
		MaxSize:                     10,
		MaxBackups:                  1,
		MaxAge:                      7,
		Compress:                    false,
		UTC:                         false,
		DefaultPrefixWidth:          10,
		DefaultEnableSourceLocation: false,
		DefaultLevel:                "TRACE",
	}

	booter.Register(
		"machbase.com/neo-logging",
		func() *logging.Config {
			conf := defaultLogConf
			return &conf
		},
		func(conf *logging.Config) (booter.Boot, error) {
			logging.Configure(conf)
			return &logging.Module{}, nil
		},
	)
}

type Config struct {
	DataDir        string
	PrefDir        string
	MachbasePreset MachbasePreset
	Machbase       MachbaseConfig
	StartupQueries []string
	AuthHandler    AuthHandlerConfig
	Shell          sshsvr.Config
	Grpc           GrpcConfig
	Http           HttpConfig
	Mqtt           mqttsvr.Config

	NoBanner bool

	EnableMachbaseSigHandler bool
}

type AuthHandlerConfig struct {
	Enabled bool
}

type GrpcConfig struct {
	Listeners      []string
	MaxRecvMsgSize int
	MaxSendMsgSize int
}

type HttpConfig struct {
	Listeners []string
	Handlers  []httpsvr.HandlerConfig

	EnableTokenAuth bool
}

type Server interface {
	booter.Boot
}

type svr struct {
	mgmt.ManagementServer

	conf  *Config
	log   logging.Log
	db    spi.Database
	grpcd *grpc.Server
	mgmtd *grpc.Server
	httpd *http.Server
	mqttd *mqttsvr.Server
	shsvr *sshsvr.MachShell

	certdir           string
	authHandler       AuthHandler
	authorizedKeysDir string

	cachedServerPrivateKey crypto.PrivateKey

	authorizedSshKeysLock sync.RWMutex
}

func NewConfig() *Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	conf := Config{
		DataDir: ".",
		PrefDir: filepath.Join(homeDir, ".config", "machbase"),
		Grpc: GrpcConfig{
			Listeners:      []string{"unix://./mach-grpc.sock"},
			MaxRecvMsgSize: 4,
			MaxSendMsgSize: 4,
		},
		Http: HttpConfig{
			Listeners: []string{},
			Handlers: []httpsvr.HandlerConfig{
				{Prefix: "/db", Handler: "machbase"},
			},
		},
		Mqtt: mqttsvr.Config{
			Listeners: []string{},
			Handlers: []mqttsvr.HandlerConfig{
				{Prefix: "db", Handler: "machbase"},
			},
			MaxMessageSizeLimit: 1024 * 1024,
		},
		Shell: sshsvr.Config{
			Listeners:   []string{},
			IdleTimeout: 2 * time.Minute,
		},
		NoBanner: false,
	}

	switch mach.Edition() {
	case "fog":
		conf.MachbasePreset = PresetFog
	case "edge":
		conf.MachbasePreset = PresetEdge
	default:
		sysCPU := runtime.NumCPU()
		conf.MachbasePreset = PresetNone
		if sysCPU < 8 {
			conf.MachbasePreset = PresetEdge
		} else {
			conf.MachbasePreset = PresetFog
		}
	}
	conf.Machbase = *DefaultMachbaseConfig(conf.MachbasePreset)
	return &conf
}

func NewServer(conf *Config) (Server, error) {
	return &svr{
		conf: conf,
	}, nil
}

func (s *svr) Start() error {
	s.log = logging.GetLog("neosvr")

	prefpath, err := filepath.Abs(s.conf.PrefDir)
	if err != nil {
		return errors.Wrap(err, "prefdir")
	}
	if err := util.MkDirIfNotExists(filepath.Dir(prefpath)); err != nil {
		return errors.Wrap(err, "prefdir")
	}
	if err := mkDirIfNotExists(prefpath); err != nil {
		return errors.Wrap(err, "prefdir")
	}
	s.certdir = filepath.Join(prefpath, "cert")
	if err := mkDirIfNotExistsMode(s.certdir, 0700); err != nil {
		return errors.Wrap(err, "prefdir cert")
	}
	if err := s.mkKeysIfNotExists(); err != nil {
		return errors.Wrap(err, "prefdir keys")
	}

	s.authorizedKeysDir = filepath.Join(s.certdir, "authorized_keys")
	if err := mkDirIfNotExistsMode(s.authorizedKeysDir, 0700); err != nil {
		return errors.Wrap(err, "authorized keys")
	}

	homepath, err := filepath.Abs(s.conf.DataDir)
	if err != nil {
		return errors.Wrap(err, "datadir")
	}
	if err := mkDirIfNotExists(homepath); err != nil {
		return errors.Wrap(err, "machbase_home")
	}
	if err := mkDirIfNotExists(filepath.Join(homepath, "conf")); err != nil {
		return errors.Wrap(err, "machbase conf")
	}
	if err := mkDirIfNotExists(filepath.Join(homepath, "dbs")); err != nil {
		return errors.Wrap(err, "machbase dbs")
	}
	if err := mkDirIfNotExists(filepath.Join(homepath, "trc")); err != nil {
		return errors.Wrap(err, "machbase trc")
	}

	s.authHandler = NewAuthenticator(s.ServerCertificatePath(), s.authorizedKeysDir, s.conf.AuthHandler.Enabled)

	s.log.Infof("apply machbase '%s' preset", s.conf.MachbasePreset)
	confpath := filepath.Join(homepath, "conf", "machbase.conf")
	if err := applyMachbaseConfig(confpath, &s.conf.Machbase); err != nil {
		return errors.Wrap(err, "machbase.conf")
	}

	initOption := mach.OPT_SIGHANDLER_DISABLE // default, it is required to shutdown by SIGTERM
	if s.conf.EnableMachbaseSigHandler {
		// internal use only, for debuging call stack
		initOption = mach.OPT_NONE
	}
	s.log.Infof("apply machbase init option: %d", initOption)
	if err := mach.InitializeOption(homepath, initOption); err != nil {
		return errors.Wrap(err, "initialize database failed")
	}
	if !mach.ExistsDatabase() {
		s.log.Info("create database")
		if err := mach.CreateDatabase(); err != nil {
			return errors.Wrap(err, "create database failed")
		}
	}

	s.db, err = spi.NewDatabase(mach.FactoryName)
	if err != nil {
		return errors.Wrap(err, "database instance failed")
	}
	if s.db == nil {
		return errors.New("database instance failed")
	}
	if mdb, ok := s.db.(spi.DatabaseServer); ok {
		ver := mods.GetVersion()
		if ver != nil {
			mach.BuildVersion.Major = int32(ver.Major)
			mach.BuildVersion.Minor = int32(ver.Minor)
			mach.BuildVersion.Patch = int32(ver.Patch)
			mach.BuildVersion.GitSHA = ver.GitSHA
			mach.BuildVersion.BuildTimestamp = mods.BuildTimestamp()
			mach.BuildVersion.BuildCompiler = mods.BuildCompiler()
		}
		if err := mdb.Startup(); err != nil {
			return errors.Wrap(err, "startup database")
		}
	}

	if !s.conf.NoBanner {
		// print banner if banner module is not configured
		s.log.Infof("\n%s", GenBanner())
	}

	for n, sqlText := range s.conf.StartupQueries {
		// ex) "alter system set trace_log_level=1023"
		result := s.db.Exec(sqlText)
		if result.Err() != nil {
			s.log.Warnf("StartupQueries[%d] %s %s", n, result.Err().Error(), sqlText)
			break
		} else {
			s.log.Debugf("StartupQueries[%d] %s", n, sqlText)
		}
	}

	// native port
	s.log.Infof("MACH Listen tcp://%s:%d", s.conf.Machbase.BIND_IP_ADDRESS, s.conf.Machbase.PORT_NO)

	// grpc server
	if len(s.conf.Grpc.Listeners) > 0 {
		machrpcSvr, err := rpcsvr.New(s.db, &rpcsvr.Config{})
		if err != nil {
			return errors.Wrap(err, "grpc handler")
		}
		// ingest gRPC options
		grpcOpt := []grpc.ServerOption{
			grpc.MaxRecvMsgSize(s.conf.Grpc.MaxRecvMsgSize * 1024 * 1024),
			grpc.MaxSendMsgSize(s.conf.Grpc.MaxSendMsgSize * 1024 * 1024),
			grpc.StatsHandler(machrpcSvr),
		}

		// create grpc server
		s.grpcd = grpc.NewServer(grpcOpt...)
		s.mgmtd = grpc.NewServer(grpcOpt...)
		// s.grpcd is serving only db service
		machrpc.RegisterMachbaseServer(s.grpcd, machrpcSvr)
		// s.mgmtd is serving general db service + mgmt service
		machrpc.RegisterMachbaseServer(s.mgmtd, machrpcSvr)
		mgmt.RegisterManagementServer(s.mgmtd, s)

		// listeners
		for _, listen := range s.conf.Grpc.Listeners {
			lsnr, err := makeListener(listen)
			if err != nil {
				return errors.Wrap(err, "cannot start with failed listener")
			}
			s.log.Infof("gRPC Listen %s", listen)

			if strings.HasPrefix(listen, "unix://") || strings.HasPrefix(listen, "tcp://127.0.0.1:") {
				// only gRPC via Unix Socket and loopback is allowed to perform mgmt service
				go s.mgmtd.Serve(lsnr)
			} else {
				go s.grpcd.Serve(lsnr)
			}
		}
	}

	// http server
	if len(s.conf.Http.Listeners) > 0 {
		machHttpSvr, err := httpsvr.New(s.db, &httpsvr.Config{Handlers: s.conf.Http.Handlers})
		if err != nil {
			return errors.Wrap(err, "http handler")
		}
		if s.conf.Http.EnableTokenAuth {
			s.log.Infof("HTTP token authentication enabled")
			machHttpSvr.SetAuthServer(s)
		} else {
			s.log.Infof("HTTP token authentication disabled")
		}

		gin.SetMode(gin.ReleaseMode)
		r := gin.New()
		r.Use(ginutil.RecoveryWithLogging(s.log))
		r.Use(ginutil.HttpLogger("http-log"))

		machHttpSvr.Route(r)

		s.httpd = &http.Server{}
		s.httpd.Handler = r

		for _, listen := range s.conf.Http.Listeners {
			lsnr, err := makeListener(listen)
			if err != nil {
				return errors.Wrap(err, "cannot start with failed listener")
			}
			s.log.Infof("HTTP Listen %s", listen)

			go s.httpd.Serve(lsnr)
		}
	}

	// mqtt server
	if len(s.conf.Mqtt.Listeners) > 0 {
		if s.conf.Mqtt.EnableTls {
			if len(s.conf.Mqtt.ServerCertPath) == 0 {
				s.conf.Mqtt.ServerCertPath = s.ServerCertificatePath()
			}
			if len(s.conf.Mqtt.ServerKeyPath) == 0 {
				s.conf.Mqtt.ServerKeyPath = s.ServerPrivateKeyPath()
			}
			s.log.Infof("MQTT TLS enabled")
		}

		s.mqttd = mqttsvr.New(s.db, &s.conf.Mqtt)
		s.mqttd.SetAuthServer(s)

		if s.conf.Mqtt.EnableTokenAuth && !s.conf.Mqtt.EnableTls {
			s.log.Infof("MQTT token authentication enabled")
		} else {
			s.log.Infof("MQTT token authentication disabled")
		}

		err := s.mqttd.Start()
		if err != nil {
			return errors.Wrap(err, "mqtt server")
		}
	}

	// ssh shell server
	if len(s.conf.Shell.Listeners) > 0 {
		s.conf.Shell.ServerKeyPath = s.ServerPrivateKeyPath()
		s.shsvr = sshsvr.New(s.db, &s.conf.Shell)
		s.shsvr.Server = s
		err := s.shsvr.Start()
		if err != nil {
			return errors.Wrap(err, "shell server")
		}
	}

	return nil
}

func (s *svr) Stop() {
	if s.shsvr != nil {
		s.shsvr.Stop()
	}
	if s.mqttd != nil {
		s.mqttd.Stop()
	}

	if s.httpd != nil {
		ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
		s.httpd.Shutdown(ctx)
		cancelFunc()
	}

	if s.grpcd != nil {
		s.grpcd.Stop()
	}
	if s.mgmtd != nil {
		s.mgmtd.Stop()
	}

	if mdb, ok := s.db.(spi.DatabaseServer); ok {
		if err := mdb.Shutdown(); err != nil {
			s.log.Warnf("db shutdown; %s", err.Error())
		}
	}
	mach.Finalize()

	s.log.Infof("shutdown.")
}

func GenBanner() string {
	options := figlet4go.NewRenderOptions()
	options.FontColor = []figlet4go.Color{
		figlet4go.ColorMagenta,
		figlet4go.ColorYellow,
		figlet4go.ColorCyan,
		figlet4go.ColorBlue,
	}
	fig := figlet4go.NewAsciiRender()
	machbase, _ := fig.Render("Machbase")
	logo, _ := fig.RenderOpts("neo", options)

	v := mods.GetVersion()

	lines := strings.Split(logo, "\n")
	lines[2] = lines[2] + fmt.Sprintf("  v%d.%d.%d (%s %s)", v.Major, v.Minor, v.Patch, v.GitSHA, mods.BuildTimestamp())
	lines[3] = lines[3] + fmt.Sprintf("  engine v%s (%s)", native.Version, native.GitHash)
	lines[4] = lines[4] + fmt.Sprintf("  %s", mods.EngineInfoString())
	return strings.TrimRight(strings.TrimRight(machbase, "\n")+strings.Join(lines, "\n"), "\n")
}

func (s *svr) ServerPrivateKeyPath() string {
	return filepath.Join(s.certdir, "machbase_key.pem")
}

func (s *svr) ServerPublicKeyPath() string {
	return filepath.Join(s.certdir, "machbase_pub.pem")
}

func (s *svr) ServerCertificatePath() string {
	return filepath.Join(s.certdir, "machbase_cert.pem")
}

func (s *svr) ServerCertificate() (*x509.Certificate, error) {
	buff, err := os.ReadFile(s.ServerCertificatePath())
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(buff)
	return x509.ParseCertificate(block.Bytes)
}

type AuthorizedSshKey struct {
	KeyType     string
	Key         string
	Fingerprint string
	Comment     string
}

const authorized_ssh_keys = "ssh_keys"

func (s *svr) GetAllAuthorizedSshKeys() ([]*AuthorizedSshKey, error) {
	s.authorizedSshKeysLock.RLock()
	defer s.authorizedSshKeysLock.RUnlock()

	list := []*AuthorizedSshKey{}

	file, err := os.OpenFile(filepath.Join(s.authorizedKeysDir, authorized_ssh_keys), os.O_RDONLY, 0600)
	if err != nil {
		// ignore error intended
		return list, nil
	}
	defer file.Close()
	reader := bufio.NewReader(file)

	var line string
	var parts = []string{}
	for {
		part, isPrefix, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return list, err
		}
		parts = append(parts, string(part))
		if isPrefix {
			continue
		}
		line = strings.Join(parts, "")
		parts = parts[:0]

		record := strings.Fields(line)
		if len(record) != 3 {
			continue
		}

		switch record[0] {
		case "ssh-rsa":
		case "ecdsa-sha2-nistp256":
		default:
			continue
		}

		hash := md5.Sum([]byte(record[1]))
		fingerprint := hex.EncodeToString(hash[:])
		list = append(list, &AuthorizedSshKey{KeyType: record[0], Key: record[1], Fingerprint: fingerprint, Comment: record[2]})
	}
	return list, nil
}

func (s *svr) AddAuthorizedSshKey(keyType string, key string, comment string) error {
	switch keyType {
	case "ssh-rsa":
	case "ecdsa-sha2-nistp256":
	default:
		return fmt.Errorf("key type '%s' is not supported", keyType)
	}
	if len(key) == 0 {
		return errors.New("invalid ssh key")
	}
	if len(comment) == 0 {
		return errors.New("ssh key name should not empty")
	}
	list, err := s.GetAllAuthorizedSshKeys()
	if err != nil {
		return err
	}
	for _, r := range list {
		if r.Key == key {
			return fmt.Errorf("ssh key already exists")
		}
	}

	s.authorizedSshKeysLock.Lock()
	defer s.authorizedSshKeysLock.Unlock()

	file, err := os.OpenFile(filepath.Join(s.authorizedKeysDir, authorized_ssh_keys), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(fmt.Sprintf("%s %s %s\n", keyType, key, comment))
	if err != nil {
		return err
	}
	return nil
}

func (s *svr) RemoveAuthorizedSshKey(fingerprint string) error {
	list, err := s.GetAllAuthorizedSshKeys()
	if err != nil {
		return err
	}
	found := false
	for i, r := range list {
		if r.Fingerprint == fingerprint {
			head := []*AuthorizedSshKey{}
			tail := []*AuthorizedSshKey{}
			if i > 0 {
				head = list[0:i]
			}
			if i+1 < len(list) {
				tail = list[i+1:]
			}
			list = append(head, tail...)
			found = true
			break
		}
	}

	if !found {
		return errors.New("ssh key doesn't exist")
	}

	s.authorizedSshKeysLock.Lock()
	defer s.authorizedSshKeysLock.Unlock()

	file, err := os.OpenFile(filepath.Join(s.authorizedKeysDir, authorized_ssh_keys), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, rec := range list {
		_, err := file.WriteString(fmt.Sprintf("%s %s %s\n", rec.KeyType, rec.Key, rec.Comment))
		if err != nil {
			return err
		}
	}
	return nil
}

// AuthorizedCertificate returns client's X.509 certificate, it returns nil if not found with the given id
func (s *svr) AuthorizedCertificate(id string) (*x509.Certificate, error) {
	path := filepath.Join(s.authorizedKeysDir, fmt.Sprintf("%s_cert.pem", id))
	nfo, err := os.Stat(path)
	if err != nil {
		return nil, os.ErrNotExist
	}
	if nfo.IsDir() || nfo.Size() == 0 {
		return nil, os.ErrExist
	}
	buff, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(buff)
	return x509.ParseCertificate(block.Bytes)
}

func (s *svr) IterateAuthorizedCertificates(cb func(id string) bool) error {
	if cb == nil {
		return nil
	}
	entries, err := os.ReadDir(s.authorizedKeysDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), "_cert.pem") || entry.IsDir() {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), "_cert.pem")
		flag := cb(id)
		if !flag {
			break
		}
	}
	return nil
}

func (s *svr) SetAuthorizedCertificate(id string, pemBytes []byte) error {
	path := filepath.Join(s.authorizedKeysDir, fmt.Sprintf("%s_cert.pem", id))
	return os.WriteFile(path, pemBytes, 00600)
}

func (s *svr) RemoveAuthorizedCertificate(id string) error {
	path := filepath.Join(s.authorizedKeysDir, fmt.Sprintf("%s_cert.pem", id))
	return os.Remove(path)
}

func (s *svr) ServerPublicKey() (any, error) {
	buff, err := os.ReadFile(s.ServerPublicKeyPath())
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(buff)
	priKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	switch p := priKey.(type) {
	case *rsa.PrivateKey:
		return p.PublicKey, nil
	case *ecdsa.PrivateKey:
		return p.PublicKey, nil
	default:
		return nil, fmt.Errorf("unsupported key type: %T", priKey)
	}
}

func (s *svr) ServerPrivateKey() (crypto.PrivateKey, error) {
	if s.cachedServerPrivateKey != nil {
		return s.cachedServerPrivateKey, nil
	}

	buff, err := os.ReadFile(s.ServerPrivateKeyPath())
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(buff)
	priKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	if priKey != nil {
		s.cachedServerPrivateKey = priKey
	}
	return priKey, nil
}

func (s *svr) mkKeysIfNotExists() error {
	priPath := s.ServerPrivateKeyPath()
	pubPath := s.ServerPublicKeyPath()
	certPath := s.ServerCertificatePath()

	needGen := false

	if _, err := os.Stat(priPath); err != nil {
		needGen = true
	}
	if _, err := os.Stat(pubPath); err != nil {
		needGen = true
	}
	if _, err := os.Stat(certPath); err != nil {
		needGen = true
	}

	if !needGen {
		return nil
	}

	ec := NewEllipticCurveP521()
	pri, pub, err := ec.GenerateKeys()
	if err != nil {
		return err
	}

	priPem, err := ec.EncodePrivate(pri)
	if err != nil {
		return errors.Wrap(err, "private key encoder")
	}

	pubPem, err := ec.EncodePublic(pub)
	if err != nil {
		return errors.Wrap(err, "public key encoder")
	}

	certBytes, err := GenerateServerCertificate(pri, pub)
	if err != nil {
		return errors.Wrap(err, "certificate encoder")
	}

	if err := os.WriteFile(priPath, []byte(priPem), 0600); err != nil {
		return errors.Wrap(err, "private key writer")
	}
	if err := os.WriteFile(pubPath, []byte(pubPem), 0644); err != nil {
		return errors.Wrap(err, "public key writer")
	}
	if err := os.WriteFile(certPath, certBytes, 0644); err != nil {
		return errors.Wrap(err, "certificate writer")
	}

	return nil
}

func mkDirIfNotExists(path string) error {
	return mkDirIfNotExistsMode(path, 0755)
}

func mkDirIfNotExistsMode(path string, mode fs.FileMode) error {
	_, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		if err := os.Mkdir(path, mode); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func makeListener(addr string) (net.Listener, error) {
	if strings.HasPrefix(addr, "unix://") {
		pwd, _ := os.Getwd()
		if strings.HasPrefix(addr, "unix://../") {
			addr = fmt.Sprintf("unix:///%s", filepath.Join(filepath.Dir(pwd), addr[len("unix://../"):]))
		} else if strings.HasPrefix(addr, "../") {
			addr = fmt.Sprintf("unix:///%s", filepath.Join(filepath.Dir(pwd), addr[len("../"):]))
		} else if strings.HasPrefix(addr, "unix://./") {
			addr = fmt.Sprintf("unix:///%s", filepath.Join(pwd, addr[len("unix://./"):]))
		} else if strings.HasPrefix(addr, "./") {
			addr = fmt.Sprintf("unix:///%s", filepath.Join(pwd, addr[len("./"):]))
		} else if strings.HasPrefix(addr, "/") {
			addr = fmt.Sprintf("unix://%s", addr)
		}
		path := addr[len("unix://"):]
		// delete existing .sock file
		if _, err := os.Stat(path); err == nil {
			os.Remove(path)
		}
		return net.Listen("unix", path)
	} else if strings.HasPrefix(addr, "tcp://") {
		return net.Listen("tcp", addr[len("tcp://"):])
	} else {
		return nil, fmt.Errorf("unuspported listen scheme %s", addr)
	}
}

// ////////////////////////////////
// implements neo-shell/server/sshsvr/Server interface
func (s *svr) GetGrpcAddresses() []string {
	return s.conf.Grpc.Listeners
}

func (s *svr) ValidateSshPublicKey(keyType string, key string) bool {
	list, err := s.GetAllAuthorizedSshKeys()
	if err != nil {
		s.log.Warnf("ssh public key", err.Error())
		return false
	}

	for _, rec := range list {
		if rec.KeyType == keyType && rec.Key == key {
			s.log.Debugf("ssh public key authorized: %s %s", rec.KeyType, rec.Fingerprint)
			return true
		}
	}
	return false
}

// ////////////////////////////////
// implements neo-shell/server/httpsvr/AuthServer interface
func (s *svr) ValidateClientToken(token string) (bool, error) {
	parts := strings.SplitN(token, ":", 3)
	if len(parts) == 0 {
		return false, errors.New("invalid token")
	}
	cliCert, err := s.AuthorizedCertificate(parts[0])
	if err != nil {
		return false, err
	}
	return VerifyClientToken(token, cliCert.PublicKey)
}

func (s *svr) ValidateClientCertificate(clientId string, certHash string) (bool, error) {
	cert, err := s.AuthorizedCertificate(clientId)
	if err != nil {
		if err == os.ErrNotExist {
			return false, fmt.Errorf("client-id %s not found", clientId)
		} else {
			return false, err
		}
	}

	hash, err := security.HashCertificate(cert)
	if err != nil {
		return false, err
	}
	return hash == certHash, nil
}
