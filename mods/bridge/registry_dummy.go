//go:build linux && arm
// +build linux,arm

package bridge

import (
	"github.com/machbase/neo-server/mods/model"
)

func Register(def *model.BridgeDefinition) (err error) {
	return ErrBridgeDisabled
}

func Unregister(name string) {
}

func UnregisterAll() {
}

func GetBridge(name string) (Bridge, error) {
	return nil, ErrBridgeDisabled
}

func GetSqlBridge(name string) (SqlBridge, error) {
	return nil, ErrBridgeDisabled
}

func GetMqttBridge(name string) (MqttBridge, error) {
	return nil, ErrBridgeDisabled
}
