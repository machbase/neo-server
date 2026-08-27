package publisher

import (
	"context"
	"testing"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/stretchr/testify/require"
)

type publisherProviderStub struct {
	definition *model.BridgeDefinition
}

func (p *publisherProviderStub) LoadBridge(context.Context, model.UserScope, string) (*model.BridgeDefinition, error) {
	return p.definition, nil
}
func (p *publisherProviderStub) LoadAllBridgesForBootstrap(context.Context) ([]*model.BridgeDefinition, error) {
	return nil, nil
}
func (p *publisherProviderStub) LoadAllBridges(context.Context, model.UserScope) ([]*model.BridgeDefinition, error) {
	return nil, nil
}
func (p *publisherProviderStub) SaveBridge(context.Context, model.UserScope, *model.BridgeDefinition) error {
	return nil
}
func (p *publisherProviderStub) RemoveBridge(context.Context, model.UserScope, string) error {
	return nil
}

func TestModulePublisherBridgeErrors(t *testing.T) {
	bridge.SetBridgeProvider(nil)
	runtime := goja.New()
	module := runtime.NewObject()
	module.Set("exports", runtime.NewObject())
	Module(context.Background(), runtime, module)

	exports := module.Get("exports").(*goja.Object)
	publisher, ok := goja.AssertFunction(exports.Get("publisher"))
	require.True(t, ok)

	value, err := publisher(exports)
	require.NoError(t, err)
	require.Contains(t, value.String(), "publisher: bridge '' not found")
	require.NotNil(t, value)
}

func TestModulePublisherUnsupportedBridge(t *testing.T) {
	provider := &publisherProviderStub{definition: &model.BridgeDefinition{Id: 91, Name: "sqlite"}}
	bridge.SetBridgeProvider(provider)
	require.NoError(t, bridge.RegisterByID(&model.BridgeDefinition{Id: 91, Type: model.BRIDGE_SQLITE, Name: "sqlite", Path: ":memory:"}))
	t.Cleanup(func() {
		bridge.UnregisterAll()
		bridge.SetBridgeProvider(nil)
	})

	runtime := goja.New()
	module := runtime.NewObject()
	module.Set("exports", runtime.NewObject())
	Module(context.Background(), runtime, module)
	exports := module.Get("exports").(*goja.Object)
	publisher, ok := goja.AssertFunction(exports.Get("publisher"))
	require.True(t, ok)

	value, err := publisher(exports, runtime.ToValue(map[string]any{"bridge": "sqlite"}))
	require.NoError(t, err)
	require.Contains(t, value.String(), "publisher: bridge 'sqlite' not supported")
	require.NotNil(t, value)
}
