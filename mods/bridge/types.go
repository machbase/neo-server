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

type MqttBridge interface {
	Bridge
	OnConnect(cb func(bridge any))
	OnDisconnect(cb func(bridge any))
	Subscribe(topic string, qos byte, cb func(topic string, payload []byte, msgId int, dup bool, retained bool)) (bool, error)
	Unsubscribe(topics ...string) (bool, error)
	Publish(topic string, payload any) (bool, error)
	IsConnected() bool
}

type NatsBridge interface {
	Bridge
	Subscribe(topic string, cb func(topic string, data []byte, header map[string][]string, respond func([]byte))) (bool, error)
	Unsubscribe(topic string) (bool, error)
	Publish(topic string, payload any) (bool, error)
}

type PythonBridge interface {
	Bridge
	Invoke(ctx context.Context, args []string, stdin []byte) (exitCode int, stdout []byte, stderr []byte, err error)
	Version(ctx context.Context) (string, error)
}
