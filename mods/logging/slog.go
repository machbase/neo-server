package logging

import (
	"context"
	"fmt"
	"log/slog"
)

func Wrap(l Log, filter func(string, context.Context, slog.Record) bool) *slog.Logger {
	if h := l.(*levelLogger); h != nil {
		h.filter = filter
		return slog.New(h)
	} else {
		return slog.Default()
	}
}

// Enabled reports whether the handler handles records at the given level.
// The handler ignores records whose level is lower.
// It is called early, before any arguments are processed,
// to save effort if the log event should be discarded.
// If called from a Logger method, the first argument is the context
// passed to that method, or context.Background() if nil was passed
// or the method does not take a context.
// The context is passed so Enabled can use its values
// to make a decision.
func (ll *levelLogger) Enabled(ctx context.Context, level slog.Level) bool {
	switch level {
	case slog.LevelDebug:
		return ll.TraceEnabled() || ll.DebugEnabled()
	case slog.LevelInfo:
		return ll.InfoEnabled()
	case slog.LevelWarn:
		return ll.WarnEnabled()
	case slog.LevelError:
		return ll.ErrorEnabled()
	}
	return false
}

// Handle handles the Record.
// It will only be called when Enabled returns true.
// The Context argument is as for Enabled.
// It is present solely to provide Handlers access to the context's values.
// Canceling the context should not affect record processing.
// (Among other things, log messages may be necessary to debug a
// cancellation-related problem.)
//
// Handle methods that produce output should observe the following rules:
//   - If r.Time is the zero time, ignore the time.
//   - If r.PC is zero, ignore it.
//   - Attr's values should be resolved.
//   - If an Attr's key and value are both the zero value, ignore the Attr.
//     This can be tested with attr.Equal(Attr{}).
//   - If a group's key is empty, inline the group's Attrs.
//   - If a group has no Attrs (even if it has a non-empty key),
//     ignore it.
func (ll *levelLogger) Handle(ctx context.Context, r slog.Record) error {
	lvl := LevelDebug
	switch r.Level {
	case slog.LevelDebug:
		lvl = LevelDebug
	case slog.LevelInfo:
		lvl = LevelInfo
	case slog.LevelWarn:
		lvl = LevelWarn
	default:
		lvl = LevelError
	}
	if ll.filter != nil && !ll.filter(ll.name, ctx, r) {
		return nil
	}
	args := []any{r.Message}
	r.Attrs(func(a slog.Attr) bool {
		args = append(args, fmt.Sprintf("%v=%v", a.Key, a.Value))
		return true
	})
	ll._log(lvl, 0, args)
	return nil
}

// WithAttrs returns a new Handler whose attributes consist of
// both the receiver's attributes and the arguments.
// The Handler owns the slice: it may retain, modify or discard it.
func (ll *levelLogger) WithAttrs(attrs []slog.Attr) slog.Handler {
	ret := &levelLogger{
		name:         ll.name,
		level:        ll.level,
		underlying:   ll.underlying,
		prefixWidth:  ll.prefixWidth,
		enableSrcLoc: ll.enableSrcLoc,
		attrs:        append(ll.attrs, attrs...),
	}
	return ret
}

// WithGroup returns a new Handler with the given group appended to
// the receiver's existing groups.
// The keys of all subsequent attributes, whether added by With or in a
// Record, should be qualified by the sequence of group names.
//
// How this qualification happens is up to the Handler, so long as
// this Handler's attribute keys differ from those of another Handler
// with a different sequence of group names.
//
// A Handler should treat WithGroup as starting a Group of Attrs that ends
// at the end of the log event. That is,
//
//	logger.WithGroup("s").LogAttrs(level, msg, slog.Int("a", 1), slog.Int("b", 2))
//
// should behave like
//
//	logger.LogAttrs(level, msg, slog.Group("s", slog.Int("a", 1), slog.Int("b", 2)))
//
// If the name is empty, WithGroup returns the receiver.
func (ll *levelLogger) WithGroup(name string) slog.Handler {
	if r, ok := GetLog(name).(*levelLogger); ok {
		return r
	}
	return ll
}
