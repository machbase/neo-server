package server

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/api/machsvr"
	"github.com/machbase/neo-server/v8/booter"
	"github.com/machbase/neo-server/v8/mods/logging"
)

func init() {
	booter.Register(
		"machbase.com/neo-server",
		func() *Config {
			return NewConfig()
		},
		func(conf *Config) (booter.Boot, error) {
			files := []string{}
			for _, f := range conf.FileDirs {
				toks := strings.Split(f, ",")
				files = append(files, toks...)
			}
			conf.FileDirs = files
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
	BackupDir      string
	FileDirs       []string
	MachbasePreset MachbasePreset
	Machbase       MachbaseConfig
	AuthHandler    AuthHandlerConfig
	Shell          ShellConfig
	Grpc           GrpcConfig
	Http           HttpConfig
	Mqtt           MqttConfig
	Jwt            JwtConfig
	NavelCord      *NavelCordConfig

	CreateDBQueries     []string // sql sentences
	CreateDBScriptFiles []string // file path
	StartupQueries      []string // sql sentences
	StartupScriptFiles  []string // file path

	NoBanner       bool
	ExperimentMode bool

	MachbaseInitOption machsvr.InitOption
	MaxPoolSize        int
	MaxOpenConn        int
	MaxOpenConnFactor  float64
	MaxOpenQuery       int
	MaxOpenQueryFactor float64
}

var PreferredPreset string = "auto"
var Headless bool = false
var HeadOnly bool = false

func NewConfig() *Config {
	conf := Config{}
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

type AuthHandlerConfig struct {
	Enabled bool
}

type GrpcConfig struct {
	Listeners      []string
	MaxRecvMsgSize int
	MaxSendMsgSize int
	Insecure       bool
}

type HttpConfig struct {
	Listeners []string
	WebDir    string

	NoAppendWorker  bool // deprecated, left for previous version's configuration file compatibility
	EnableWebUI     bool // deprecated, left for previous version's configuration file compatibility
	EnableTokenAuth bool
	DebugMode       bool
	AllowStatz      []string
	DebugLatency    string
	WriteBufSize    int
	ReadBufSize     int
	Linger          int
	KeepAlive       int
}

type MqttConfig struct {
	Listeners []string

	EnableTokenAuth bool
	EnableTls       bool
	ServerCertPath  string
	ServerKeyPath   string

	MaxMessageSizeLimit int
	EnablePersistence   bool
}

type ShellConfig struct {
	Listeners     []string
	IdleTimeout   time.Duration
	ServerKeyPath string
}

type NavelCordConfig struct {
	Port int
}

//go:embed svrconf.hcl
var DefaultFallbackConfig []byte

var DefaultFallbackPname string = "neo"

func (s *Server) GetConfig() string {
	return string(DefaultFallbackConfig)
}

func (s *Server) checkRewriteMachbaseConf(confpath string) (bool, error) {
	shouldRewrite := false
	content, err := os.ReadFile(confpath)
	if err != nil {
		return false, fmt.Errorf("MACH machbase.conf not available, %s", err.Error())
	}
	reader := bufio.NewReader(bytes.NewBuffer(content))
	parts := []string{}
	for !shouldRewrite {
		str, isPrefix, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return false, fmt.Errorf("MACH machbase.conf malformed, %s", err.Error())
			}
		}
		parts = append(parts, string(str))
		if isPrefix {
			continue
		}
		line := strings.TrimSpace(strings.Join(parts, ""))
		parts = parts[0:0]
		if strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "PORT_NO" && strconv.FormatInt(int64(s.Machbase.PORT_NO), 10) != value {
			s.log.Infof("MACH PORT_NO will be %d, previously %s", s.Machbase.PORT_NO, value)
			shouldRewrite = true
		} else if key == "BIND_IP_ADDRESS" && s.Machbase.BIND_IP_ADDRESS != value {
			s.log.Infof("MACH BIND_IP_ADDRESS will be %s, previously %s", s.Machbase.BIND_IP_ADDRESS, value)
			shouldRewrite = true
		}
	}
	return shouldRewrite, nil
}

func (s *Server) rewriteMachbaseConf(confpath string) error {
	content, err := os.ReadFile(confpath)
	if err != nil {
		return fmt.Errorf("MACH machbase.conf not available, %s", err.Error())
	}
	reader := bufio.NewReader(bytes.NewBuffer(content))
	newConfLines := []string{}
	parts := []string{}
	for {
		str, isPrefix, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return fmt.Errorf("MACH machbase.conf malformed, %s", err.Error())
			}
		}
		parts = append(parts, string(str))
		if isPrefix {
			continue
		}
		line := strings.TrimSpace(strings.Join(parts, ""))
		parts = parts[0:0]
		if strings.HasPrefix(line, "#") {
			newConfLines = append(newConfLines, line)
			continue
		}
		key, _, ok := strings.Cut(line, "=")
		if !ok {
			newConfLines = append(newConfLines, line)
			continue
		}
		key = strings.TrimSpace(key)
		switch key {
		case "PORT_NO":
			newConfLines = append(newConfLines, fmt.Sprintf("PORT_NO = %d", s.Machbase.PORT_NO))
		case "BIND_IP_ADDRESS":
			newConfLines = append(newConfLines, fmt.Sprintf("BIND_IP_ADDRESS = %s", s.Machbase.BIND_IP_ADDRESS))
		default:
			newConfLines = append(newConfLines, line)
		}
	}
	fd, err := os.OpenFile(confpath, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("MACH machbase.conf unable to write, %s", err.Error())
	}
	if _, err = fd.Write([]byte(strings.Join(newConfLines, "\n"))); err != nil {
		return fmt.Errorf("MACH machbase.conf write error, %s", err.Error())
	}
	if err = fd.Close(); err != nil {
		return fmt.Errorf("MACH machbase.conf close error, %s", err.Error())
	}
	return nil
}
