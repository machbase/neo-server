package logging

import (
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/util/glob"
	gometrics "github.com/rcrowley/go-metrics"
)

type Level int

func (lvl *Level) UnmarshalJSON(b []byte) error {
	*lvl = ParseLogLevel(string(b))
	return nil
}

func StringToLogLevelHookFunc(f reflect.Type, t reflect.Type, data any) (any, error) {
	if f.Kind() != reflect.String || t != reflect.TypeOf(LevelInfo) {
		return data, nil
	}
	lvl, flag := ParseLogLevelP(data.(string))
	if flag {
		return lvl, nil
	} else {
		return nil, fmt.Errorf("invalid log level: %v", data)
	}
}

const (
	LevelAll Level = iota
	LevelTrace
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
)

var logLevelNames = []string{"ALL", "TRACE", "DEBUG", "INFO", "WARN", "ERROR"}

func ParseLogLevel(name string) Level {
	n := strings.ToUpper(name)
	switch n {
	default:
		return LevelAll
	case "TRACE":
		return LevelTrace
	case "DEBUG":
		return LevelDebug
	case "INFO":
		return LevelInfo
	case "WARN":
		return LevelWarn
	case "ERROR":
		return LevelError
	case "NONE":
		return LevelError + 1
	}
}

func ParseLogLevelP(name string) (Level, bool) {
	n := strings.ToUpper(name)
	switch n {
	default:
		return LevelAll, false
	case "TRACE":
		return LevelTrace, true
	case "DEBUG":
		return LevelDebug, true
	case "INFO":
		return LevelInfo, true
	case "WARN":
		return LevelWarn, true
	case "ERROR":
		return LevelError, true
	case "NONE":
		return LevelError + 1, false
	}
}

func LogLevelName(level Level) string {
	if level >= 0 && int(level) < len(logLevelNames) {
		return logLevelNames[level]
	}
	return "UNKNOWN"
}

type Log interface {
	io.Writer

	TraceEnabled() bool
	Trace(...any)
	Tracef(format string, args ...any)
	DebugEnabled() bool
	Debug(...any)
	Debugf(format string, args ...any)
	InfoEnabled() bool
	Info(...any)
	Infof(format string, args ...any)
	WarnEnabled() bool
	Warn(...any)
	Warnf(format string, args ...any)
	ErrorEnabled() bool
	Error(...any)
	Errorf(format string, args ...any)

	LogEnabled(level Level) bool

	Log(level Level, m ...any)
	Logf(level Level, format string, args ...any)
	LogWithSkipCallstack(lvl Level, skip int, m ...any)
	LogfWithSkipCallstack(lvl Level, skip int, format string, args ...any)

	SetLevel(level Level)
	Level() Level
}

type levelLogger struct {
	name         string
	level        Level
	underlying   []*logWriter
	prefixWidth  int
	enableSrcLoc bool
	// slog compat
	attrs []slog.Attr
}

func (l *levelLogger) SetLevel(level Level) { l.level = level }
func (l *levelLogger) Level() Level         { return l.level }

func (l *levelLogger) TraceEnabled() bool { return l.level <= LevelTrace }
func (l *levelLogger) DebugEnabled() bool { return l.level <= LevelDebug }
func (l *levelLogger) InfoEnabled() bool  { return l.level <= LevelInfo }
func (l *levelLogger) WarnEnabled() bool  { return l.level <= LevelWarn }
func (l *levelLogger) ErrorEnabled() bool { return l.level <= LevelError }

func (l *levelLogger) LogEnabled(lvl Level) bool { return l.level <= lvl }

func (l *levelLogger) Trace(m ...any) { l._log(LevelTrace, 1, m) }
func (l *levelLogger) Debug(m ...any) { l._log(LevelDebug, 1, m) }
func (l *levelLogger) Info(m ...any)  { l._log(LevelInfo, 1, m) }
func (l *levelLogger) Warn(m ...any)  { l._log(LevelWarn, 1, m) }
func (l *levelLogger) Error(m ...any) { l._log(LevelError, 1, m) }

