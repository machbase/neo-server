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
	"text/template"
	"time"

	"github.com/machbase/neo-server/v8/booter"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/spi/machsvr"
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
	Grpc           GrpcConfig // deprecated, left for previous version's configuration file compatibility
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
	MaxPoolSize        int // deprecated
	MaxOpenConn        int
	MaxIdleConn        int
	ConnMaxLifetime    time.Duration
	ConnMaxIdleTime    time.Duration
	MaxOpenConnFactor  float64 // deprecated
	MaxOpenQuery       int     // deprecated
	MaxOpenQueryFactor float64 // deprecated
	StatzOut           string
}

var PreferredPreset string = "auto"
var Headless bool = false

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

// deprecated, left for previous version's configuration file compatibility
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
	StatzToken      string
	QueryCypher     string // format: "alg=AES key=1234567890abcdef pad=pkcs5"
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

//go:embed config.hcl
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

type MachbaseConfig struct {
	PORT_NO         int
	BIND_IP_ADDRESS string
	DBS_PATH        string

	TRACE_LOGFILE_SIZE  int64
	TRACE_LOGFILE_COUNT int
	TRACE_LOGFILE_PATH  string
	TRACE_LOG_LEVEL     int

	GEN_CALLSTACK_FOR_ABORT_ERROR int
	GEN_CORE_FILE                 int

	DURATION_GAP int
	CPU_PARALLEL int

	DISK_COLUMNAR_TABLE_CHECKPOINT_INTERVAL_SEC int
	DISK_COLUMNAR_INDEX_CHECKPOINT_INTERVAL_SEC int

	DISK_COLUMNAR_TABLE_COLUMN_PART_FLUSH_MODE              int
	DISK_COLUMNAR_TABLE_COLUMN_PART_IO_INTERVAL_MIN_SEC     int
	DISK_COLUMNAR_TABLESPACE_MEMORY_MIN_SIZE                int64
	DISK_COLUMNAR_TABLESPACE_MEMORY_MAX_SIZE                int64
	DISK_COLUMNAR_TABLESPACE_MEMORY_EXT_SIZE                int64
	DISK_COLUMNAR_TABLESPACE_MEMORY_SLOWDOWN_HIGH_LIMIT_PCT int
	DISK_COLUMNAR_TABLESPACE_MEMORY_SLOWDOWN_MSEC           int
	DISK_COLUMNAR_TABLESPACE_DWFILE_INT_SIZE                int64
	DISK_COLUMNAR_TABLESPACE_DWFILE_EXT_SIZE                int64

	PROCESS_MAX_SIZE                int64
	DUMP_APPEND_ERROR               int
	DISK_TABLESPACE_DIRECT_IO_WRITE int
	DISK_TABLESPACE_DIRECT_IO_READ  int
	DISK_TABLESPACE_SYNCHRONOUS     int

	INDEX_BUILD_THREAD_COUNT                int
	INDEX_FLUSH_MAX_REQUEST_COUNT_PER_INDEX int
	INDEX_BUILD_MAX_ROW_COUNT_PER_THREAD    int

	DISK_COLUMNAR_INDEX_SHUTDOWN_BUILD_FINISH int
	DISK_IO_THREAD_COUNT                      int

	CPU_AFFINITY_BEGIN_ID int
	CPU_AFFINITY_COUNT    int

	INDEX_LEVEL_PARTITION_BUILD_THREAD_COUNT int
	INDEX_LEVEL_PARTITION_AGER_THREAD_COUNT  int

	DISK_COLUMNAR_PAGE_CACHE_MAX_SIZE                 int64
	INDEX_LEVEL_PARTITION_BUILD_MEMORY_HIGH_LIMIT_PCT int
	VOLATILE_TABLESPACE_MEMORY_MAX_SIZE               int64

	GRANT_REMOTE_ACCESS int

	DISK_COLUMNAR_TABLE_TIME_INVERSION_MODE int

	DEFAULT_LSM_MAX_LEVEL    int
	MAX_QPX_MEM              int64
	TAGDATA_AUTO_META_INSERT int
	SESSION_IDLE_TIMEOUT_SEC int

	HTTP_ENABLE  int
	HTTP_MAX_MEM int64
	HTTP_AUTH    int

	LOOKUP_APPEND_UPDATE_ON_DUPKEY int

	ROLLUP_FETCH_COUNT_LIMIT int

	HANDLE_LIMIT int

	TAG_PARTITION_COUNT       int
	TAG_DATA_PART_SIZE        int
	TAG_CACHE_MAX_MEMORY_SIZE int
	DISK_TAG_INDEX_BLOCKS     int
	TAG_TABLE_META_MAX_SIZE   int64
	DISK_BUFFER_COUNT         int
	TAG_CACHE_ENABLE          int
}

