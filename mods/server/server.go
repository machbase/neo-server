package server

import (
	"bufio"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/md5"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machcli"
	"github.com/machbase/neo-server/v8/api/machsvr"
	"github.com/machbase/neo-server/v8/api/mgmt"
	"github.com/machbase/neo-server/v8/booter"
	"github.com/machbase/neo-server/v8/mods"
	"github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/jsh"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/pkgs"
	"github.com/machbase/neo-server/v8/mods/scheduler"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/machbase/neo-server/v8/mods/util"
	_ "github.com/machbase/neo-server/v8/mods/util/jemalloc"
	"github.com/machbase/neo-server/v8/mods/util/snowflake"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
	"golang.org/x/crypto/ssh"
)

type Server struct {
	mgmt.UnimplementedManagementServer
	Config

	log   logging.Log
	navel *net.TCPConn
	grpcd *grpcd
	mqttd *mqttd
	httpd *httpd
	sshd  *sshd
	bakd  *backupd

	bridgeSvc bridge.Service
	schedSvc  scheduler.Service

	authHandler       AuthHandler
	authorizedKeysDir string
	licenseFilePath   string
	licenseFileTime   time.Time
	databaseCreated   bool

	pkgMgr *pkgs.PkgManager

	models model.Service

	cachedServerPrivateKey crypto.PrivateKey

	startupTime      time.Time
	servicePorts     map[string][]*model.ServicePort
	servicePortsLock sync.RWMutex

	certDirPath           string
	prefDirPath           string
	homeDirPath           string
	authorizedSshKeysLock sync.RWMutex
	genSnowflake          *snowflake.Node
	snowflakes            []string
}

var _ booter.Boot = (*Server)(nil)

var tqlLoader tql.Loader = tql.NewLoader()

func NewServer(conf *Config) (*Server, error) {
	if navelCord := os.Getenv(NAVEL_ENV); navelCord != "" {
		if port, err := strconv.ParseInt(navelCord, 10, 64); err == nil {
			conf.NavelCord = &NavelCordConfig{
				Port: int(port),
			}
		}
	}
	return &Server{
		Config:       *conf,
		servicePorts: make(map[string][]*model.ServicePort),
	}, nil
}

func Restore(dataDir string, backupDir string) error {
	if err := machsvr.Initialize(dataDir, 0, machsvr.OPT_SIGHANDLER_OFF); err != nil {
		return err
	}
	if err := machsvr.RestoreDatabase(backupDir); err != nil {
		return err
	}
	return nil
}

