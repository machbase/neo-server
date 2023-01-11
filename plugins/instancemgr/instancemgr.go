package instancemgr

import (
	"github.com/machbase/neo-server/plugins/bridge"
)

type InstanceManager interface {
}

func New(ip InstanceProvider) InstanceManager {
	return nil
}

type InstanceProvider interface {
	NeedsUpdate(pluginCtx bridge.PluginContext) bool
	NewInstance(pluginCtx bridge.PluginContext) (bridge.PluginInstance, error)
}