type MachbasePreset int

const (
	PresetNone MachbasePreset = iota
	PresetFog
	PresetEdge
)

func (p MachbasePreset) String() string {
	switch p {
	default:
		return "none"
	case PresetFog:
		return "fog"
	case PresetEdge:
		return "edge"
	}
}

func DefaultMachbaseConfig(preset MachbasePreset) *MachbaseConfig {
	c := &MachbaseConfig{
		PORT_NO:             5656,
		BIND_IP_ADDRESS:     "127.0.0.1",
		DBS_PATH:            "?/dbs",
		TRACE_LOGFILE_SIZE:  10485760, // 10MB
		TRACE_LOGFILE_COUNT: 1000,
		TRACE_LOGFILE_PATH:  "?/trc",
		TRACE_LOG_LEVEL:     277,

		GEN_CALLSTACK_FOR_ABORT_ERROR: 0,
		GEN_CORE_FILE:                 1,

		DURATION_GAP: 0,
		CPU_PARALLEL: 1,

		DISK_COLUMNAR_TABLE_CHECKPOINT_INTERVAL_SEC: 120,
		DISK_COLUMNAR_INDEX_CHECKPOINT_INTERVAL_SEC: 120,

		DISK_COLUMNAR_TABLE_COLUMN_PART_FLUSH_MODE:              0,
		DISK_COLUMNAR_TABLE_COLUMN_PART_IO_INTERVAL_MIN_SEC:     3,
		DISK_COLUMNAR_TABLESPACE_MEMORY_MIN_SIZE:                104857600,  // 100MB
		DISK_COLUMNAR_TABLESPACE_MEMORY_MAX_SIZE:                8589934592, // 8GB
		DISK_COLUMNAR_TABLESPACE_MEMORY_EXT_SIZE:                2097152,    // 2MB
		DISK_COLUMNAR_TABLESPACE_MEMORY_SLOWDOWN_HIGH_LIMIT_PCT: 80,
		DISK_COLUMNAR_TABLESPACE_MEMORY_SLOWDOWN_MSEC:           1,
		DISK_COLUMNAR_TABLESPACE_DWFILE_INT_SIZE:                2097152, // 2MB
		DISK_COLUMNAR_TABLESPACE_DWFILE_EXT_SIZE:                1048576, // 1MB

		PROCESS_MAX_SIZE:                4294967296, // 4GB
		DUMP_APPEND_ERROR:               0,
		DISK_TABLESPACE_DIRECT_IO_WRITE: 1,
		DISK_TABLESPACE_DIRECT_IO_READ:  0,
		DISK_TABLESPACE_SYNCHRONOUS:     1,

		INDEX_BUILD_THREAD_COUNT:                3,
		INDEX_FLUSH_MAX_REQUEST_COUNT_PER_INDEX: 3,
		INDEX_BUILD_MAX_ROW_COUNT_PER_THREAD:    100000,

		DISK_COLUMNAR_INDEX_SHUTDOWN_BUILD_FINISH: 0,
		DISK_IO_THREAD_COUNT:                      3,

		CPU_AFFINITY_BEGIN_ID: 0,
		CPU_AFFINITY_COUNT:    0,

		INDEX_LEVEL_PARTITION_BUILD_THREAD_COUNT: 3,
		INDEX_LEVEL_PARTITION_AGER_THREAD_COUNT:  1,

		INDEX_LEVEL_PARTITION_BUILD_MEMORY_HIGH_LIMIT_PCT: 70,
		VOLATILE_TABLESPACE_MEMORY_MAX_SIZE:               2147483648, // 2GB

		GRANT_REMOTE_ACCESS: 1,

		DISK_COLUMNAR_TABLE_TIME_INVERSION_MODE: 1,

		DEFAULT_LSM_MAX_LEVEL:    2,
		MAX_QPX_MEM:              1073741824, // 1GB
		TAGDATA_AUTO_META_INSERT: 2,
		SESSION_IDLE_TIMEOUT_SEC: 0,

		HANDLE_LIMIT: 8192,

		LOOKUP_APPEND_UPDATE_ON_DUPKEY: 0,
		ROLLUP_FETCH_COUNT_LIMIT:       3000000,
		TAG_CACHE_MAX_MEMORY_SIZE:      512 * 1024 * 1024,
		DISK_TAG_INDEX_BLOCKS:          2048,
		TAG_TABLE_META_MAX_SIZE:        104857600, // 100MB
		DISK_BUFFER_COUNT:              16,
		TAG_CACHE_ENABLE:               31, // 0b11111
	}
	switch preset {
	case PresetFog:
		c.DISK_COLUMNAR_PAGE_CACHE_MAX_SIZE = 2 * 1024 * 1024 * 1024        // 2GB
		c.DISK_COLUMNAR_TABLESPACE_MEMORY_MAX_SIZE = 8 * 1024 * 1024 * 1024 // 8GB
		c.PROCESS_MAX_SIZE = 64 * 1024 * 1024 * 1024                        // 64GB
		c.TAG_TABLE_META_MAX_SIZE = 512 * 1024 * 1024                       // 512MB
		c.VOLATILE_TABLESPACE_MEMORY_MAX_SIZE = 2 * 1024 * 1024 * 1024      // 2GB
		c.TAG_CACHE_MAX_MEMORY_SIZE = 512 * 1024 * 1024                     // 512MB
		c.DEFAULT_LSM_MAX_LEVEL = 2                                         //
		c.MAX_QPX_MEM = 1024 * 1024 * 1024                                  // 1GB
		c.ROLLUP_FETCH_COUNT_LIMIT = 3000000                                //
		c.TAG_CACHE_ENABLE = 31                                             //
		c.TAG_PARTITION_COUNT = 4                                           //
		c.TAG_DATA_PART_SIZE = 16 * 1024 * 1024                             // 16MB
	case PresetEdge:
		c.DISK_COLUMNAR_PAGE_CACHE_MAX_SIZE = 128 * 1024 * 1024        // 128MB
		c.DISK_COLUMNAR_TABLESPACE_MEMORY_MAX_SIZE = 256 * 1024 * 1024 // 256MB
		c.PROCESS_MAX_SIZE = 32 * 1024 * 1024 * 1024                   // 32GB
		c.TAG_TABLE_META_MAX_SIZE = 100 * 1024 * 1024                  // 100MB
		c.VOLATILE_TABLESPACE_MEMORY_MAX_SIZE = 512 * 1024 * 1024      // 512MB
		c.TAG_CACHE_MAX_MEMORY_SIZE = 256 * 1024 * 1024                // 256MB
		c.DEFAULT_LSM_MAX_LEVEL = 0                                    //
		c.MAX_QPX_MEM = 256 * 1024 * 1024                              // 256MB
		c.ROLLUP_FETCH_COUNT_LIMIT = 10000                             // Max speed of 32bit rollup : 10000rec/sec
		c.DISK_TAG_INDEX_BLOCKS = 512                                  //
		c.DISK_BUFFER_COUNT = 1                                        //
		c.TAG_CACHE_ENABLE = 3                                         //
		c.TAG_PARTITION_COUNT = 1                                      //
		c.TAG_DATA_PART_SIZE = 1024 * 1024                             // 1MB
		c.HANDLE_LIMIT = 4096                                          //
	}
	return c
}

//go:embed config_machbase.conf
var conf_template string

func applyMachbaseConfig(confpath string, conf *MachbaseConfig) error {
	tmpl, err := template.New("machbase_conf").Parse(conf_template)
	if err != nil {
		return fmt.Errorf("config template, %s", err.Error())
	}
	f, err := os.OpenFile(confpath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("config file open, %s", err.Error())
	}
	defer f.Close()
	err = tmpl.Execute(f, conf)
	if err != nil {
		return fmt.Errorf("config file write, %s", err.Error())
	}
	return nil
}