func (s *Server) Start() error {
	s.startupTime = time.Now()
	s.log = logging.GetLog("neosvr")

	s.genSnowflake, _ = snowflake.NewNode(0)
	for i := 0; i < 11; i++ {
		s.snowflakes = append(s.snowflakes, s.genSnowflake.Generate().Base64())
	}

	HeadOnly = strings.HasPrefix(s.DataDir, "machbase://")
	if Headless {
		if HeadOnly {
			return fmt.Errorf("headless mode is not possible alongside head-only mode, %s", s.DataDir)
		}
		return s.StartHeadless()
	}

	if err := s.preparePrefDir(); err != nil {
		return err
	}

	if err := s.startModelService(); err != nil {
		return err
	}

	if err := s.prepareHomeDir(); err != nil {
		return err
	}

	if err := s.preparePorts(); err != nil {
		return err
	}

	if s.Jwt.AtDuration > 0 && s.Jwt.RtDuration > 0 {
		JwtConfigure(&s.Jwt)
	}

	s.authHandler = NewAuthenticator(s.ServerCertificatePath(), s.authorizedKeysDir, s.AuthHandler.Enabled)

	if HeadOnly {
		if err := s.startMachbaseCli(); err != nil {
			return err
		}
	} else {
		if err := s.startMachbaseSvr(); err != nil {
			return err
		}
	}

	if !s.NoBanner {
		s.log.Infof("\n%s", mods.GenBanner())
	}

	s.checkMaxOpenConnections()

	if !HeadOnly {
		if err := s.checkAndInstallLicense(); err != nil {
			return err
		}

		if err := s.runInitScripts(); err != nil {
			return err
		}

		// backup service
		if err := s.startBackupService(); err != nil {
			return err
		}
	}

	// server side file system
	if err := s.startServerFileSystem(); err != nil {
		return err
	}

	// bridge and scheduler
	if err := s.startBridgeAndSchedulerService(); err != nil {
		return err
	}

	// native port
	if !HeadOnly {
		s.log.Infof("MACH Listen tcp://%s:%d", s.Machbase.BIND_IP_ADDRESS, s.Machbase.PORT_NO)
	}

	// grpc server
	if err := s.startGrpcServer(); err != nil {
		return fmt.Errorf("grpc server: %w", err)
	}

	// mqtt server
	if err := s.startMqttServer(); err != nil {
		return fmt.Errorf("mqtt server: %w", err)
	}

	// package manager
	if err := s.initPackageManager(); err != nil {
		return fmt.Errorf("package manager: %w", err)
	}

	// http server
	if err := s.startHttpServer(); err != nil {
		return fmt.Errorf("http server: %w", err)
	}

	// shells initialize
	if err := s.initShellProvider(); err != nil {
		return fmt.Errorf("shell provider: %w", err)
	}

	// ssh shell server
	if err := s.startSshServer(); err != nil {
		return fmt.Errorf("ssh server: %w", err)
	}

	// ready message
	svcPorts, err := s.getServicePorts("http")
	if err != nil {
		return fmt.Errorf("service ports, %s", err.Error())
	}
	readyMsg := []string{}
	for _, p := range svcPorts {
		readyMsg = append(readyMsg, representativePort(p.Address))
	}
	dbInitInfo := ""
	if s.databaseCreated {
		dbInitInfo = strings.Join([]string{
			fmt.Sprintf("\n\n >> New database created at '%s'", s.homeDirPath),
			"\n >> Open web browser, login username 'sys' password 'manager'.",
		}, "\n")
	}
	s.log.Infof("%s\n\n  machbase-neo web running at:\n\n%s\n\n  ready in %s",
		dbInitInfo, strings.Join(readyMsg, "\n"), time.Since(s.startupTime).Round(time.Millisecond).String())

	// jsh service
	if svcList, err := jsh.ReadServices(); err != nil {
		if err != os.ErrNotExist {
			s.log.Warn("JshServices read failed.", err.Error())
		}
	} else {
		svcList.Update(func(sc *jsh.ServiceConfig, act string, err error) {
			if err != nil {
				s.log.Info("JshService", act, err.Error())
			} else {
				s.log.Info("JshService", act)
			}
		})
	}
	util.AddShutdownHook(func() {
		jsh.ShutdownAll()
	})

	// pkgs
	s.pkgMgr.Start()

	// navel cord
	if s.NavelCord != nil {
		s.StartNavelCord()
	}

	// metrics
	startServerMetrics(s)

	return nil
}

func (s *Server) StartHeadless() error {
	if err := s.preparePrefDir(); err != nil {
		return err
	}

	if err := s.prepareHomeDir(); err != nil {
		return err
	}

	if err := s.startMachbaseSvr(); err != nil {
		return err
	}

	if !s.NoBanner {
		s.log.Infof("\n%s", mods.GenBanner())
	}

	s.checkMaxOpenConnections()

	if err := s.checkAndInstallLicense(); err != nil {
		return err
	}

	// native port
	s.log.Infof("MACH Listen tcp://%s:%d", s.Machbase.BIND_IP_ADDRESS, s.Machbase.PORT_NO)

	dbInitInfo := ""
	if s.databaseCreated {
		dbInitInfo = fmt.Sprintf("\n\n >> New database created at '%s'", s.homeDirPath)
	}
	s.log.Infof("%s\n\n  ready in %s", dbInitInfo, time.Since(s.startupTime).Round(time.Millisecond).String())
	return nil
}

func representativePort(addr string) string {
	addr = strings.Replace(addr, "tcp://", "http://", 1)
	if strings.HasPrefix(addr, "http://127.0.0.1:") {
		addr = fmt.Sprintf("  > Local:   %s", addr)
	} else if strings.HasPrefix(addr, "unix://") {
		addr = fmt.Sprintf("  > Unix:    %s", filepath.FromSlash(strings.TrimPrefix(addr, "unix://")))
	} else {
		addr = fmt.Sprintf("  > Network: %s", addr)
	}
	return addr
}

func (s *Server) Stop() {
	util.RunShutdownHooks()

	if db, ok := api.Default().(*machsvr.Database); ok {
		if err := db.Shutdown(); err != nil {
			s.log.Warnf("db shutdown; %s", err.Error())
		}
		machsvr.Finalize()
	} else if db, ok := api.Default().(*machcli.Database); ok {
		if err := db.Close(); err != nil {
			s.log.Warnf("db close; %s", err.Error())
		}
	}
	s.log.Infof("shutdown.")
}

