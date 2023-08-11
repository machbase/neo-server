package test

import (
	"testing"

	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/booter"
	"github.com/machbase/neo-server/mods/args"
	spi "github.com/machbase/neo-spi"
	"github.com/rogpeppe/go-internal/testscript"
)

func TestScenario(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "scenario",
	})
}

func TestMain(m *testing.M) {
	testscript.RunMain(m, map[string]func() int{
		"machbase-neo": args.Main,
	})

	builder := booter.NewBuilder()
	b, err := builder.BuildWithContent(serverConf)
	if err != nil {
		panic(err)
	}
	err = b.Startup()
	if err != nil {
		panic(err)
	}

	/// preparing benchmark table
	_, err = spi.NewDatabase(mach.FactoryName)
	if err != nil {
		panic(err)
	}

	m.Run()
	b.Shutdown()
}

var serverConf = []byte(`
define VARS {
	WORKDIR = "./tmp"
}

module "machbase.com/neo-logging" {
    config {
        Console                     = false
        Filename                    = "-"
        DefaultPrefixWidth          = 30
        DefaultEnableSourceLocation = true
        DefaultLevel                = "TRACE"
        Levels = [
            { Pattern="machsvr", Level="TRACE" },
        ]
    }
}

module "machbase.com/neo-server" {
    name = "machsvr"
    config {
		PrefDir  = "${VARS_WORKDIR}/machbase_pref"
        DataDir  = "${VARS_WORKDIR}/machbase_home"
        Machbase = {
			PORT_NO = 5566
            HANDLE_LIMIT = 4096
			TAG_CACHE_MAX_MEMORY_SIZE = 536870912
			DISK_BUFFER_COUNT = 1
			TAG_CACHE_ENABLE = 3
        }
        Grpc = {
            Listeners        = [ 
                "unix://${VARS_WORKDIR}/mach.sock", 
                "tcp://127.0.0.1:6565",
            ]
            MaxRecvMsgSize   = 4
            MaxSendMsgSize   = 4
        }
        Http = {
            Listeners        = [ "tcp://127.0.0.1:5654" ]
            Handlers         = [
                { Prefix: "/db",       Handler: "machbase" },
                { Prefix: "/metrics",  Handler: "influx" },
				{ Prefix: "/logvault", Handler: "logvault"},
            ]
        }
        Mqtt = {
            Listeners        = [ "tcp://127.0.0.1:5653"]
            Handlers         = [
                { Prefix: "db",      Handler: "machbase" },
                { Prefix: "metrics", Handler: "influx" },
            ]
        }
    }
}
`)
