package logging

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"
)

func TestLogLevels(t *testing.T) {
	tests := []struct {
		name  string
		level Level
		valid bool
	}{
		{name: "trace", level: LevelTrace, valid: true},
		{name: "DEBUG", level: LevelDebug, valid: true},
		{name: "Info", level: LevelInfo, valid: true},
		{name: "warn", level: LevelWarn, valid: true},
		{name: "error", level: LevelError, valid: true},
		{name: "none", level: LevelError + 1},
		{name: "invalid", level: LevelAll},
	}
	for _, tc := range tests {
		if got := ParseLogLevel(tc.name); got != tc.level {
			t.Fatalf("ParseLogLevel(%q) was %v, want %v", tc.name, got, tc.level)
		}
		if got, valid := ParseLogLevelP(tc.name); got != tc.level || valid != tc.valid {
			t.Fatalf("ParseLogLevelP(%q) was %v, %v", tc.name, got, valid)
		}
	}
	if LogLevelName(LevelTrace) != "TRACE" || LogLevelName(-1) != "UNKNOWN" || LogLevelName(100) != "UNKNOWN" {
		t.Fatal("LogLevelName() returned an unexpected name")
	}

	var level Level
	if err := level.UnmarshalJSON([]byte("WARN")); err != nil || level != LevelWarn {
		t.Fatalf("UnmarshalJSON() returned level=%v err=%v", level, err)
	}
	stringType := reflect.TypeOf("")
	levelType := reflect.TypeOf(LevelInfo)
	if got, err := StringToLogLevelHookFunc(stringType, levelType, "DEBUG"); err != nil || got != LevelDebug {
		t.Fatalf("StringToLogLevelHookFunc() returned %v, %v", got, err)
	}
	if _, err := StringToLogLevelHookFunc(stringType, levelType, "invalid"); err == nil {
		t.Fatal("StringToLogLevelHookFunc() returned nil for invalid level")
	}
	if got, err := StringToLogLevelHookFunc(reflect.TypeOf(1), levelType, 1); err != nil || got != 1 {
		t.Fatalf("non-string hook returned %v, %v", got, err)
	}
}

func TestLevelConfiguration(t *testing.T) {
	previousLevels := levelConfig
	previousDefault := levelDefault
	previousWidth := prefixWidthDefault
	previousSourceLocation := enableSourceLocationDefault
	t.Cleanup(func() {
		levelConfig = previousLevels
		levelDefault = previousDefault
		prefixWidthDefault = previousWidth
		enableSourceLocationDefault = previousSourceLocation
	})

	levelConfig = make(map[string]Level)
	SetDefaultLevel(LevelInfo)
	SetDefaultPrefixWidth(24)
	SetDefaultEnableSourceLocation(true)
	SetLevel("server.*", LevelDebug)
	SetLevel("server.http.*", LevelTrace)
	if DefaultLevel() != LevelInfo || DefaultPrefixWidth() != 24 {
		t.Fatal("default log configuration was not retained")
	}
	if GetLevel("server.http.query") != LevelTrace || GetLevel("server.mqtt") != LevelDebug || GetLevel("other") != LevelInfo {
		t.Fatal("pattern log levels were not selected by specificity")
	}
	SetDefaultPrefixWidth(0)
	if DefaultPrefixWidth() != 18 {
		t.Fatalf("default prefix width was %d, want 18", DefaultPrefixWidth())
	}
}

func TestLoggerOutputAndControls(t *testing.T) {
	var output bytes.Buffer
	logger := NewLog("test", &output).(*levelLogger)
	logger.SetLevel(LevelTrace)
	if logger.Level() != LevelTrace || !logger.TraceEnabled() || !logger.DebugEnabled() || !logger.InfoEnabled() || !logger.WarnEnabled() || !logger.ErrorEnabled() {
		t.Fatal("trace logger did not enable all levels")
	}
	if !logger.LogEnabled(LevelInfo) {
		t.Fatal("LogEnabled(INFO) was false")
	}
	logger.SetPrefixWidth(12)
	if logger.PrefixWidth() != 12 {
		t.Fatalf("PrefixWidth() was %d", logger.PrefixWidth())
	}
	logger.SetPrefixWidth(0)
	logger.SetEnableSourceLocation(true)
	if !logger.IsEnableSourceLocation() {
		t.Fatal("source location was not enabled")
	}

	logger.Trace("trace", 1)
	logger.Debugf("debug-%d", 2)
	logger.Info("info")
	logger.Warnf("warn-%d", 3)
	logger.Error("error")
	logger.Log(LevelInfo, "log")
	logger.Logf(LevelInfo, "logf-%d", 4)
	logger.LogWithSkipCallstack(LevelInfo, 0, "skip")
	logger.LogfWithSkipCallstack(LevelInfo, 0, "skipf-%d", 5)
	for _, text := range []string{"trace 1", "debug-2", "info", "warn-3", "error", "log", "logf-4", "skip", "skipf-5"} {
		if !strings.Contains(output.String(), text) {
			t.Fatalf("log output omitted %q: %s", text, output.String())
		}
	}

	before := output.Len()
	logger.SetLevel(LevelError)
	logger.Debug("filtered")
	if output.Len() != before || logger.TraceEnabled() || logger.DebugEnabled() || logger.InfoEnabled() || logger.WarnEnabled() || !logger.ErrorEnabled() {
		t.Fatal("error-level filtering was incorrect")
	}

	terminalOutput := &bytes.Buffer{}
	terminalLogger := &levelLogger{name: "term", level: LevelTrace, underlying: []*logWriter{{Writer: terminalOutput, isTerm: true}}, prefixWidth: 8}
	terminalLogger.Warn("warning")
	if !strings.Contains(terminalOutput.String(), yellow) || !strings.Contains(terminalOutput.String(), reset) {
		t.Fatalf("terminal warning omitted colors: %q", terminalOutput.String())
	}
	if got := removeEscape("a" + red + "b" + reset + "c"); got != "abc" {
		t.Fatalf("removeEscape() was %q, want abc", got)
	}

	var raw bytes.Buffer
	rawLogger := &levelLogger{underlying: []*logWriter{{Writer: &raw}}}
	if n, err := rawLogger.Write([]byte("raw")); err != nil || n != 3 || !strings.HasSuffix(raw.String(), "raw") {
		t.Fatalf("Write() returned n=%d err=%v output=%q", n, err, raw.String())
	}
}