func (s *Server) startMachbaseSvr() error {
	s.log.Infof("apply machbase '%s' preset", s.MachbasePreset)
	confFilePath := filepath.Join(s.homeDirPath, "conf", "machbase.conf")
	if _, err := os.Stat(confFilePath); err != nil {
		if err := applyMachbaseConfig(confFilePath, &s.Machbase); err != nil {
			return fmt.Errorf("machbase.conf, %s", err.Error())
		}
	} else if rewrite, err := s.checkRewriteMachbaseConf(confFilePath); err != nil {
		return err
	} else if rewrite {
		if err := s.rewriteMachbaseConf(confFilePath); err != nil {
			return fmt.Errorf("machbase.conf, %s", err.Error())
		}
	}

	s.log.Infof("apply machbase init option: %d", s.MachbaseInitOption)
	if err := machsvr.Initialize(s.homeDirPath, s.Machbase.PORT_NO, s.MachbaseInitOption); err != nil {
		return fmt.Errorf("initialize database failed, %s", err.Error())
	}
	if !machsvr.ExistsDatabase() {
		s.log.Info("create database")
		if err := machsvr.CreateDatabase(); err != nil {
			return fmt.Errorf("create database failed, %s", err.Error())
		}
		s.databaseCreated = true
	}

	// create database instance
	db, err := machsvr.NewDatabase(machsvr.DatabaseOption{
		MaxOpenConn:        s.Config.MaxOpenConn,
		MaxOpenConnFactor:  s.Config.MaxOpenConnFactor,
		MaxOpenQuery:       s.Config.MaxOpenQuery,
		MaxOpenQueryFactor: s.Config.MaxOpenQueryFactor,
		MaxWorkerPoolSize:  s.Config.MaxPoolSize,
	})
	if err != nil {
		return fmt.Errorf("database instance failed, %s", err.Error())
	}
	if db == nil {
		return errors.New("database instance failed")
	}
	if err := db.Startup(); err != nil {
		return fmt.Errorf("startup database, %s", err.Error())
	}
	api.SetDefault(db)
	api.StartAppendWorkers()
	util.AddShutdownHook(func() {
		api.StopAppendWorkers()
	})
	return nil
}

func (s *Server) startMachbaseCli() error {
	addr, err := url.Parse(s.DataDir)
	if err != nil {
		return fmt.Errorf("invalid --data address for headless mode, %s", s.DataDir)
	}
	if addr.Scheme != "machbase" {
		return fmt.Errorf("invalid --data address for headless mode, %s", s.DataDir)
	}
	if addr.Host == "" {
		return fmt.Errorf("invalid --data address for headless mode, %s", s.DataDir)
	}
	if addr.Port() == "" {
		return fmt.Errorf("invalid --data address for headless mode, %s", s.DataDir)
	}
	host := strings.TrimSuffix(addr.Host, ":"+addr.Port())
	port := 0
	if p, err := strconv.ParseInt(addr.Port(), 10, 32); err == nil {
		port = int(p)
	} else {
		return fmt.Errorf("invalid --data address for headless mode, %s", s.DataDir)
	}

	db, err := machcli.NewDatabase(&machcli.Config{
		Host:               host,
		Port:               port,
		MaxOpenConn:        s.Config.MaxOpenConn,
		MaxOpenConnFactor:  s.Config.MaxOpenConnFactor,
		MaxOpenQuery:       s.Config.MaxOpenQuery,
		MaxOpenQueryFactor: s.Config.MaxOpenQueryFactor,
		TrustUsers:         map[string]string{"sys": "manager"},
	})
	if err != nil {
		return err
	}

	if addr.User != nil {
		user := addr.User.Username()
		pass, set := addr.User.Password()
		if set {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ok, _, err := db.UserAuth(ctx, user, pass)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("head-only mode user auth failed")
			}
			db.SetTrustUser(user, pass)
		}
	}
	api.SetDefault(db)
	api.StartAppendWorkers()
	util.AddShutdownHook(func() {
		api.StopAppendWorkers()
	})
	return nil
}

func (s *Server) AddServicePort(svc string, addr string) error {
	svc = strings.ToLower(svc)
	if strings.HasPrefix(addr, "tcp://") {
		host, port, err := net.SplitHostPort(strings.TrimPrefix(addr, "tcp://"))
		if err != nil {
			return fmt.Errorf("%s host:port invalid syntax, %s", svc, err.Error())
		}
		lsnrHost := net.ParseIP(host)
		if lsnrHost.Equal(net.IPv4zero) || lsnrHost.Equal(net.IPv6zero) {
			addrs := util.FindAllAddresses(lsnrHost)
			for _, addr := range addrs {
				lsnrPort := fmt.Sprintf("tcp://%s:%s", addr.IP.String(), port)
				s.servicePortsLock.Lock()
				lst := s.servicePorts[svc]
				lst = append(lst, &model.ServicePort{Service: svc, Address: lsnrPort})
				s.servicePorts[svc] = lst
				s.servicePortsLock.Unlock()
			}
		} else {
			s.servicePortsLock.Lock()
			lst := s.servicePorts[svc]
			lst = append(lst, &model.ServicePort{Service: svc, Address: addr})
			s.servicePorts[svc] = lst
			s.servicePortsLock.Unlock()
		}
	} else {
		s.servicePortsLock.Lock()
		lst := s.servicePorts[svc]
		lst = append(lst, &model.ServicePort{Service: svc, Address: addr})
		s.servicePorts[svc] = lst
		s.servicePortsLock.Unlock()
	}
	return nil
}

