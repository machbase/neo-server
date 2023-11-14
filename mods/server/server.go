package server

import (
	"bufio"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/md5"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-engine/native"
	"github.com/machbase/neo-grpc/mgmt"
	"github.com/machbase/neo-server/booter"
	"github.com/machbase/neo-server/mods"
	"github.com/machbase/neo-server/mods/bridge"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/leak"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/model"
	"github.com/machbase/neo-server/mods/scheduler"
	"github.com/machbase/neo-server/mods/service/grpcd"
	"github.com/machbase/neo-server/mods/service/httpd"
	"github.com/machbase/neo-server/mods/service/mqttd"
	"github.com/machbase/neo-server/mods/service/security"
	"github.com/machbase/neo-server/mods/service/sshd"
	"github.com/machbase/neo-server/mods/tql"
	"github.com/machbase/neo-server/mods/util"
	"github.com/machbase/neo-server/mods/util/snowflake"
	"github.com/machbase/neo-server/mods/util/ssfs"
	spi "github.com/machbase/neo-spi"
	"github.com/mbndr/figlet4go"
	"github.com/pkg/errors"
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
	FileDirs       []string
	MachbasePreset MachbasePreset
	Machbase       MachbaseConfig
	AuthHandler    AuthHandlerConfig
	Shell          ShellConfig
	Grpc           GrpcConfig
	Http           HttpConfig
	Mqtt           MqttConfig

	CreateDBQueries     []string // sql sentences
	CreateDBScriptFiles []string // file path
	StartupQueries      []string // sql sentences
	StartupScriptFiles  []string // file path

	NoBanner       bool
	ExperimentMode bool

	MachbaseInitOption mach.InitOption
	// deprecated, use mach.InitOption instead
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
	Handlers  []httpd.HandlerConfig
	WebDir    string

	EnableTokenAuth bool
	DebugMode       bool
}

type MqttConfig struct {
	Listeners []string
	Handlers  []mqttd.HandlerConfig

	EnableTokenAuth bool
	EnableTls       bool
	ServerCertPath  string
	ServerKeyPath   string

	MaxMessageSizeLimit int
}

type ShellConfig struct {
	Listeners     []string
	IdleTimeout   time.Duration
	ServerKeyPath string
}

type Server interface {
	booter.Boot
}

type svr struct {
	mgmt.UnimplementedManagementServer

	conf *Config
	log  logging.Log
	db   spi.Database

	mqttd mqttd.Service
	grpcd grpcd.Service
	httpd httpd.Service
	sshd  sshd.Service

	bridgeSvc bridge.Service
	schedSvc  scheduler.Service

	certdir           string
	authHandler       AuthHandler
	authorizedKeysDir string
	licenseFilePath   string
	licenseFileTime   time.Time
	databaseCreated   bool

	models model.Service

	cachedServerPrivateKey crypto.PrivateKey

	servicePorts     map[string][]*spi.ServicePort
	servicePortsLock sync.RWMutex

	authorizedSshKeysLock sync.RWMutex
	genSnowflake          *snowflake.Node
	snowflakes            []string
}

var PreferredPreset string = "auto"

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
			Handlers: []httpd.HandlerConfig{
				{Prefix: "/db", Handler: "machbase"},
			},
		},
		Mqtt: MqttConfig{
			Listeners: []string{},
			Handlers: []mqttd.HandlerConfig{
				{Prefix: "db", Handler: "machbase"},
			},
			MaxMessageSizeLimit: 1024 * 1024,
		},
		Shell: ShellConfig{
			Listeners:   []string{},
			IdleTimeout: 2 * time.Minute,
		},
		NoBanner: false,
	}

	switch strings.ToLower(PreferredPreset) {
	case "fog":
		conf.MachbasePreset = PresetFog
	case "edge":
		conf.MachbasePreset = PresetEdge
	default:
		sysCPU := runtime.NumCPU()
		if sysCPU <= 4 {
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
		conf:         conf,
		servicePorts: make(map[string][]*spi.ServicePort),
	}, nil
}