func (l *levelLogger) Tracef(format string, args ...any)          { l._logf(LevelTrace, 0, format, args) }
func (l *levelLogger) Debugf(format string, args ...any)          { l._logf(LevelDebug, 0, format, args) }
func (l *levelLogger) Infof(format string, args ...any)           { l._logf(LevelInfo, 0, format, args) }
func (l *levelLogger) Warnf(format string, args ...any)           { l._logf(LevelWarn, 0, format, args) }
func (l *levelLogger) Errorf(format string, args ...any)          { l._logf(LevelError, 0, format, args) }
func (l *levelLogger) Logf(lvl Level, format string, args ...any) { l._logf(lvl, 0, format, args) }
func (l *levelLogger) LogfWithSkipCallstack(lvl Level, skip int, format string, args ...any) {
	l._logf(lvl, skip, format, args)
}

func (l *levelLogger) PrefixWidth() int { return l.prefixWidth }
func (l *levelLogger) SetPrefixWidth(width int) {
	if width > 0 {
		l.prefixWidth = width
	} else {
		l.prefixWidth = prefixWidthDefault
	}
}

func (l *levelLogger) IsEnableSourceLocation() bool { return l.enableSrcLoc }
func (l *levelLogger) SetEnableSourceLocation(flag bool) {
	l.enableSrcLoc = flag
}

func (l *levelLogger) Log(lvl Level, m ...any) {
	l._logf(lvl, 0, "%s", m)
}

func (l *levelLogger) LogWithSkipCallstack(lvl Level, skip int, m ...any) {
	l._logf(lvl, skip, "%s", m)
}

func (l *levelLogger) Write(buff []byte) (n int, err error) {
	ts := fmt.Sprintf("%s -     ", time.Now().Format("2006/01/02 15:04:05.000"))
	for _, w := range l.underlying {
		w.Write([]byte(ts))
		n, err = w.Write(buff)
	}
	return
}

const (
	green   = "\033[97;42m"
	white   = "\033[90;47m"
	yellow  = "\033[90;43m"
	red     = "\033[97;41m"
	blue    = "\033[97;44m"
	magenta = "\033[97;45m"
	cyan    = "\033[97;46m"
	reset   = "\033[0m"
)

var (
	warnCounter  gometrics.Counter
	errorCounter gometrics.Counter
	totalCounter gometrics.Counter
)

func init() {
	totalCounter = gometrics.NewRegisteredCounter("log.total", gometrics.DefaultRegistry)
	warnCounter = gometrics.NewRegisteredCounter("log.warns", gometrics.DefaultRegistry)
	errorCounter = gometrics.NewRegisteredCounter("log.errors", gometrics.DefaultRegistry)
}

// ///////////////////////////////////////////
var levelConfig = make(map[string]Level)
var levelDefault = LevelInfo
var prefixWidthDefault = 18
var enableSourceLocationDefault = false

func SetDefaultLevel(lvl Level) {
	levelDefault = lvl
}

func DefaultLevel() Level {
	return levelDefault
}

func SetDefaultEnableSourceLocation(flag bool) {
	enableSourceLocationDefault = flag
}

func SetDefaultPrefixWidth(width int) {
	if width > 0 {
		prefixWidthDefault = width
	} else {
		prefixWidthDefault = 18
	}
}

func DefaultPrefixWidth() int {
	return prefixWidthDefault
}

func SetLevel(name string, lvl Level) {
	levelConfig[name] = lvl
}

func GetLevel(name string) Level {
	var matchedPattern string
	var matchedLevel Level

	for pattern, level := range levelConfig {
		if match, err := glob.Match(pattern, name); match && err == nil {
			if len(matchedPattern) < len(pattern) {
				matchedPattern = pattern
				matchedLevel = level
			}
		}
	}

	if matchedPattern != "" {
		return matchedLevel
	}

	return levelDefault
}