func (s *Server) preparePorts() error {
	// port-check MACH
	if !HeadOnly {
		if err := s.checkListenPort(fmt.Sprintf("tcp://%s:%d", s.Machbase.BIND_IP_ADDRESS, s.Machbase.PORT_NO)); err != nil {
			return fmt.Errorf("MACH port not available, %s", err.Error())
		} else {
			machPort := fmt.Sprintf("tcp://%s:%d", s.Machbase.BIND_IP_ADDRESS, s.Machbase.PORT_NO)
			s.AddServicePort("mach", machPort)
		}
	}
	// port-check gRPC
	for _, addr := range s.Grpc.Listeners {
		if err := s.checkListenPort(addr); err != nil {
			return fmt.Errorf("gRPC port not available, %s", err.Error())
		}
		s.AddServicePort("grpc", addr)
	}
	// port-check HTTP
	for _, addr := range s.Http.Listeners {
		if err := s.checkListenPort(addr); err != nil {
			return fmt.Errorf("HTTP port not available, %s", err.Error())
		}
		s.AddServicePort("http", addr)
	}
	// port-check MQTT
	for _, addr := range s.Mqtt.Listeners {
		if err := s.checkListenPort(addr); err != nil {
			return fmt.Errorf("MQTT port not available, %s", err.Error())
		}
		s.AddServicePort("mqtt", addr)
	}
	// port-check SSHD
	for _, addr := range s.Shell.Listeners {
		if err := s.checkListenPort(addr); err != nil {
			return fmt.Errorf("SSHD port not available, %s", err.Error())
		}
		s.AddServicePort("shell", addr)
	}
	return nil
}

func (s *Server) preparePrefDir() error {
	if path, err := filepath.Abs(s.PrefDir); err != nil {
		return fmt.Errorf("prefdir: %s", err.Error())
	} else {
		s.prefDirPath = path
	}
	if err := util.MkDirIfNotExists(filepath.Dir(s.prefDirPath)); err != nil {
		return fmt.Errorf("prefdir: %s", err.Error())
	}
	if err := mkDirIfNotExists(s.prefDirPath); err != nil {
		return fmt.Errorf("prefdir: %s", err.Error())
	}
	s.certDirPath = filepath.Join(s.prefDirPath, "cert")
	if err := mkDirIfNotExistsMode(s.certDirPath, 0700); err != nil {
		return fmt.Errorf("prefdir cert: %s", err.Error())
	}
	if err := s.mkKeysIfNotExists(); err != nil {
		return fmt.Errorf("prefdir keys: %s", err.Error())
	}

	s.authorizedKeysDir = filepath.Join(s.certDirPath, "authorized_keys")
	if err := mkDirIfNotExistsMode(s.authorizedKeysDir, 0700); err != nil {
		return fmt.Errorf("authorized keys: %s", err.Error())
	}

	s.licenseFilePath = filepath.Join(s.prefDirPath, "license.dat")
	if stat, err := os.Stat(s.licenseFilePath); err == nil && !stat.IsDir() {
		s.licenseFileTime = stat.ModTime()
	}
	return nil
}

func (s *Server) prepareHomeDir() error {
	if HeadOnly {
		bin, err := os.Executable()
		if err != nil {
			return err
		}
		if path, err := filepath.Abs(filepath.Dir(bin)); err != nil {
			return err
		} else {
			s.homeDirPath = filepath.Join(path, "machbase_home")
		}
	} else {
		if path, err := filepath.Abs(s.DataDir); err != nil {
			return fmt.Errorf("datadir, %s", err.Error())
		} else {
			s.homeDirPath = path
		}
	}

	if err := mkDirIfNotExists(s.homeDirPath); err != nil {
		return fmt.Errorf("machbase_home, %s", err.Error())
	}

	if !HeadOnly {
		if err := mkDirIfNotExists(filepath.Join(s.homeDirPath, "conf")); err != nil {
			return fmt.Errorf("machbase conf, %s", err.Error())
		}
		if err := mkDirIfNotExists(filepath.Join(s.homeDirPath, "dbs")); err != nil {
			return fmt.Errorf("machbase dbs, %s", err.Error())
		}
		if err := mkDirIfNotExists(filepath.Join(s.homeDirPath, "trc")); err != nil {
			return fmt.Errorf("machbase trc, %s", err.Error())
		}
	}
	return nil
}

