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
// subscriber tables exist, since schedule names share a single global
// namespace across the two tables.
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

// scheduleNameExists reports whether name is already used by a timer or a
// subscriber definition. Schedule names are a single global namespace
// shared across both tables, so callers must check both before inserting.
func (s *Provider) scheduleNameExists(ctx context.Context, conn *sql.Conn, name string) (bool, error) {
	var count int64
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM _NEO_TIMER_DEF WHERE NAME = ?`, name).Scan(&count); err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM _NEO_SUBSCRIBER_DEF WHERE NAME = ?`, name).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// attributesToDB converts a forward-compatible JSON attributes blob into a
// value bindable to the ATTRIBUTES column, or nil when empty.
func attributesToDB(attr json.RawMessage) any {
	if len(attr) == 0 {
		return nil
	}
	return string(attr)
}
