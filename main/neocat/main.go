package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/machbase/neo-client/pkg/pstag"
	"github.com/machbase/neo-client/pkg/pstag/plugin"
	"github.com/machbase/neo-client/pkg/util"
)

var usageStr = `
Usage: neocat [options]

Options:
	--help, -h              Show this help message
	--interval <duration>   The interval to report the statistics (default: 10s, min: 1s)
	--tag-prefix <prefix>   The prefix of the tag name
	--pid <path>            Write PID file
	--log-filename <path>   Log file path (default: -)
	--log-level <level>     Log level (default: INFO), DEBUG, INFO, WARN, ERROR
	--log-max-size <size>   Maximum size of the log file in MB (default: 100)
	--log-max-age <days>    Maximum days to retain old log files (default: 7)
	--log-max-backups <n>   Maximum number of old log files to retain (default: 10)
	--log-compress          Compress the old log files (default: false)
`

func usage() {
	fmt.Printf("%s\n", strings.ReplaceAll(usageStr, "\t", "    "))
	plugin.PrintUsage()
	os.Exit(0)
}

func main() {
	optInterval := flag.Duration("interval", 10*time.Second, "The interval to report the statistics")
	optPid := flag.String("pid", "", "pid file")
	optTagPrefix := flag.String("tag-prefix", "", "tag prefix")
	optLogFilename := flag.String("log-filename", "-", "log file path")
	optLogLevel := flag.String("log-level", "INFO", "log level")
	optLogMaxSize := flag.Int("log-max-size", 100, "maximum size of the log file in MB")
	optLogMaxAge := flag.Int("log-max-age", 7, "maximum number of days to retain old log files")
	optLogMaxBackups := flag.Int("log-max-backups", 10, "maximum number of old log files to retain")
	optLogCompress := flag.Bool("log-compress", false, "compress the log backup files")

	optInputs := map[string]any{}
	for _, name := range plugin.GetInletNames() {
		reg := plugin.GetInletRegistry(name)
		switch def := reg.ArgDefault.(type) {
		case string:
			optInputs[name] = flag.String(name, def, reg.ArgDesc)
		case bool:
			optInputs[name] = flag.Bool(name, def, reg.ArgDesc)
		}
	}

	optOutputs := map[string]any{}
	for _, name := range plugin.GetOutletNames() {
		reg := plugin.GetOutletRegistry(name)
		switch def := reg.ArgDefault.(type) {
		case string:
			optOutputs[name] = flag.String(name, def, reg.ArgDesc)
		case bool:
			optOutputs[name] = flag.Bool(name, def, reg.ArgDesc)
		}
	}
	flag.Usage = usage
	flag.Parse()

	if *optLogFilename != "" {
		util.InitLogger(*optLogFilename, *optLogLevel, *optLogMaxSize, *optLogMaxAge, *optLogMaxBackups, *optLogCompress)
	}

	// NavelCord
	var navelcord *Navelcord
	if port := os.Getenv(NAVEL_ENV); port != "" {
		if port, err := strconv.ParseInt(port, 10, 64); err == nil {
			navelcord = &Navelcord{port: int(port)}
		}
	}

	// PID file
	if optPid != nil {
		pfile, _ := os.OpenFile(*optPid, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		pfile.WriteString(fmt.Sprintf("%d", os.Getpid()))
		pfile.Close()
		defer func() {
			os.Remove(*optPid)
		}()
	}

	process := pstag.New(
		pstag.WithInterval(*optInterval),
		pstag.WithTagPrefix(*optTagPrefix),
	)

	for name, opt := range optInputs {
		switch v := opt.(type) {
		case *bool:
			if *v {
				process.AddInput(plugin.NewInlet(name))
			}
		case *string:
			if *v != "" {
				process.AddInput(plugin.NewInlet(name, *v))
			}
		}
	}

	for name, opt := range optOutputs {
		switch v := opt.(type) {
		case *bool:
			if *v {
				process.AddOutput(plugin.NewOutlet(name))
			}
		case *string:
			if *v != "" {
				process.AddOutput(plugin.NewOutlet(name, *v))
			}
		}
	}

	// NavelCord
	if navelcord != nil {
		if err := navelcord.StartNavelCord(); err != nil {
			os.Exit(1)
		}
	}

	go process.Run()

	// wait Ctrl+C
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	fmt.Println("started, press ctrl+c to stop...")
	<-done

	if navelcord != nil {
		navelcord.StopNavelCord()
	}
	process.Stop()
}
