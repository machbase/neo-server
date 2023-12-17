package facility

import "testing"

type Logger interface {
	Logf(format string, args ...any)
	Log(args ...any)
	LogDebugf(format string, args ...any)
	LogDebug(args ...any)
	LogWarnf(format string, args ...any)
	LogWarn(args ...any)
	LogErrorf(format string, args ...any)
	LogError(args ...any)
}

var DiscardLogger = &discardLogger{}

type discardLogger struct {
}

func (l *discardLogger) Logf(format string, args ...any)      {}
func (l *discardLogger) Log(args ...any)                      {}
func (l *discardLogger) LogDebugf(format string, args ...any) {}
func (l *discardLogger) LogDebug(args ...any)                 {}
func (l *discardLogger) LogWarnf(format string, args ...any)  {}
func (l *discardLogger) LogWarn(args ...any)                  {}
func (l *discardLogger) LogErrorf(format string, args ...any) {}
func (l *discardLogger) LogError(args ...any)                 {}

type testLogger struct {
	t *testing.T
}

func TestLogger(t *testing.T) Logger {
	return &testLogger{t}
}
func (l *testLogger) Logf(format string, args ...any)      { l.t.Helper(); l.t.Logf(format, args...) }
func (l *testLogger) Log(args ...any)                      { l.t.Helper(); l.t.Log(args...) }
func (l *testLogger) LogDebugf(format string, args ...any) { l.t.Helper(); l.t.Logf(format, args...) }
func (l *testLogger) LogDebug(args ...any)                 { l.t.Helper(); l.t.Log(args...) }
func (l *testLogger) LogWarnf(format string, args ...any)  { l.t.Helper(); l.t.Logf(format, args...) }
func (l *testLogger) LogWarn(args ...any)                  { l.t.Helper(); l.t.Log(args...) }
func (l *testLogger) LogErrorf(format string, args ...any) { l.t.Helper(); l.t.Logf(format, args...) }
func (l *testLogger) LogError(args ...any)                 { l.t.Helper(); l.t.Log(args...) }
