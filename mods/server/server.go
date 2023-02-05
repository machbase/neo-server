package server

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/booter"
	"github.com/machbase/cemlib/ginutil"
	"github.com/machbase/cemlib/logging"
	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-engine/native"
	"github.com/machbase/neo-grpc/machrpc"
	"github.com/machbase/neo-grpc/mgmt"
	"github.com/machbase/neo-server/mods"
	"github.com/machbase/neo-server/mods/httpsvr"
	"github.com/machbase/neo-server/mods/mqttsvr"
	"github.com/machbase/neo-server/mods/rpcsvr"
	shell "github.com/machbase/neo-server/mods/sshsvr"
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
	AuthHandler    AuthHandlerConfig
	Shell          shell.Config
	Grpc           GrpcConfig
	Http           HttpConfig
	Mqtt           mqttsvr.Config
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
}

type Server interface {
	booter.Boot
}

type svr struct {
	mgmt.ManagementServer

	conf  *Config
	log   logging.Log
	db    *mach.Database
	grpcd *grpc.Server
	mgmtd *grpc.Server
	httpd *http.Server
	mqttd *mqttsvr.Server
	shsvr *shell.MachShell

	certdir           string
	authHandler       AuthHandler
	authorizedKeysDir string
}

func NewConfig() *Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	conf := Config{
		DataDir: ".",
		PrefDir: filepath.Join(homeDir, ".config", ".machbase"),
		Grpc: GrpcConfig{
			Listeners:      []string{"unix://./mach.sock"},
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
		Shell: shell.Config{
			Listeners:   []string{},
			IdleTimeout: 2 * time.Minute,
		},
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

	if booter.GetDefinition("github.com/machbase/cemlib/banner") == nil {
		// print banner if banner module is not configured
		s.log.Infof("\n%s", GenBanner())
	}

	prefpath, err := filepath.Abs(s.conf.PrefDir)
	if err != nil {
		return errors.Wrap(err, "prefdir")
	}
	if err := mkDirIfNotExists(filepath.Dir(prefpath)); err != nil {
		return errors.Wrap(err, "prefdir")
	}
	if err := mkDirIfNotExists(prefpath); err != nil {
		return errors.Wrap(err, "prefdir")
	}
	s.certdir = filepath.Join(prefpath, "cert")
	if err := mkDirIfNotExists(s.certdir); err != nil {
		return errors.Wrap(err, "prefdir cert")
	}
	if err := s.mkKeysIfNotExists(); err != nil {
		return errors.Wrap(err, "prefdir keys")
	}

	s.authorizedKeysDir = filepath.Join(s.certdir, "authorized_keys")
	if err := mkDirIfNotExists(s.authorizedKeysDir); err != nil {
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

	if err := mach.Initialize(homepath); err != nil {
		return errors.Wrap(err, "initialize database")
	}
	if !mach.ExistsDatabase() {
		s.log.Info("create database")
		if err := mach.CreateDatabase(); err != nil {
			return errors.Wrap(err, "create database")
		}
	}

	s.db = mach.New()
	if s.db == nil {
		return errors.New("database instance failed")
	}

	if err := s.db.Startup(); err != nil {
		return errors.Wrap(err, "startup database")
	}

	result := s.db.Exec("alter system set trace_log_level=1023")
	if result.Err() != nil {
		return errors.Wrap(result.Err(), "alter log level")
	}

	// grpc server
	if len(s.conf.Grpc.Listeners) > 0 {
		machrpcSvr, err := rpcsvr.New(&rpcsvr.Config{})
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
		machHttpSvr, err := httpsvr.New(&httpsvr.Config{Handlers: s.conf.Http.Handlers})
		if err != nil {
			return errors.Wrap(err, "http handler")
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
		s.mqttd = mqttsvr.New(&s.conf.Mqtt)
		err := s.mqttd.Start()
		if err != nil {
			return errors.Wrap(err, "mqtt server")
		}
	}

	// ssh shell server
	if len(s.conf.Shell.Listeners) > 0 {
		s.conf.Shell.ServerKeyPath = s.ServerPrivateKeyPath()
		s.shsvr = shell.New(&s.conf.Shell)
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

	if err := s.db.Shutdown(); err != nil {
		s.log.Warnf("db shutdown; %s", err.Error())
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

func (s *svr) ServerPrivateKey() (any, error) {
	buff, err := os.ReadFile(s.ServerPrivateKeyPath())
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(buff)
	priKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, err
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

	if err := os.WriteFile(priPath, []byte(priPem), 0600); err != nil {
		return errors.Wrap(err, "private key writer")
	}
	if err := os.WriteFile(pubPath, []byte(pubPem), 0644); err != nil {
		return errors.Wrap(err, "public key writer")
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return errors.Wrap(err, "failed to generate serial number")
	}

	ca := &x509.Certificate{
		IsCA:                  true,
		BasicConstraintsValid: true,
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: "machbase-neo"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * time.Hour * 24 * 365),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, pub, pri)
	if err != nil {
		return err
	}
	certBuf := bytes.NewBuffer(nil)
	if err := pem.Encode(certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return err
	}
	if err := os.WriteFile(certPath, certBuf.Bytes(), 0644); err != nil {
		return err
	}

	return nil
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

func (s *svr) GetGrpcAddresses() []string {
	return s.conf.Grpc.Listeners
}
