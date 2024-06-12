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
	Connect(ctx context.Context) (*sql.Conn, error)
	NewScanType(reflectType string, databaseTypeName string) any
	NormalizeType(value []any) []any
	ParameterMarker(idx int) string
	SupportLastInsertId() bool
}

type PythonBridge interface {
	Bridge
	Invoke(ctx context.Context, args []string, stdin []byte) (exitCode int, stdout []byte, stderr []byte, err error)
	Version(ctx context.Context) (string, error)
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
