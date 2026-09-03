package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"

	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/spi"
)

type UserScope struct {
	User string
}

type userScopeContextKey struct{}

// ContextWithUserScope returns a copy of ctx that carries a fixed scope,
// retrievable via UserScopeFromContext. Used to thread the calling user's
// identity into jsh native modules (e.g. @jsh/db, @jsh/publisher) invoked
// from TQL's SCRIPT({}) node, which otherwise have no way to resolve it from
// ctx alone.
func ContextWithUserScope(ctx context.Context, scope UserScope) context.Context {
	return ContextWithUserScopeFunc(ctx, func() UserScope { return scope })
}

// ContextWithUserScopeFunc is like ContextWithUserScope, but scope is
// resolved lazily by calling fn on every UserScopeFromContext lookup instead
// of being snapshotted once. Use this when the identity can change after the
// context is created (e.g. jsh's interactive `connect`/`login` shell command
// re-authenticating mid-session via jsh/session.SwitchUser) so that later
// lookups see the current identity instead of a stale one baked in at
// context-creation time.
func ContextWithUserScopeFunc(ctx context.Context, fn func() UserScope) context.Context {
	return context.WithValue(ctx, userScopeContextKey{}, fn)
}

// UserScopeFromContext retrieves the UserScope previously attached via
// ContextWithUserScope or ContextWithUserScopeFunc. ok is false if ctx
// carries no scope.
func UserScopeFromContext(ctx context.Context) (scope UserScope, ok bool) {
	fn, ok := ctx.Value(userScopeContextKey{}).(func() UserScope)
	if !ok {
		return UserScope{}, false
	}
	return fn(), true
}

type connectFunc func(context.Context, string) (*sql.Conn, error)

func NewProvider(opts ...Option) *Provider {
	ret := &Provider{
		log:     logging.GetLog("model"),
		connect: spi.Connect,
	}
	for _, o := range opts {
		o(ret)
	}
	return ret
}

type Option func(*Provider)

type Provider struct {
	log logging.Log

	connect connectFunc

	shellTableMu    sync.Mutex
	shellTableReady bool

	bridgeTableMu    sync.Mutex
	bridgeTableReady bool

	timerTableMu    sync.Mutex
	timerTableReady bool

	subscriberTableMu    sync.Mutex
	subscriberTableReady bool

	apiTokenTableMu    sync.Mutex
	apiTokenTableReady bool

	x509CertTableMu    sync.Mutex
	x509CertTableReady bool

	experimentMode func() bool
}

func WithExperimentModeProvider(provider func() bool) Option {
	return func(s *Provider) {
		s.experimentMode = provider
	}
}

// scheduleRowScanner is satisfied by both *sql.Row and *sql.Rows.
type scheduleRowScanner interface {
	Scan(dest ...any) error
}

// scheduleConn opens a SYS connection and ensures both the timer and the
// subscriber tables exist.
func (s *Provider) scheduleConn(ctx context.Context) (*sql.Conn, error) {
	if err := s.normalizeContext(ctx); err != nil {
		return nil, err
	}
	if s.connect == nil {
		return nil, errors.New("database connect function is not configured")
	}
	conn, err := s.connect(ctx, "sys")
	if err != nil {
		return nil, err
	}
	if err := s.ensureTimerTable(ctx, conn); err != nil {
		conn.Close()
		return nil, err
	}
	if err := s.ensureSubscriberTable(ctx, conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// attributesToDB converts a forward-compatible JSON attributes blob into a
// value bindable to the ATTRIBUTES column, or nil when empty.
func attributesToDB(attr json.RawMessage) any {
	if len(attr) == 0 {
		return nil
	}
	return string(attr)
}
