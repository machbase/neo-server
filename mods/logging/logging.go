package logging

import (
	"fmt"
	"io"
	"os"

	"github.com/robfig/cron/v3"
	"gopkg.in/natefinch/lumberjack.v2"
)

/*
	Log rotation schedule

	"0 30 * * * *"             Every hour on the half hour
	"@hourly"                  Every hour
	"@every 1h30m"             Every hour thirty

	@yearly
	@monthly
	@daily
	@hourly
	@midnight
*/

type Config struct {
	Console                     bool          `json:"console" default:"true" help:"enable console output"`
	Filename                    string        `json:"filename" placeholder:"<path>" help:"log file path"`
	Append                      bool          `json:"append" help:"append to existing log file"`
	RotateSchedule              string        `json:"rotateSchedule" help:"schedule to roate log file"`
	MaxSize                     int           `json:"maxSize" help:"log file max size in MB"`
	MaxBackups                  int           `json:"maxBackups" help:"number of backup files"`
	MaxAge                      int           `json:"maxAge" help:"how many days keep backup files"`
	Compress                    bool          `json:"compress" placeholder:"true|false" default:"false" help:"compress backup files"`
	Levels                      []LevelConfig `json:"levels" hidden:""`
	UTC                         bool          `json:"utc" help:"log time format in UTC"`
	DefaultPrefixWidth          int           `json:"defaultPrefixWidth" default:"20" hidden:""`
	DefaultEnableSourceLocation bool          `json:"defaultEnableSourceLocation" default:"false" hidden:""`
	DefaultLevel                string        `json:"defaultLevel"  enum:"TRACE,DEBUG,INFO,WARN,ERROR" default:"INFO" help:"TRACE,DEBUG,INFO,WARN,ERROR"`
}

type LogServerConfig struct {
	Address string
	Labels  map[string]string
}

type LevelConfig struct {
	Pattern              string `json:"pattern"`
	Level                string `json:"level" enum:"TRACE,DEBUG,INFO,WARN,ERROR" default:"INFO"`
	EnableSourceLocation bool   `json:"enableSourceLocation"`
}

var PresetConfigStdout = Config{
	Console:                     false,
	Filename:                    "-",
	Append:                      true,
	DefaultPrefixWidth:          30,
	DefaultEnableSourceLocation: true,
	DefaultLevel:                "TRACE",
}

var PresetConfigDiscard = Config{
	Console:                     false,
	Filename:                    ".",
	Append:                      false,
	DefaultPrefixWidth:          30,
	DefaultEnableSourceLocation: true,
	DefaultLevel:                "TRACE",
}

var rotateCron = cron.New()

var defaultWriter []*logWriter

func Configure(cfg *Config) {
	for _, c := range cfg.Levels {
		levelConfig[c.Pattern] = ParseLogLevel(c.Level)
	}
	SetDefaultPrefixWidth(cfg.DefaultPrefixWidth)
	SetDefaultLevel(ParseLogLevel(cfg.DefaultLevel))
	SetDefaultEnableSourceLocation(cfg.DefaultEnableSourceLocation)

	if cfg.Filename == "." {
		// defaultWriter = []*logWriter{{Writer: io.Discard, isTerm: false}}
		defaultWriter = []*logWriter{}
	} else if cfg.Filename == "-" {
		defaultWriter = []*logWriter{{Writer: os.Stdout, isTerm: true}}
	} else {
		lj := &lumberjack.Logger{
			Filename:   cfg.Filename,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
			LocalTime:  !cfg.UTC,
		}
		if !cfg.Append {
			lj.Rotate()
		}
		if len(cfg.RotateSchedule) > 0 {
			_, err := rotateCron.AddFunc(cfg.RotateSchedule, func() {
				lj.Rotate()
			})
			if err == nil {
				go rotateCron.Run()
			} else {
				fmt.Fprintf(os.Stderr, "ERR logger rotate schdule %s", err.Error())
			}
		}
		if cfg.Console {
			defaultWriter = []*logWriter{
				{Writer: lj, isTerm: false},
				{Writer: os.Stdout, isTerm: true},
			}
		} else {
			defaultWriter = []*logWriter{{Writer: lj, isTerm: false}}
		}
	}
}

func GetLog(name string) Log {
	return &levelLogger{
		name:         name,
		level:        GetLevel(name),
		underlying:   defaultWriter,
		prefixWidth:  prefixWidthDefault,
		enableSrcLoc: enableSourceLocationDefault,
	}
}

func NewLog(name string, writer io.Writer) Log {
	return &levelLogger{
		name:         name,
		level:        GetLevel(name),
		underlying:   []*logWriter{{Writer: writer, isTerm: false}},
		prefixWidth:  prefixWidthDefault,
		enableSrcLoc: enableSourceLocationDefault,
	}
}

type LogFileConf struct {
	Filename             string
	Level                string
	MaxSize              int
	MaxBackups           int
	MaxAge               int
	Compress             bool
	Append               bool
	RotateSchedule       string
	Console              bool
	PrefixWidth          int
	EnableSourceLocation bool
	LogServer            LogServerConfig
}

func NewLogFile(name string, cfg LogFileConf) Log {
	var underlying []*logWriter
	level := ParseLogLevel(cfg.Level)

	if len(cfg.Filename) == 0 || cfg.Filename == "." {
		defaultWriter = []*logWriter{}
	} else if cfg.Filename == "-" {
		defaultWriter = []*logWriter{{Writer: os.Stdout, isTerm: true}}
	} else {
		lj := &lumberjack.Logger{
			Filename:   cfg.Filename,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
			LocalTime:  true,
		}
		if !cfg.Append {
			lj.Rotate()
		}
		if len(cfg.RotateSchedule) > 0 {
			rotateCron.AddFunc(cfg.RotateSchedule, func() {
				lj.Rotate()
			})
			go rotateCron.Run()
		}

		if cfg.Console {
			underlying = []*logWriter{
				{Writer: lj, isTerm: false},
				{Writer: os.Stdout, isTerm: true},
			}
		} else {
			underlying = []*logWriter{{Writer: lj, isTerm: false}}
		}
	}

	return &levelLogger{
		name:         name,
		level:        level,
		underlying:   underlying,
		prefixWidth:  cfg.PrefixWidth,
		enableSrcLoc: cfg.EnableSourceLocation,
	}
}

type logWriter struct {
	io.Writer
	isTerm bool
}
