package plugins

import (
	"github.com/machbase/neo-server/plugins/bridge"
	"github.com/machbase/neo-server/plugins/instancemgr"
)

type PluginInstanceFactoryFunc func(settings PluginInstanceSettings) (bridge.PluginInstance, error)

func NewInstanceManager(fn PluginInstanceFactoryFunc) instancemgr.InstanceManager {
	ip := NewInstanceProvider(fn)
	return instancemgr.New(ip)
}

func NewInstanceProvider(fn PluginInstanceFactoryFunc) instancemgr.InstanceProvider {
	return &instanceProvider{
		fn: fn,
	}
}

type instanceProvider struct {
	fn PluginInstanceFactoryFunc
}

func (ip *instanceProvider) NeedsUpdate(ctx bridge.PluginContext) bool {
	return false
}

func (ip *instanceProvider) NewInstance(ctx bridge.PluginContext) (bridge.PluginInstance, error) {
	return nil, nil
}