func (s *svr) Start() error {
	tick := time.Now()
	s.log = logging.GetLog("neosvr")

	s.genSnowflake, _ = snowflake.NewNode(0)
	for i := 0; i < 11; i++ {
		s.snowflakes = append(s.snowflakes, s.genSnowflake.Generate().Base64())
	}

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

	s.licenseFilePath = filepath.Join(prefpath, "license.dat")
	if stat, err := os.Stat(s.licenseFilePath); err == nil && !stat.IsDir() {
		s.licenseFileTime = stat.ModTime()
	}

	s.models = model.NewService(
		model.WithConfigDirPath(prefpath),
		model.WithExperimentModeProvider(func() bool { return s.conf.ExperimentMode }),
	)
	if err := s.models.Start(); err != nil {
		return err
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

	shouldInstallLicense := false
	if !s.licenseFileTime.IsZero() {
		stat, err := os.Stat(filepath.Join(homepath, "conf", "license.dat"))
		if err != nil {
			shouldInstallLicense = true
		} else if stat.ModTime().Sub(s.licenseFileTime) < 0 {
			shouldInstallLicense = true
		}
	}

	// port-check MACH
	if err := s.checkListenPort(fmt.Sprintf("tcp://%s:%d", s.conf.Machbase.BIND_IP_ADDRESS, s.conf.Machbase.PORT_NO)); err != nil {
		return errors.Wrap(err, "MACH port not available")
	} else {
		machPort := fmt.Sprintf("tcp://%s:%d", s.conf.Machbase.BIND_IP_ADDRESS, s.conf.Machbase.PORT_NO)
		s.AddServicePort("mach", machPort)
	}

	// port-check gRPC
	for _, addr := range s.conf.Grpc.Listeners {
		if err := s.checkListenPort(addr); err != nil {
			return errors.Wrap(err, "gRPC port not available")
		}
		s.AddServicePort("grpc", addr)
	}
	// port-check HTTP
	for _, addr := range s.conf.Http.Listeners {
		if err := s.checkListenPort(addr); err != nil {
			return errors.Wrap(err, "HTTP port not available")
		}
		s.AddServicePort("http", addr)
	}
	// port-check MQTT
	for _, addr := range s.conf.Mqtt.Listeners {
		if err := s.checkListenPort(addr); err != nil {
			return errors.Wrap(err, "MQTT port not available")
		}
		s.AddServicePort("mqtt", addr)
	}
	// port-check SSHD
	for _, addr := range s.conf.Shell.Listeners {
		if err := s.checkListenPort(addr); err != nil {
			return errors.Wrap(err, "SSHD port not available")
		}
		s.AddServicePort("shell", addr)
	}

	s.authHandler = NewAuthenticator(s.ServerCertificatePath(), s.authorizedKeysDir, s.conf.AuthHandler.Enabled)

	s.log.Infof("apply machbase '%s' preset", s.conf.MachbasePreset)
	confpath := filepath.Join(homepath, "conf", "machbase.conf")
	if _, err := os.Stat(confpath); err != nil {
		if err := applyMachbaseConfig(confpath, &s.conf.Machbase); err != nil {
			return errors.Wrap(err, "machbase.conf")
		}
	}

	// default is mach.OPT_SIGHANDLER_SIGINT_OFF, it is required to shutdown by SIGINT
	if s.conf.EnableMachbaseSigHandler {
		// internal use only, for debuging call stack raised inside the engine
		s.conf.MachbaseInitOption = mach.OPT_SIGHANDLER_ON
	}
	s.log.Infof("apply machbase init option: %d", s.conf.MachbaseInitOption)
	if err := mach.InitializeOption(homepath, s.conf.Machbase.PORT_NO, s.conf.MachbaseInitOption); err != nil {
		return errors.Wrap(err, "initialize database failed")
	}
	if !mach.ExistsDatabase() {
		s.log.Info("create database")
		if err := mach.CreateDatabase(); err != nil {
			return errors.Wrap(err, "create database failed")
		}
		s.databaseCreated = true
	}

	// leak detector
	leakDetector := leak.NewDetector(leak.Timer(10 * time.Second))
	// mach.DefaultDetective = leakDetector

	// create database instance
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
		mach.ServicePorts = s.servicePorts

		if err := mdb.Startup(); err != nil {
			return errors.Wrap(err, "startup database")
		}
	}

	if !s.conf.NoBanner {
		// print banner if banner module is not configured
		s.log.Infof("\n%s", GenBanner())
	}

	if shouldInstallLicense {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		conn, err := s.db.Connect(ctx, mach.WithTrustUser("sys"))
		if err != nil {
			s.log.Error("ERR", err.Error())
			return err
		}
		if err != nil {
			s.log.Error("ERR", err.Error())
			return err
		}
		if _, err = do.InstallLicenseFile(ctx, conn, s.licenseFilePath); err != nil {
			s.log.Warn("set license fail,", err.Error())
		} else {
			s.log.Info("set license success")
		}
		conn.Close()
		cancel()
	}

	if len(s.conf.CreateDBQueries) > 0 && s.databaseCreated {
		if err := s.runSqlScripts("CreateDBQueries", s.conf.CreateDBQueries); err != nil {
			s.log.Error("ERR", err.Error())
			return err
		}
	}

	if len(s.conf.CreateDBScriptFiles) > 0 && s.databaseCreated {
		for _, f := range s.conf.CreateDBScriptFiles {
			if f == "" {
				continue
			}
			if err := s.runSqlScriptFile("CreateDBScriptFiles", f); err != nil {
				return err
			}
		}
	}

	if len(s.conf.StartupQueries) > 0 {
		if err := s.runSqlScripts("StartupQueries", s.conf.StartupQueries); err != nil {
			s.log.Error("ERR", err.Error())
			return err
		}
	}

	if len(s.conf.StartupScriptFiles) > 0 {
		for _, f := range s.conf.StartupScriptFiles {
			if f == "" {
				continue
			}
			if err := s.runSqlScriptFile("StartupScriptFiles", f); err != nil {
				return err
			}
		}
	}

	serverFs, err := ssfs.NewServerSideFileSystem(s.conf.FileDirs)
	if err != nil {
		s.log.Warnf("Server filesystem, %s", err.Error())
		return errors.Wrap(err, "server side file system")
	}
	ssfs.SetDefault(serverFs)

	tqlLoader := tql.NewLoader(s.conf.FileDirs)

	s.bridgeSvc = bridge.NewService(
		bridge.WithProvider(s.models.BridgeProvider()),
	)

	s.schedSvc = scheduler.NewService(
		scheduler.WithVerbose(false),
		scheduler.WithProvider(s.models.ScheduleProvider()),
		scheduler.WithTqlLoader(tqlLoader),
		scheduler.WithDatabase(s.db),
	)

	// start bridge service
	if err := s.bridgeSvc.Start(); err != nil {
		return err
	}
	// start scheduler service
	if err := s.schedSvc.Start(); err != nil {
		return err
	}

	// native port
	s.log.Infof("MACH Listen tcp://%s:%d", s.conf.Machbase.BIND_IP_ADDRESS, s.conf.Machbase.PORT_NO)

	// grpc server
	if len(s.conf.Grpc.Listeners) > 0 {
		s.grpcd, err = grpcd.New(s.db,
			grpcd.OptionListenAddress(s.conf.Grpc.Listeners...),
			grpcd.OptionMaxRecvMsgSize(s.conf.Grpc.MaxRecvMsgSize*1024*1024),
			grpcd.OptionMaxSendMsgSize(s.conf.Grpc.MaxSendMsgSize*1024*1024),
			grpcd.OptionTlsCreds(s.ServerPrivateKeyPath(), s.ServerCertificatePath()),
			grpcd.OptionManagementServer(s),
			grpcd.OptionBridgeServer(s.bridgeSvc),
			grpcd.OptionScheduleServer(s.schedSvc),
			grpcd.OptionLeakDetector(leakDetector),
			grpcd.OptionAuthServer(s),
		)
		if err != nil {
			return errors.Wrap(err, "grpc server")
		}
		err := s.grpcd.Start()
		if err != nil {
			return errors.Wrap(err, "grpc server")
		}
	}

	enabledWebUI := false
	// http server
	if len(s.conf.Http.Listeners) > 0 {
		opts := []httpd.Option{
			httpd.OptionLicenseFilePath(s.licenseFilePath),
			httpd.OptionListenAddress(s.conf.Http.Listeners...),
			httpd.OptionAuthServer(s, s.conf.Http.EnableTokenAuth),
			httpd.OptionTqlLoader(tqlLoader),
			httpd.OptionServerSideFileSystem(serverFs),
			httpd.OptionDebugMode(s.conf.Http.DebugMode),
			httpd.OptionExperimentModeProvider(func() bool { return s.conf.ExperimentMode }),
			httpd.OptionWebShellProvider(s.models.ShellProvider()),
		}
		for _, h := range s.conf.Http.Handlers {
			if h.Handler == httpd.HandlerWeb {
				enabledWebUI = true
			}
			opts = append(opts, httpd.OptionHandler(h.Prefix, h.Handler))
		}
		shellPorts := s.servicePorts["shell"]
		shellAddrs := []string{}
		for _, sp := range shellPorts {
			shellAddrs = append(shellAddrs, sp.Address)
		}
		opts = append(opts, httpd.OptionNeoShellAddress(shellAddrs...))
		if s.conf.Http.WebDir != "" {
			stat, err := os.Stat(s.conf.Http.WebDir)
			if err != nil {
				return err
			}
			if !stat.IsDir() {
				return fmt.Errorf("web ui path is not a directory")
			}
			opts = append(opts, httpd.OptionWebDir(s.conf.Http.WebDir))
		}
		s.httpd, err = httpd.New(s.db, opts...)
		if err != nil {
			return errors.Wrap(err, "http server")
		}
		err = s.httpd.Start()
		if err != nil {
			return errors.Wrap(err, "http server")
		}
	}

	// mqtt server
	if len(s.conf.Mqtt.Listeners) > 0 {
		opts := []mqttd.Option{
			mqttd.OptionListenAddress(s.conf.Mqtt.Listeners...),
			mqttd.OptionMaxMessageSizeLimit(s.conf.Mqtt.MaxMessageSizeLimit),
			mqttd.OptionAuthServer(s, s.conf.Mqtt.EnableTokenAuth && !s.conf.Mqtt.EnableTls),
			mqttd.OptionTqlLoader(tqlLoader),
		}
		for _, h := range s.conf.Mqtt.Handlers {
			opts = append(opts, mqttd.OptionHandler(h.Prefix, h.Handler))
		}
		if s.conf.Mqtt.EnableTls {
			serverCert := s.conf.Mqtt.ServerCertPath
			if len(serverCert) == 0 {
				serverCert = s.ServerCertificatePath()
			}
			serverKey := s.conf.Mqtt.ServerKeyPath
			if len(serverKey) == 0 {
				serverKey = s.ServerPrivateKeyPath()
			}
			opts = append(opts, mqttd.OptionTls(serverCert, serverKey))
		}
		s.mqttd, err = mqttd.New(s.db, opts...)
		if err != nil {
			return errors.Wrap(err, "mqtt server")
		}
		err = s.mqttd.Start()
		if err != nil {
			return errors.Wrap(err, "mqtt server")
		}
	}

	// shells initialize
	s.initShellProvider()

	// ssh shell server
	if len(s.conf.Shell.Listeners) > 0 {
		s.sshd, err = sshd.New(s.db,
			sshd.OptionListenAddress(s.conf.Shell.Listeners...),
			sshd.OptionServerKeyPath(s.ServerPrivateKeyPath()),
			sshd.OptionIdleTimeout(s.conf.Shell.IdleTimeout),
			sshd.OptionAuthServer(s),
			sshd.OptionMotdMessage(fmt.Sprintf("machbase-neo %s %s", mods.VersionString(), mods.Edition())),
			sshd.OptionShellProvider(s.provideShellForSsh),
		)
		if err != nil {
			return errors.Wrap(err, "shell server")
		}
		err := s.sshd.Start()
		if err != nil {
			return errors.Wrap(err, "shell server")
		}
	}

	if aux, ok := s.db.(spi.DatabaseAux); ok && enabledWebUI {
		svcPorts, err := aux.GetServicePorts("http")
		if err != nil {
			return errors.Wrap(err, "service ports")
		}
		readyMsg := []string{}
		for _, p := range svcPorts {
			addr := strings.Replace(p.Address, "tcp://", "http://", 1)
			if strings.HasPrefix(addr, "http://127.0.0.1:") {
				addr = fmt.Sprintf("  > Local:   %s", addr)
			} else {
				addr = fmt.Sprintf("  > Network: %s", addr)
			}
			readyMsg = append(readyMsg, addr)
		}
		dbInitInfo := ""
		if s.databaseCreated {
			dbInitInfo = strings.Join([]string{
				fmt.Sprintf("\n\n >> New database created at '%s'", homepath),
				"\n >> Open web browser, login username 'sys' password 'manager'.",
			}, "\n")
		}
		s.log.Infof("%s\n\n  machbase-neo web running at:\n\n%s\n\n  ready in %s",
			dbInitInfo, strings.Join(readyMsg, "\n"), time.Since(tick).Round(time.Millisecond).String())
	} else {
		s.log.Infof("\n\n  machbase-neo ready in %s", time.Since(tick).Round(time.Millisecond).String())
	}

	return nil
}

