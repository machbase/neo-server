package booter

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/zclconf/go-cty/cty/function"
)

type Config struct {
	Daemon      bool
	BootlogFile string
	PidFile     string
	Pname       string
	ConfDir     string
	ConfFile    string
	GenConfig   bool

	flags         map[BootFlagType]BootFlag
	versionString string
}

type BootFlag struct {
	Long        string
	Short       string
	Placeholder string
	Help        string
	Default     string
}

type BootFlagType int

const (
	noneFlag BootFlagType = iota
	ConfigDirFlag
	ConfigFileFlag
	GenConfigFlag
	PnameFlag
	PidFlag
	BootlogFlag
	DaemonFlag
	HelpFlag
	numofFlags
)

var defaultBooter Booter
var defaultBuilder = NewBuilder()
var conf *Config
var fallbackConfigContent []byte
var fallbackPname string
var bootlog *log.Logger

func init() {
	bootlog = log.New(os.Stdout, "booter ", log.LstdFlags|log.Lmsgprefix)
	conf = &Config{
		flags: map[BootFlagType]BootFlag{
			ConfigDirFlag:  {Long: "config-dir", Placeholder: "<dir>", Help: "config directory path"},
			ConfigFileFlag: {Long: "config", Short: "c", Placeholder: "<file>", Help: "a single file config"},
			GenConfigFlag:  {Long: "gen-config", Help: "print default config"},
			PnameFlag:      {Long: "pname", Placeholder: "<name>", Help: "assign process name"},
			PidFlag:        {Long: "pid", Placeholder: "<path>", Help: "pid file path"},
			BootlogFlag:    {Long: "bootlog", Placeholder: "<path>", Help: "boot log path"},
			DaemonFlag:     {Long: "daemon", Short: "d", Help: "run process in background, daemonize"},
			HelpFlag:       {Long: "help", Short: "h", Help: "print this message"},
		},
	}
}

