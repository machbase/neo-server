package plugins

import (
	"fmt"

	"github.com/machbase/neo-server/plugins/bridge"
	"github.com/pkg/errors"
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
		return errors.New(fmt.Sprintf("plugin '%s' nil factory", pluginId))
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