func (s *Server) startModelService() error {
	s.models = model.NewService(
		model.WithConfigDirPath(s.prefDirPath),
		model.WithExperimentModeProvider(func() bool { return s.ExperimentMode }),
	)
	if err := s.models.Start(); err != nil {
		return err
	}
	util.AddShutdownHook(func() { s.models.Stop() })
	return nil
}

func (s *Server) startBackupService() error {
	if s.BackupDir != "" {
		if backupDirAbs, err := filepath.Abs(s.BackupDir); err != nil {
			s.log.Errorf("Can not decide absolute path for backup dir, %s", err.Error())
		} else {
			s.bakd = NewBackupd(
				WithBackupdBaseDir(backupDirAbs),
				WithBackupdDatabase(api.Default()),
			)
		}
	}
	if s.bakd != nil {
		if err := s.bakd.Start(); err != nil {
			return err
		}
		util.AddShutdownHook(func() { s.bakd.Stop() })
	}
	return nil
}

func (s *Server) startServerFileSystem() error {
	serverFs, err := ssfs.NewServerSideFileSystem(s.FileDirs)
	if err != nil {
		s.log.Warnf("Server filesystem, %s", err.Error())
		return fmt.Errorf("server side file system, %s", err.Error())
	}
	ssfs.SetDefault(serverFs)
	return nil
}

func (s *Server) checkMaxOpenConnections() {
	if db, ok := api.Default().(*machsvr.Database); ok {
		strOrUnlimited := func(n int) string {
			if n < 0 {
				return "unlimited"
			}
			return strconv.Itoa(n)
		}
		maxConn, _ := db.MaxOpenConn()
		maxQuery, _ := db.MaxOpenQuery()
		s.log.Info("MACH MaxOpenConn:", strOrUnlimited(maxConn))
		s.log.Info("MACH MaxOpenQuery:", strOrUnlimited(maxQuery))
	}
}

func (s *Server) checkAndInstallLicense() error {
	if !s.licenseFileTime.IsZero() {
		stat, err := os.Stat(filepath.Join(s.homeDirPath, "conf", "license.dat"))
		if err != nil || stat.ModTime().Sub(s.licenseFileTime) < 0 {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			conn, err := api.Default().Connect(ctx, api.WithTrustUser("sys"))
			if err != nil {
				s.log.Error("ERR", err.Error())
				return err
			}
			if _, err = api.InstallLicenseFile(ctx, conn, s.licenseFilePath); err != nil {
				s.log.Warn("set license fail,", err.Error())
			} else {
				s.log.Info("set license success")
			}
			if err := conn.Close(); err != nil {
				s.log.Warn("ERR", err.Error())
			}
		}
	}
	return nil
}