func Startup() {
	parseflags()

	if conf.GenConfig && len(fallbackConfigContent) > 0 {
		fmt.Println(string(fallbackConfigContent))
		os.Exit(0)
	}

	if conf.Daemon {
		// daemon mode일 때는 bootlog와 pidfile을 Damonize()내에서 처리한다.
		Daemonize(conf.BootlogFile, conf.PidFile, func() { serve(conf) })
		return
	}

	// foreground process mode일 때 bootlog와 pidfile을 생성 한다.
	var writer io.Writer
	if len(conf.BootlogFile) > 0 {
		logfile, _ := os.OpenFile(conf.BootlogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		defer logfile.Close()
		if conf.Daemon {
			writer = logfile
		} else {
			writer = io.MultiWriter(os.Stdout, logfile)
		}
	} else {
		if conf.Daemon {
			writer = io.Discard
		} else {
			writer = os.Stdout
		}
	}
	bootlog = log.New(writer, fmt.Sprintf("boot-%s ", conf.Pname), log.LstdFlags|log.Lmsgprefix)
	bootlog.Println("pid:", os.Getpid())

	if len(conf.PidFile) > 0 && !conf.Daemon {
		pfile, _ := os.OpenFile(conf.PidFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		pfile.WriteString(fmt.Sprintf("%d", os.Getpid()))
		pfile.Close()
	}

	serve(conf)
}

func Shutdown() {
	bootlog.Println("shutdown", conf.Pname)
	defaultBooter.Shutdown()
}

func ShutdownAndExit(exitCode int) {
	if defaultBooter == nil {
		// daemon mode에서 parent process의 경우.
		return
	}
	defaultBooter.ShutdownAndExit(exitCode)
}

func WaitSignal() {
	if defaultBooter == nil {
		// daemon mode에서 parent process의 경우.
		return
	}
	defaultBooter.WaitSignal()
}

func NotifySignal() {
	if defaultBooter == nil {
		// daemon mode에서 parent process의 경우.
		return
	}
	defaultBooter.NotifySignal()
}

func AddStartupHook(hooks ...func()) {
	defaultBuilder.AddStartupHook(hooks...)
}

func AddShutdownHook(hooks ...func()) {
	// booter가 시작되고 나면 builder에 hook을 추가하는 것은 의미가 없다.
	if defaultBooter == nil {
		defaultBuilder.AddShutdownHook(hooks...)
	} else {
		defaultBooter.AddShutdownHook(hooks...)
	}
}

func SetFunction(name string, f function.Function) {
	defaultBuilder.SetFunction(name, f)
}

func SetVariable(name string, value any) error {
	return defaultBuilder.SetVariable(name, value)
}

func SetConfiFileSuffix(ext string) {
	defaultBuilder.SetConfiFileSuffix(ext)
}

func GetDefinition(id string) *Definition {
	if defaultBooter == nil {
		return nil
	} else {
		return defaultBooter.GetDefinition(id)
	}
}

func GetInstance(id string) Boot {
	if defaultBooter == nil {
		return nil
	} else {
		return defaultBooter.GetInstance(id)
	}
}

func GetConfig(id string) any {
	if defaultBooter == nil {
		return nil
	} else {
		return defaultBooter.GetInstance(id)
	}
}

func Pname() string {
	if conf == nil {
		return ""
	}
	return conf.Pname
}

func VersionString() string {
	if conf == nil {
		return ""
	}
	return conf.versionString
}

func SetVersionString(str string) {
	if conf == nil {
		return
	}
	conf.versionString = str
}

func SetFallbackConfig(content []byte) {
	fallbackConfigContent = content
}

func SetFallbackPname(pname string) {
	fallbackPname = pname
}

func SetFlag(flagType BootFlagType, longflag, shortflag, defaultValue string) {
	if flag, ok := conf.flags[flagType]; ok {
		flag.Long = longflag
		flag.Short = shortflag
		flag.Default = defaultValue
		conf.flags[flagType] = flag
	} else {
		panic(fmt.Errorf("invalid flag type: %d", flagType))
	}
}

func serve(conf *Config) {
	var err error
	if len(conf.ConfFile) > 0 {
		defaultBooter, err = defaultBuilder.BuildWithFiles([]string{conf.ConfFile})
	} else if len(conf.ConfDir) > 0 {
		defaultBooter, err = defaultBuilder.BuildWithDir(conf.ConfDir)
	} else if len(fallbackConfigContent) > 0 {
		defaultBooter, err = defaultBuilder.BuildWithContent(fallbackConfigContent)
	} else {
		panic(fmt.Errorf("one of --%s --%s should be provided",
			conf.flags[ConfigDirFlag].Long, conf.flags[ConfigFileFlag].Long))
	}
	if err != nil {
		panic(err)
	}

	bootlog.Println("startup", conf.Pname)
	err = defaultBooter.Startup()
	if err != nil {
		panic(err)
	}
}

func usage() {
	bin, _ := os.Executable()
	bin = filepath.Base(bin)
	fmt.Println(bin, "flags...")

	var maxlen = 0
	for _, v := range conf.flags {
		l := len(v.Long) + len(v.Short) + len(v.Placeholder)
		if maxlen < l {
			maxlen = l
		}
	}

	var uses = map[BootFlagType]string{}
	for k, v := range conf.flags {
		if runtime.GOOS == "windows" {
			if k == DaemonFlag || k == BootlogFlag || k == PidFlag {
				continue
			}
		}
		var format = "  %%s"
		if len(v.Default) > 0 {
			format = fmt.Sprintf("    %%-%ds  %s (default %s)", maxlen+5, v.Help, v.Default)
		} else {
			format = fmt.Sprintf("    %%-%ds  %s", maxlen+5, v.Help)
		}
		use := ""
		if len(v.Long) > 0 {
			use = fmt.Sprintf("--%s", v.Long)
		}
		if len(v.Short) > 0 {
			if len(use) > 0 {
				use = fmt.Sprintf("%s, -%s", use, v.Short)
			} else {
				use = fmt.Sprintf("-%s", v.Short)
			}
		}
		if len(v.Placeholder) > 0 {
			use = fmt.Sprintf("%s %s", use, v.Placeholder)
		}
		line := fmt.Sprintf(format, use)
		uses[k] = line
	}

	for i := 1; i < int(numofFlags); i++ {
		line := uses[BootFlagType(i)]
		if len(line) == 0 {
			continue
		}
		fmt.Println(line)
	}
}

func parseflags() {
	flag.Usage = usage

	// init with default values
	for k, v := range conf.flags {
		switch k {
		case ConfigDirFlag:
			conf.ConfDir = v.Default
		case ConfigFileFlag:
			conf.ConfFile = v.Default
		case PnameFlag:
			conf.Pname = v.Default
		case PidFlag:
			conf.PidFile = v.Default
		case BootlogFlag:
			conf.BootlogFile = v.Default
		case GenConfigFlag:
			conf.GenConfig = false
		case DaemonFlag:
			if len(v.Default) > 0 {
				if b, err := strconv.ParseBool(v.Default); err != nil {
					panic(err)
				} else {
					conf.Daemon = b
				}
			}
		}
	}

	// parse args
	parser := NewCommandLineParser(os.Args)
	parser.AddHintBool(conf.flags[DaemonFlag].Long, conf.flags[DaemonFlag].Short, false)
	parser.AddHintBool(conf.flags[HelpFlag].Long, conf.flags[HelpFlag].Short, false)

	cli, err := parser.Parse()
	if err != nil {
		fmt.Printf("\n Error: command line, %s\n\n", err.Error())
		flag.Usage()
		os.Exit(1)
	}

	var stringFlag = func(t BootFlagType, def string) string {
		f := cli.Flag(conf.flags[t].Long, conf.flags[t].Short)
		if f == nil {
			return def
		}
		return f.String(def)
	}
	var boolFlag = func(t BootFlagType, def bool) bool {
		f := cli.Flag(conf.flags[t].Long, conf.flags[t].Short)
		if f == nil {
			return def
		}
		return f.Bool(def)
	}
	conf.ConfDir = stringFlag(ConfigDirFlag, conf.ConfDir)
	conf.ConfFile = stringFlag(ConfigFileFlag, conf.ConfFile)
	conf.GenConfig = boolFlag(GenConfigFlag, conf.GenConfig)
	conf.Pname = stringFlag(PnameFlag, conf.Pname)
	conf.PidFile = stringFlag(PidFlag, conf.PidFile)
	conf.BootlogFile = stringFlag(BootlogFlag, conf.BootlogFile)
	conf.Daemon = boolFlag(DaemonFlag, conf.Daemon)
	doHelp := boolFlag(HelpFlag, false)

	if doHelp {
		flag.Usage()
		os.Exit(0)
	}

	if len(conf.ConfDir) == 0 && len(conf.ConfFile) == 0 && len(fallbackConfigContent) == 0 {
		fmt.Printf("\n  Error: at least one of --%s, --%s is required\n\n",
			conf.flags[ConfigDirFlag].Long, conf.flags[ConfigFileFlag].Long)
		flag.Usage()
		os.Exit(1)
	}
	if len(conf.Pname) == 0 {
		if len(fallbackPname) > 0 {
			conf.Pname = fallbackPname
		} else {
			conf.Pname = fmt.Sprintf("boot-%d", os.Getpid())
		}
	}
}
