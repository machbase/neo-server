package facility

import "testing"

func TestDiscardLoggerAndTestLogger(t *testing.T) {
	loggers := []Logger{DiscardLogger, TestLogger(t)}
	for _, l := range loggers {
		if l == nil {
			t.Fatal("logger should not be nil")
		}
		l.Logf("hello %s", "world")
		l.Log("plain log")
		l.LogDebugf("debug %d", 1)
		l.LogDebug("debug")
		l.LogWarnf("warn %d", 2)
		l.LogWarn("warn")
		l.LogErrorf("error %d", 3)
		l.LogError("error")
	}
}
