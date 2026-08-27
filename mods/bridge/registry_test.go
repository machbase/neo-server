package bridge_test

import (
	"context"
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

// registryDefProviderStub maps a bridge name to its BridgeDefinition without
// any owner/scope enforcement, for tests that only exercise the runtime
// registry behavior.
type registryDefProviderStub struct {
	defs map[string]*model.BridgeDefinition
}

func (p *registryDefProviderStub) LoadBridge(ctx context.Context, scope model.UserScope, name string) (*model.BridgeDefinition, error) {
	def, ok := p.defs[name]
	if !ok {
		return nil, fmt.Errorf("undefined bridge name '%s'", name)
	}
	return def, nil
}

func (p *registryDefProviderStub) LoadAllBridgesForBootstrap(ctx context.Context) ([]*model.BridgeDefinition, error) {
	return []*model.BridgeDefinition{}, nil
}
func (p *registryDefProviderStub) LoadAllBridges(ctx context.Context, scope model.UserScope) ([]*model.BridgeDefinition, error) {
	return []*model.BridgeDefinition{}, nil
}
func (p *registryDefProviderStub) SaveBridge(ctx context.Context, scope model.UserScope, def *model.BridgeDefinition) error {
	return nil
}
func (p *registryDefProviderStub) RemoveBridge(ctx context.Context, scope model.UserScope, name string) error {
	return nil
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

	ctx := context.Background()
	scope := model.UserScope{User: "sys"}

	sqliteDef := &model.BridgeDefinition{
		Id:   1,
		Name: sqliteName,
		Type: model.BRIDGE_SQLITE,
		Path: sqlitePath,
	}
	mqttDef := &model.BridgeDefinition{
		Id:   2,
		Name: mqttName,
		Type: model.BRIDGE_MQTT,
		Path: "",
	}
	bridge.SetBridgeProvider(&registryDefProviderStub{defs: map[string]*model.BridgeDefinition{
		sqliteName: sqliteDef,
		mqttName:   mqttDef,
	}})

	require.NoError(t, bridge.RegisterByID(sqliteDef))
	require.NoError(t, bridge.RegisterByID(mqttDef))

	sqlBr, err := bridge.GetSqlBridge(ctx, scope, sqliteName)
	require.NoError(t, err)
	require.Equal(t, sqliteName, sqlBr.Name())

	_, err = bridge.GetSqlBridge(ctx, scope, mqttName)
	require.EqualError(t, err, fmt.Sprintf("'%s' is not a SqlBridge", mqttName))

	mqttBr, err := bridge.GetMqttBridge(ctx, scope, mqttName)
	require.NoError(t, err)
	require.Equal(t, mqttName, mqttBr.Name())

	_, err = bridge.GetMqttBridge(ctx, scope, sqliteName)
	require.EqualError(t, err, fmt.Sprintf("'%s' is not a MqttBridge", sqliteName))

	err = bridge.RegisterByID(&model.BridgeDefinition{
		Id:   3,
		Name: "unsupported",
		Type: model.BridgeType("unsupported"),
		Path: "ignored",
	})
	require.EqualError(t, err, "undefined bridge type unsupported, unable to register")
}
