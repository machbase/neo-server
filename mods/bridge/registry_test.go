package bridge_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/stretchr/testify/require"
)

func bridgeSqlitePath(t *testing.T) string {
	t.Helper()
	return "file:" + filepath.Join(t.TempDir(), "registry.db") + "?cache=shared"
}

func TestRegistryGettersAndUnsupportedType(t *testing.T) {
	bridge.UnregisterAll()

	sqliteName := "registry_sqlite"
	mqttName := "registry_mqtt"

	// Call bridgeSqlitePath (which calls t.TempDir) before t.Cleanup so that
	// the TempDir cleanup is registered first. LIFO ordering then ensures
	// UnregisterAll (and its db.Close) runs before the temp dir is removed.
	// On Windows this prevents "file used by another process" errors.
	sqlitePath := bridgeSqlitePath(t)
	t.Cleanup(bridge.UnregisterAll)

	require.NoError(t, bridge.Register(&model.BridgeDefinition{
		Name: sqliteName,
		Type: model.BRIDGE_SQLITE,
		Path: sqlitePath,
	}))
	require.NoError(t, bridge.Register(&model.BridgeDefinition{
		Name: mqttName,
		Type: model.BRIDGE_MQTT,
		Path: "",
	}))

	sqlBr, err := bridge.GetSqlBridge(sqliteName)
	require.NoError(t, err)
	require.Equal(t, sqliteName, sqlBr.Name())

	_, err = bridge.GetSqlBridge(mqttName)
	require.EqualError(t, err, fmt.Sprintf("'%s' is not a SqlBridge", mqttName))

	mqttBr, err := bridge.GetMqttBridge(mqttName)
	require.NoError(t, err)
	require.Equal(t, mqttName, mqttBr.Name())

	_, err = bridge.GetMqttBridge(sqliteName)
	require.EqualError(t, err, fmt.Sprintf("'%s' is not a MqttBridge", sqliteName))

	err = bridge.Register(&model.BridgeDefinition{
		Name: "unsupported",
		Type: model.BridgeType("unsupported"),
		Path: "ignored",
	})
	require.EqualError(t, err, "undefined bridge type unsupported, unable to register")
}
