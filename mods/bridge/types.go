package bridge

import (
	"context"
	"database/sql"
	"errors"
)

var ErrBridgeDisabled = errors.New("bridge is not enabled")

type Bridge interface {
	Name() string
	String() string

	BeforeRegister() error
	AfterUnregister() error
}

type SqlBridge interface {
	Bridge
	Type() string
	DB() *sql.DB
	Connect(ctx context.Context) (*sql.Conn, error)
	NewScanType(reflectType string, databaseTypeName string) any
	NormalizeType(value []any) []any
	ParameterMarker(idx int) string
	SupportLastInsertId() bool
}

type WriteStats struct {
	Appended uint64
	Inserted uint64
}

type Subscription interface {
	Unsubscribe() error
	AddAppended(delta uint64)
	AddInserted(delta uint64)
}

type BridgeTrafficStats struct {
	InMsgs   uint64
	InBytes  uint64
	OutMsgs  uint64
	OutBytes uint64
	Inserted uint64
	Appended uint64
}

type ConnectionTestBridge interface {
	Bridge
	TestConnection() (bool, string)
}

type StatsBridge interface {
	Bridge
	StatsSnapshot() BridgeTrafficStats
}
