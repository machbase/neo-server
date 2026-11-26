package server

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/machbase/neo-server/v8/mods"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/spi/machsvr"
	"github.com/zclconf/go-cty/cty"
)

const (
	loggingModuleID = "machbase.com/neo-logging"
	serverModuleID  = "machbase.com/neo-server"
)

type serverExecution struct {
	args       []string
	preset     string
	headless   bool
	options    serverBootOptions
	logger     *log.Logger
	requested  chan struct{}
	signals    chan os.Signal
	server     *Server
	startTried bool

	shutdownOnce sync.Once
	shutdownDone chan struct{}
}

func newServerExecution(args []string, preset string, headless bool, options serverBootOptions, logger *log.Logger, requested chan struct{}) *serverExecution {
	if requested == nil {
		requested = make(chan struct{}, 1)
	}
	execution := &serverExecution{
		args:         append([]string(nil), args...),
		preset:       preset,
		headless:     headless,
		options:      options,
		logger:       logger,
		requested:    requested,
		signals:      make(chan os.Signal, 1),
		shutdownDone: make(chan struct{}),
	}
	signal.Notify(execution.signals, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	return execution
}

func (execution *serverExecution) requestShutdown() {
	select {
	case execution.requested <- struct{}{}:
	default:
	}
}

func (execution *serverExecution) waitSignal() {
	select {
	case <-execution.signals:
	case <-execution.requested:
	case <-execution.shutdownDone:
	}
}

func (execution *serverExecution) shutdown() {
	execution.shutdownOnce.Do(func() {
		signal.Stop(execution.signals)
		if execution.startTried && execution.server != nil {
			execution.server.Stop()
		}
		close(execution.shutdownDone)
	})
}

func (execution *serverExecution) loadConfig() error {
	loggingConfig, serverConfig, err := loadServerConfiguration(
		execution.args,
		execution.preset,
		execution.headless,
		execution.options,
		execution.requestShutdown,
	)
	if err != nil {
		return err
	}
	if loggingConfig != nil {
		logging.Configure(loggingConfig)
	}

	server, err := NewServer(serverConfig)
	if err != nil {
		return err
	}
	execution.server = server
	return nil
}

func loadServerConfiguration(
	args []string,
	preset string,
	headless bool,
	options serverBootOptions,
	requestShutdown func(),
) (*logging.Config, *Config, error) {
	context := configEvalContext{
		Args:      append([]string(nil), args...),
		Arguments: options.arguments,
		Pname:     options.pname,
		Version:   mods.VersionString() + " " + machsvr.LinkInfo(),
	}
	if context.Arguments == nil {
		context.Arguments = scanServerArguments(args)
	}
	definitions, err := loadServerConfigDefinitions(context, options)
	if err != nil {
		return nil, nil, err
	}

	loggingDefinition, serverDefinition, err := selectServerModules(definitions)
	if err != nil {
		return nil, nil, err
	}
	if serverDefinition.Disabled {
		return nil, nil, fmt.Errorf("module %s is disabled; server must be enabled", serverModuleID)
	}
	if err := validateModuleOrder(loggingDefinition, serverDefinition); err != nil {
		return nil, nil, err
	}

	serverConfig := newConfig(preset, headless)
	for _, arg := range context.Arguments {
		if arg.name == "--file" {
			serverConfig.fileFlagSet = true
			break
		}
	}
	serverConfig.notifyShutdown = requestShutdown
	if hasModuleConfig(serverDefinition) {
		if err := evalConfigObject("Config", serverConfig, serverDefinition.Config); err != nil {
			return nil, nil, fmt.Errorf("config %s: %w", serverModuleID, err)
		}
	}

	var loggingConfig *logging.Config
	if loggingDefinition != nil && !loggingDefinition.Disabled {
		loggingConfig = logging.DefaultConfig()
		if hasModuleConfig(loggingDefinition) {
			if err := evalConfigObject("Config", loggingConfig, loggingDefinition.Config); err != nil {
				return nil, nil, fmt.Errorf("config %s: %w", loggingModuleID, err)
			}
		}
	}
	return loggingConfig, serverConfig, nil
}

func hasModuleConfig(definition *configModuleDefinition) bool {
	return definition.Config.Type() != cty.NilType && !definition.Config.IsNull()
}

func validateModuleOrder(loggingDefinition, serverDefinition *configModuleDefinition) error {
	if loggingDefinition != nil && !loggingDefinition.Disabled && loggingDefinition.Priority >= serverDefinition.Priority {
		return fmt.Errorf("module %s must have a lower priority than %s", loggingModuleID, serverModuleID)
	}
	return nil
}

func selectServerModules(definitions []*configModuleDefinition) (*configModuleDefinition, *configModuleDefinition, error) {
	var loggingDefinition *configModuleDefinition
	var serverDefinition *configModuleDefinition
	for _, definition := range definitions {
		switch definition.Id {
		case loggingModuleID:
			if loggingDefinition != nil {
				return nil, nil, fmt.Errorf("duplicate module %s", definition.Id)
			}
			loggingDefinition = definition
		case serverModuleID:
			if serverDefinition != nil {
				return nil, nil, fmt.Errorf("duplicate module %s", definition.Id)
			}
			serverDefinition = definition
		default:
			if definition.Disabled {
				continue
			}
			return nil, nil, fmt.Errorf("unsupported module %s", definition.Id)
		}
		if !definition.Disabled && len(definition.Injects) != 0 {
			return nil, nil, fmt.Errorf("module %s uses unsupported dependency injection", definition.Id)
		}
	}
	if serverDefinition == nil {
		return nil, nil, fmt.Errorf("configuration must define module %s", serverModuleID)
	}
	return loggingDefinition, serverDefinition, nil
}

func (execution *serverExecution) start() error {
	if execution.server == nil {
		return errors.New("server configuration is not loaded")
	}
	execution.logger.Println("startup", execution.options.pname)
	execution.logger.Println("start", serverModuleID, "neosvr")
	execution.startTried = true
	if err := execution.server.Start(); err != nil {
		return fmt.Errorf("mod start %s: %w", serverModuleID, err)
	}
	return nil
}
