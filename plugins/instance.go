package plugins

import (
	"errors"
	"fmt"

	"github.com/machbase/neo-server/plugins/bridge"
)

type PluginInstanceSettings struct {
	Config any
}

type ManageOptions struct {
	bridge.GrpcSettings
}

func Manage(pluginId string, instanceFactory PluginInstanceFactoryFunc, options ManageOptions) error {
	if pluginId == "" {
		return errors.New("invalid pluginId")
	}
	if instanceFactory == nil {
		return fmt.Errorf("plugin '%s' nil factory", pluginId)
	}

	_, err := GetArgs(pluginId)
	if err != nil {
		return err
	}

	handler := NewInstanceManager(instanceFactory)
	serveOpts := bridge.ServeOptions{
		GrpcSettings: options.GrpcSettings,
		QueryHandler: handler,
	}
	return bridge.Serve(serveOpts)
}

type Args struct {
	Address string
	dir     string
}

func GetArgs(pluginId string) (Args, error) {
	return Args{}, nil
}