func (s *svr) Stop() {
	if s.sshd != nil {
		s.sshd.Stop()
	}
	if s.mqttd != nil {
		s.mqttd.Stop()
	}
	if s.httpd != nil {
		s.httpd.Stop()
	}
	if s.grpcd != nil {
		s.grpcd.Stop()
	}
	if s.schedSvc != nil {
		s.schedSvc.Stop()
	}
	if s.bridgeSvc != nil {
		s.bridgeSvc.Stop()
	}
	if s.models != nil {
		s.models.Stop()
	}
	if mdb, ok := s.db.(spi.DatabaseServer); ok {
		if err := mdb.Shutdown(); err != nil {
			s.log.Warnf("db shutdown; %s", err.Error())
		}
	}
	mach.Finalize()
	s.log.Infof("shutdown.")
}

func (s *svr) AddServicePort(svc string, addr string) error {
	svc = strings.ToLower(svc)
	if strings.HasPrefix(addr, "tcp://") {
		host, port, err := net.SplitHostPort(strings.TrimPrefix(addr, "tcp://"))
		if err != nil {
			return errors.Wrapf(err, "%s host:port invalid syntax", svc)
		}
		lsnrHost := net.ParseIP(host)
		addrs := util.FindAllAddresses(lsnrHost)
		for _, addr := range addrs {
			lsnrPort := fmt.Sprintf("tcp://%s:%s", addr.IP.String(), port)
			s.servicePortsLock.Lock()
			lst := s.servicePorts[svc]
			lst = append(lst, &spi.ServicePort{Service: svc, Address: lsnrPort})
			s.servicePorts[svc] = lst
			s.servicePortsLock.Unlock()
		}
	} else {
		if strings.HasPrefix(addr, "unix://") && runtime.GOOS == "windows" {
			return nil
		}
		s.servicePortsLock.Lock()
		lst := s.servicePorts[svc]
		lst = append(lst, &spi.ServicePort{Service: svc, Address: addr})
		s.servicePorts[svc] = lst
		s.servicePortsLock.Unlock()
	}
	return nil
}