func (s *Server) runInitScripts() error {
	if len(s.CreateDBQueries) > 0 && s.databaseCreated {
		if err := s.runSqlScripts("CreateDBQueries", s.CreateDBQueries); err != nil {
			s.log.Error("ERR", err.Error())
			return err
		}
	}

	if len(s.CreateDBScriptFiles) > 0 && s.databaseCreated {
		for _, f := range s.CreateDBScriptFiles {
			if f == "" {
				continue
			}
			if err := s.runSqlScriptFile("CreateDBScriptFiles", f); err != nil {
				return err
			}
		}
	}

	if len(s.StartupQueries) > 0 {
		if err := s.runSqlScripts("StartupQueries", s.StartupQueries); err != nil {
			s.log.Error("ERR", err.Error())
			return err
		}
	}

	if len(s.StartupScriptFiles) > 0 {
		for _, f := range s.StartupScriptFiles {
			if f == "" {
				continue
			}
			if err := s.runSqlScriptFile("StartupScriptFiles", f); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Server) startGrpcServer() error {
	if len(s.Grpc.Listeners) == 0 {
		return nil
	}
	machRpcSvr := machsvr.NewRPCServer(api.Default(),
		machsvr.WithLogger(logging.Wrap(s.log, nil)),
		machsvr.WithAuthProvider(s),
	)
	if grpcd, err := NewGrpc(machRpcSvr,
		WithGrpcListenAddress(s.Grpc.Listeners...),
		WithGrpcMaxRecvMsgSize(s.Grpc.MaxRecvMsgSize*1024*1024),
		WithGrpcMaxSendMsgSize(s.Grpc.MaxSendMsgSize*1024*1024),
		WithGrpcTlsCreds(s.ServerPrivateKeyPath(), s.ServerCertificatePath()),
		WithGrpcManagementServer(s),
		WithGrpcBridgeServer(s.bridgeSvc),
		WithGrpcScheduleServer(s.schedSvc),
		WithGrpcServerInsecure(s.Grpc.Insecure),
	); err != nil {
		return err
	} else {
		s.grpcd = grpcd
	}
	if err := s.grpcd.Start(); err != nil {
		return err
	}
	util.AddShutdownHook(func() { s.grpcd.Stop() })

	tql.SetGrpcAddresses(s.Grpc.Listeners)

	tql.StartCache(tql.CacheOption{MaxCapacity: 500})
	util.AddShutdownHook(func() { tql.StopCache() })

	return nil
}

func (s *Server) startBridgeAndSchedulerService() error {
	s.schedSvc = scheduler.NewService(
		scheduler.WithVerbose(false),
		scheduler.WithProvider(s.models.ScheduleProvider()),
		scheduler.WithTqlLoader(tqlLoader),
		scheduler.WithDatabase(api.Default()),
	)

	s.bridgeSvc = bridge.NewService(
		bridge.WithProvider(s.models.BridgeProvider()),
		bridge.WithScheduleServer(s.schedSvc),
	)

	if err := s.bridgeSvc.Start(); err != nil {
		return err
	}
	util.AddShutdownHook(func() { s.bridgeSvc.Stop() })

	if err := s.schedSvc.Start(); err != nil {
		return err
	}
	util.AddShutdownHook(func() { s.schedSvc.Stop() })
	return nil
}

func (s *Server) startMqttServer() error {
	if len(s.Mqtt.Listeners) == 0 {
		return nil
	}
	var tlsConf *tls.Config
	if s.Mqtt.EnableTls {
		serverCert := s.Mqtt.ServerCertPath
		if len(serverCert) == 0 {
			serverCert = s.ServerCertificatePath()
		}
		serverKey := s.Mqtt.ServerKeyPath
		if len(serverKey) == 0 {
			serverKey = s.ServerPrivateKeyPath()
		}
		if cfg, err := LoadTlsConfig(serverCert, serverKey, false, true); err != nil {
			return fmt.Errorf("mqtt server, %s", err.Error())
		} else {
			tlsConf = cfg
		}
	}
	opts := []MqttOption{
		WithMqttAuthServer(s, s.Mqtt.EnableTokenAuth && !s.Mqtt.EnableTls),
		WithMqttMaxMessageSizeLimit(s.Mqtt.MaxMessageSizeLimit),
		WithMqttTqlLoader(tqlLoader),
		WithMqttWsHandleListener(s.Http.Listeners),
	}
	if s.Mqtt.EnablePersistence {
		mqtt_dir := filepath.Join(s.homeDirPath, "mqtt", "data")
		opts = append(opts, WithMqttBadgerPersistent(mqtt_dir))
	}

	// mqtt server listeners
	for _, addr := range s.Mqtt.Listeners {
		if strings.HasPrefix(addr, "ws://") || strings.HasPrefix(addr, "wss://") {
			addr = strings.TrimPrefix(addr, "ws://")
			addr = strings.TrimPrefix(addr, "wss://")
			opts = append(opts, WithMqttWebsocketListener(addr, tlsConf))
		} else if strings.HasPrefix(addr, "unix://") {
			addr = strings.TrimPrefix(addr, "unix://")
			opts = append(opts, WithMqttUnixSockListener(addr))
		} else {
			addr = strings.TrimPrefix(addr, "tcp://")
			addr = strings.TrimPrefix(addr, "tls://")
			opts = append(opts, WithMqttTcpListener(addr, tlsConf))
		}
	}
	if mqttd, err := NewMqtt(api.Default(), opts...); err != nil {
		return fmt.Errorf("mqtt server, %s", err.Error())
	} else {
		s.mqttd = mqttd
	}
	if err := s.mqttd.Start(); err != nil {
		return fmt.Errorf("mqtt server, %s", err.Error())
	}
	util.AddShutdownHook(func() { s.mqttd.Stop() })
	return nil
}

func (s *Server) startHttpServer() error {
	if len(s.Http.Listeners) == 0 {
		return nil
	}
	opts := []HttpOption{
		WithHttpLicenseFilePath(s.licenseFilePath),
		WithHttpEulaFilePath(filepath.Join(s.prefDirPath, "EULA.TXT")),
		WithHttpListenAddress(s.Http.Listeners...),
		WithHttpAuthServer(s, s.Http.EnableTokenAuth),
		WithHttpTqlLoader(tqlLoader),
		WithHttpManagementServer(s),        // add, key
		WithHttpScheduleServer(s.schedSvc), // add, timer
		WithHttpBridgeServer(s.bridgeSvc),
		WithHttpServerSideFileSystem(ssfs.Default()),
		WithHttpBackupService(s.bakd),
		WithHttpDebugMode(s.Http.DebugMode, s.Http.DebugLatency),
		WithHttpExperimentModeProvider(func() bool { return s.ExperimentMode }),
		WithHttpWebShellProvider(s.models.ShellProvider()),
		WithHttpPackageManager(s.pkgMgr),
		WithHttpPathMap("data", s.homeDirPath),
		WithHttpLinger(s.Http.Linger),
		WithHttpWriteBufSize(s.Http.WriteBufSize),
		WithHttpReadBufSize(s.Http.ReadBufSize),
		WithHttpKeepAlive(s.Http.KeepAlive),
		WithHttpStatzAllow(s.Http.AllowStatz...),
	}
	if s.mqttd != nil {
		if h := s.mqttd.WsHandlerFunc(); h != nil {
			opts = append(opts, WithHttpMqttWsHandlerFunc(h))
		}
	}
	shellPorts, _ := s.getServicePorts("shell")
	shellAddrs := []string{}
	for _, sp := range shellPorts {
		shellAddrs = append(shellAddrs, sp.Address)
	}
	opts = append(opts, WithHttpNeoShellAddress(shellAddrs...))
	if s.Http.WebDir != "" {
		stat, err := os.Stat(s.Http.WebDir)
		if err != nil {
			return err
		}
		if !stat.IsDir() {
			return fmt.Errorf("web ui path is not a directory")
		}
		opts = append(opts, WithHttpWebDir(s.Http.WebDir))
	}
	if httpd, err := NewHttp(api.Default(), opts...); err != nil {
		return fmt.Errorf("http server, %s", err.Error())
	} else {
		s.httpd = httpd
	}
	if err := s.httpd.Start(); err != nil {
		return fmt.Errorf("http server, %s", err.Error())
	}
	util.AddShutdownHook(func() { s.httpd.Stop() })
	return nil
}

func (s *Server) startSshServer() error {
	if len(s.Shell.Listeners) == 0 {
		return nil
	}
	if sshd, err := NewSsh(
		WithSshListenAddress(s.Shell.Listeners...),
		WithSshServerKeyPath(s.ServerPrivateKeyPath()),
		WithSshIdleTimeout(s.Shell.IdleTimeout),
		WithSshAuthServer(s),
		WithSshMotdMessage(fmt.Sprintf("machbase-neo %s %s", mods.VersionString(), mods.Edition())),
		WithSshShellProvider(s.provideShellForSsh),
	); err != nil {
		return err
	} else {
		s.sshd = sshd
	}
	if err := s.sshd.Start(); err != nil {
		return err
	}
	util.AddShutdownHook(func() { s.sshd.Stop() })
	return nil
}

func (s *Server) initPackageManager() error {
	envs := map[string]string{}
	if b, err := os.Executable(); err == nil {
		b, _ = filepath.Abs(b)
		envs["MACHBASE_NEO"] = b
	}
	envs["MACHBASE_NEO_VERSION"] = mods.DisplayVersion()
	envs["MACHBASE_NEO_FILE"] = strings.Join(s.FileDirs, string(filepath.ListSeparator))
	envs["MACHBASE_NEO_HTTP"] = strings.Join(s.Http.Listeners, ",")
	envs["MACHBASE_NEO_MQTT"] = strings.Join(s.Mqtt.Listeners, ",")
	envs["MACHBASE_HOME"] = s.homeDirPath
	pkgsDir := filepath.Join(s.homeDirPath, "pkgs")
	if mgr, err := pkgs.NewPkgManager(pkgsDir, envs, s.ExperimentMode); err != nil {
		return fmt.Errorf("pkg manager, %s", err.Error())
	} else {
		s.pkgMgr = mgr
	}
	util.AddShutdownHook(func() { s.pkgMgr.Stop() })
	return nil
}

func (s *Server) getServicePorts(svc string) ([]*model.ServicePort, error) {
	s.servicePortsLock.RLock()
	defer s.servicePortsLock.RUnlock()

	ports := []*model.ServicePort{}
	for k, s := range s.servicePorts {
		if svc != "" {
			if strings.ToLower(svc) != k {
				continue
			}
		}
		ports = append(ports, s...)
	}
	sort.Slice(ports, func(i, j int) bool {
		if ports[i].Service == ports[j].Service {
			return ports[i].Address < ports[j].Address
		}
		return ports[i].Service < ports[j].Service
	})
	return ports, nil
}

func (s *Server) ServerPrivateKeyPath() string {
	return filepath.Join(s.certDirPath, "machbase_key.pem")
}

func (s *Server) ServerPublicKeyPath() string {
	return filepath.Join(s.certDirPath, "machbase_pub.pem")
}

func (s *Server) ServerCertificatePath() string {
	return filepath.Join(s.certDirPath, "machbase_cert.pem")
}

func (s *Server) ServerCertificate() (*x509.Certificate, error) {
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

func (s *Server) GetAllAuthorizedSshKeys() ([]*AuthorizedSshKey, error) {
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

func (s *Server) AddAuthorizedSshKey(keyType string, key string, comment string) error {
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

func (s *Server) RemoveAuthorizedSshKey(fingerprint string) error {
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

func (s *Server) checkListenPort(address string) error {
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
func (s *Server) AuthorizedCertificate(id string) (*x509.Certificate, error) {
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

func (s *Server) IterateAuthorizedCertificates(cb func(id string) bool) error {
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

func (s *Server) SetAuthorizedCertificate(id string, pemBytes []byte) error {
	path := filepath.Join(s.authorizedKeysDir, fmt.Sprintf("%s_cert.pem", id))
	return os.WriteFile(path, pemBytes, 00600)
}

func (s *Server) RemoveAuthorizedCertificate(id string) error {
	path := filepath.Join(s.authorizedKeysDir, fmt.Sprintf("%s_cert.pem", id))
	return os.Remove(path)
}

func (s *Server) ServerPublicKey() (any, error) {
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

func (s *Server) ServerPrivateKey() (crypto.PrivateKey, error) {
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

func (s *Server) mkKeysIfNotExists() error {
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
		return fmt.Errorf("private key encoder, %s", err.Error())
	}

	pubPem, err := ec.EncodePublic(pub)
	if err != nil {
		return fmt.Errorf("public key encoder, %s", err.Error())
	}

	certBytes, err := GenerateServerCertificate(pri, pub)
	if err != nil {
		return fmt.Errorf("certificate encoder, %s", err.Error())
	}

	if err := os.WriteFile(priPath, []byte(priPem), 0600); err != nil {
		return fmt.Errorf("private key writer, %s", err.Error())
	}
	if err := os.WriteFile(pubPath, []byte(pubPem), 0644); err != nil {
		return fmt.Errorf("public key writer, %s", err.Error())
	}
	if err := os.WriteFile(certPath, certBytes, 0644); err != nil {
		return fmt.Errorf("certificate writer, %s", err.Error())
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

// ////////////////////////////////
// implements AuthServer interface

var _ AuthServer = (*Server)(nil)

func (s *Server) ValidateClientToken(token string) (bool, error) {
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

func (s *Server) ValidateClientCertificate(clientId string, certHash string) (bool, error) {
	cert, err := s.AuthorizedCertificate(clientId)
	if err != nil {
		if err == os.ErrNotExist {
			return false, fmt.Errorf("client-id %s not found", clientId)
		} else {
			return false, err
		}
	}

	hash, err := HashCertificate(cert)
	if err != nil {
		return false, err
	}
	return hash == certHash, nil
}

func (s *Server) ValidateUserPublicKey(user string, publicKey ssh.PublicKey) (bool, string, error) {
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

func (s *Server) ValidateUserPassword(user string, password string) (bool, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	passed, reason, err := api.Default().UserAuth(ctx, user, password)
	if err != nil {
		return false, "", err
	} else if !passed {
		return false, reason, nil
	} else {
		return true, s.snowflakes[rand.Intn(len(s.snowflakes))], nil
	}
}

func (s *Server) ValidateUserOtp(user string, otp string) (bool, error) {
	for _, n := range s.snowflakes {
		if otp == n {
			return true, nil
		}
	}
	return false, nil
}

func (s *Server) GenerateOtp(user string) (string, error) {
	return s.snowflakes[rand.Intn(len(s.snowflakes))], nil
}

func (s *Server) GenerateSnowflake() string {
	return s.genSnowflake.Generate().Base64()
}

func (s *Server) runSqlScriptFile(title string, path string) error {
	if path == "" {
		return nil
	}
	if stat, err := os.Stat(path); err != nil {
		s.log.Warnf("fail to read script %s, %q", err.Error(), path)
		return nil
	} else if stat.IsDir() {
		s.log.Warnf("fail to read script dir %q", path)
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

func (s *Server) runSqlScripts(title string, queries []string) error {
	if len(queries) == 0 {
		return nil
	}
	if len(queries) == 1 && queries[0] == "" {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, err := api.Default().Connect(ctx, api.WithTrustUser("sys"))
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
		subLine := string(buff)
		buff = buff[:0]

		if strings.HasPrefix(subLine, "#") || strings.HasPrefix(subLine, "--") {
			continue
		}
		subLine = strings.TrimSpace(subLine)
		if len(subLine) == 0 {
			// skip empty line
			continue
		}

		lineBuff = append(lineBuff, subLine)
		if !strings.HasSuffix(subLine, ";") {
			continue
		}

		line := strings.Join(lineBuff, " ")
		line = strings.TrimSuffix(line, ";")
		lineBuff = lineBuff[:0]

		ret = append(ret, line)
	}
	return ret, nil
}
