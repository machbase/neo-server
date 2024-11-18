package logging

import (
	"github.com/machbase/neo-server/v8/booter"
)

const ModuleId = "machbase.com/neo-logging"

type Module struct {
}

func (m *Module) Start() error {
	return nil
}

func (m *Module) Stop() {
}

func init() {
	RegisterBootFactory()
}

func RegisterBootFactory() {
	defaultConf := Config{
		Console:                     false,
		Filename:                    "-",
		Append:                      true,
		RotateSchedule:              "@midnight",
		MaxSize:                     10,
		MaxBackups:                  1,
		MaxAge:                      7,
		Compress:                    false,
		UTC:                         false,
		DefaultPrefixWidth:          10,
		DefaultEnableSourceLocation: false,
		DefaultLevel:                "TRACE",
	}

	booter.Register(ModuleId,
		func() *Config {
			clone := defaultConf
			return &clone
		},
		func(conf *Config) (booter.Boot, error) {
			Configure(conf)
			return &Module{}, nil
		},
	)
}