func (s *svr) ServicePort(svc string) []*spi.ServicePort {
	return s.servicePorts[strings.ToLower(svc)]
}

func GenBanner() string {
	options := figlet4go.NewRenderOptions()
	supportColor := true
	windowsVersion := ""
	if runtime.GOOS == "windows" {
		major, minor, build := util.GetWindowsVersion()
		windowsVersion = fmt.Sprintf("Windows %d.%d %d", major, minor, build)
		if major <= 10 && build < 14931 {
			supportColor = false
		}
	}
	if supportColor {
		options.FontColor = []figlet4go.Color{
			figlet4go.ColorMagenta,
			figlet4go.ColorYellow,
			figlet4go.ColorCyan,
			figlet4go.ColorBlue,
		}
	}
	fig := figlet4go.NewAsciiRender()
	machbase, _ := fig.Render("Machbase")
	logo, _ := fig.RenderOpts("neo", options)

	lines := strings.Split(logo, "\n")
	lines[2] = lines[2] + fmt.Sprintf("  %s", mods.VersionString())
	lines[3] = lines[3] + fmt.Sprintf("  engine v%s (%s)", native.Version, native.GitHash)
	lines[4] = lines[4] + fmt.Sprintf("  %s %s", mach.LinkInfo(), windowsVersion)
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

func (s *svr) checkListenPort(address string) error {
	if !strings.HasPrefix(address, "tcp://") {
		return nil
	}
	ln, err := net.Listen("tcp", strings.TrimPrefix(address, "tcp://"))
	if err != nil {
		return err
	}
	err = ln.Close()
	if err != nil {
		return err
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

//lint:ignore U1000 ignore unused for now
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
// implements neo-server/mods/service/httpsvr/AuthServer interface

var _ security.AuthServer = &svr{}

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

func (s *svr) ValidateUserPublicKey(user string, publicKey ssh.PublicKey) (bool, string, error) {
	list, err := s.GetAllAuthorizedSshKeys()
	if err != nil {
		s.log.Warnf("ssh %q public key", user, err.Error())
		return false, "", err
	}

	keyType := publicKey.Type()
	keyStr := base64.StdEncoding.EncodeToString(publicKey.Marshal())
	for _, rec := range list {
		if rec.KeyType == keyType && rec.Key == keyStr {
			s.log.Debugf("ssh %q public key authorized: %s %s", user, rec.KeyType, rec.Fingerprint)
			return true, s.snowflakes[rand.Intn(len(s.snowflakes))], nil
		}
	}
	return false, "", nil
}

func (s *svr) ValidateUserPassword(user string, password string) (bool, string, error) {
	if db, ok := s.db.(spi.DatabaseAuth); ok {
		passed, err := db.UserAuth(user, password)
		if err != nil {
			return false, "", err
		} else if !passed {
			return false, "", nil
		} else {
			return true, s.snowflakes[rand.Intn(len(s.snowflakes))], nil
		}
	} else {
		return false, "", errors.New("database does not support user-auth")
	}
}

func (s *svr) ValidateUserOtp(user string, otp string) (bool, error) {
	for _, n := range s.snowflakes {
		if otp == n {
			return true, nil
		}
	}
	return false, nil
}

func (s *svr) GenerateSnowflake() string {
	return s.genSnowflake.Generate().Base64()
}

func (s *svr) runSqlScriptFile(title string, path string) error {
	if path == "" {
		return nil
	}
	if stat, err := os.Stat(path); err != nil {
		s.log.Warnf("fail to read script %s, %q", err.Error(), path)
		return nil
	} else if stat.IsDir() {
		s.log.Warnf("fail to read script, dir %q", err.Error(), path)
		return nil
	}
	fd, err := os.Open(path)
	if err != nil {
		s.log.Warnf("fail to load script %s, %q", err.Error(), path)
		return nil
	}
	defer fd.Close()

	if lines, err := loadSqlScriptFile(fd); err != nil {
		s.log.Warnf("fail to load script file %s, %q", err.Error(), path)
		return nil
	} else {
		if err := s.runSqlScripts(title, lines); err != nil {
			return fmt.Errorf("fail to run script %s, %q", err.Error(), path)
		}
	}
	return nil
}

func (s *svr) runSqlScripts(title string, queries []string) error {
	if len(queries) == 0 {
		return nil
	}
	if len(queries) == 1 && queries[0] == "" {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, err := s.db.Connect(ctx, mach.WithTrustUser("sys"))
	if err != nil {
		s.log.Error("ERR", err.Error())
		return err
	}
	for n, sqlText := range queries {
		result := conn.Exec(ctx, sqlText)
		if result.Err() != nil {
			s.log.Warnf("%s[%d] %s %s", title, n, result.Err().Error(), sqlText)
			break
		} else {
			s.log.Debugf("%s[%d] %s", title, n, sqlText)
		}
	}
	conn.Close()
	return nil
}

func loadSqlScriptFile(in io.Reader) ([]string, error) {
	reader := bufio.NewReader(in)
	lineno := 0
	ret := []string{}

	buff := []byte{}
	lineBuff := []string{}
	for {
		lineno++
		part, isPrefix, err := reader.ReadLine()
		if err != nil {
			if err != io.EOF {
				return ret, fmt.Errorf("line %d %s", lineno, err.Error())
			}
			break
		}
		buff = append(buff, part...)
		if isPrefix {
			continue
		}
		subline := string(buff)
		buff = buff[:0]

		if strings.HasPrefix(subline, "#") || strings.HasPrefix(subline, "--") {
			continue
		}
		subline = strings.TrimSpace(subline)
		if len(subline) == 0 {
			// skip empty line
			continue
		}

		lineBuff = append(lineBuff, subline)
		if !strings.HasSuffix(subline, ";") {
			continue
		}

		line := strings.Join(lineBuff, " ")
		line = strings.TrimSuffix(line, ";")
		lineBuff = lineBuff[:0]

		ret = append(ret, line)
	}
	return ret, nil
}