func TestLoggerClose(t *testing.T) {
	first := &testCloser{err: errors.New("first")}
	second := &testCloser{err: errors.New("second")}
	logger := &levelLogger{underlying: []*logWriter{nil, {closer: first}, {closer: second}, {Writer: &bytes.Buffer{}}}}
	if err := logger.Close(); !errors.Is(err, first.err) {
		t.Fatalf("Close() error was %v, want %v", err, first.err)
	}
	if !first.closed || !second.closed {
		t.Fatal("Close() did not close every closer")
	}
}

func TestConfigureAndConstructors(t *testing.T) {
	previousWriter := defaultWriter
	previousLevels := levelConfig
	t.Cleanup(func() {
		defaultWriter = previousWriter
		levelConfig = previousLevels
	})

	var output bytes.Buffer
	Configure(&Config{Writer: &output, DefaultLevel: "DEBUG", DefaultPrefixWidth: 10, Levels: []LevelConfig{{Pattern: "special", Level: "TRACE"}}})
	logger := GetLog("special").(*levelLogger)
	if logger.Level() != LevelTrace || logger.prefixWidth != 10 || len(logger.underlying) != 1 {
		t.Fatalf("Configure() produced %+v", logger)
	}
	logger.Info("configured")
	if !strings.Contains(output.String(), "configured") {
		t.Fatalf("configured writer output was %q", output.String())
	}

	Configure(&Config{Filename: ".", DefaultLevel: "INFO"})
	if len(defaultWriter) != 0 {
		t.Fatal("discard configuration retained writers")
	}
	Configure(&Config{Filename: "-", DefaultLevel: "INFO"})
	if len(defaultWriter) != 1 || !defaultWriter[0].isTerm {
		t.Fatal("stdout configuration did not create a terminal writer")
	}

	fileLogger := NewLogFile("discard", LogFileConf{Filename: ".", Level: "WARN", PrefixWidth: 7}).(*levelLogger)
	if fileLogger.Level() != LevelWarn || fileLogger.PrefixWidth() != 7 || len(fileLogger.underlying) != 0 {
		t.Fatalf("NewLogFile(discard) produced %+v", fileLogger)
	}
}

func TestSlogWrapper(t *testing.T) {
	var output bytes.Buffer
	accepted := true
	logger := NewLog("slog", &output)
	logger.SetLevel(LevelTrace)
	slogger := Wrap(logger, func(name string, ctx context.Context, record slog.Record) bool {
		return name == "slog" && accepted
	})
	slogger.Info("message", "key", "value")
	if !strings.Contains(output.String(), "message") || !strings.Contains(output.String(), "key=value") {
		t.Fatalf("slog output was %q", output.String())
	}
	before := output.Len()
	accepted = false
	slogger.Warn("filtered")
	if output.Len() != before {
		t.Fatal("slog filter did not suppress the record")
	}

	handler := logger.(*levelLogger)
	if handler.Enabled(context.Background(), slog.Level(-8)) || !handler.Enabled(context.Background(), slog.LevelDebug) || !handler.Enabled(context.Background(), slog.LevelInfo) || !handler.Enabled(context.Background(), slog.LevelWarn) || !handler.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("slog Enabled() returned unexpected values")
	}
	withAttrs := handler.WithAttrs([]slog.Attr{slog.String("a", "b")})
	if len(withAttrs.(*levelLogger).attrs) != 1 {
		t.Fatal("WithAttrs() did not retain attributes")
	}
	if handler.WithGroup("group") == nil || handler.WithGroup("") == nil {
		t.Fatal("WithGroup() returned nil")
	}
}

type testCloser struct {
	closed bool
	err    error
}

func (closer *testCloser) Close() error {
	closer.closed = true
	return closer.err
}
