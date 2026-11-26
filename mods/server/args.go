// Server startup parses boot options, loads HCL configuration, and starts the
// configured logging and server modules. Arguments are scanned once for both
// boot options and HCL flag expressions; their precedence rules differ.
// Unix daemon readiness and Windows service lifecycle are platform-specific.
package server

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode"

	"github.com/machbase/neo-server/v8/mods"
	"github.com/machbase/neo-server/v8/mods/util"
)

func Main(args []string) int {
	cli, err := ParseCommand(args)
	if err != nil {
		if cli != nil {
			doHelp(cli.Command, args)
		} else {
			doHelp("", args)
		}
		fmt.Println("ERR", err.Error())
		return 1
	}
	switch cli.Command {
	case "gen-config":
		fmt.Println(string(DefaultFallbackConfig))
	case "version":
		fmt.Println(mods.GenBanner())
	case "help":
		doHelp(cli.Help.Command, args)
	case "serve":
		return doServeWithSignal(args, cli.Serve.options, false, nil)
	case "serve-headless":
		return doServeWithSignal(args, cli.Serve.options, true, nil)
	case "restore":
		return doRestore(&cli.Restore)
	case "service":
		doService(&cli.Service, args)
	case "help <command> <sub-command>":
	}
	return 0
}

func doServeWithSignal(args []string, options serverBootOptions, headless bool, requested chan struct{}) int {
	if err := serveWithSignal(args, options, headless, requested, nil); err != nil {
		fmt.Fprintln(os.Stderr, "ERR", err)
		return 1
	}
	return 0
}

func serveWithSignal(args []string, options serverBootOptions, headless bool, requested chan struct{}, ready func() error) error {
	if options.help {
		command := "serve"
		if headless {
			command = "serve-headless"
		}
		doHelp(command, args)
		return nil
	}
	if options.generateConfig {
		fmt.Println(string(DefaultFallbackConfig))
		return nil
	}

	if options.daemon {
		return daemonizeWithError(options.bootlogFile, options.pidFile, func(daemonReady func() error, pidFile *os.File) error {
			return runServer(args, options.preset, headless, options, requested, func() error {
				if err := daemonReady(); err != nil {
					return err
				}
				if ready != nil {
					return ready()
				}
				return nil
			}, pidFile)
		})
	}
	return runServer(args, options.preset, headless, options, requested, ready, nil)
}

func runServer(args []string, preset string, headless bool, options serverBootOptions, requested chan struct{}, ready func() error, pidFile *os.File) (err error) {
	logger, logFile, err := serverBootLogger(options.pname, options.bootlogFile, options.daemon)
	if err != nil {
		return err
	}
	if logFile != nil {
		defer func() { err = errors.Join(err, logFile.Close()) }()
	}
	execution := newServerExecution(args, preset, headless, options, logger, requested)
	removePID, err := createPIDFile(options.pidFile, pidFile)
	if err != nil {
		execution.shutdown()
		return err
	}
	defer func() { err = errors.Join(err, removePID()) }()

	defer execution.shutdown()
	if err := execution.loadConfig(); err != nil {
		return err
	}
	logger.Println("pid:", os.Getpid())
	if err := execution.start(); err != nil {
		return err
	}
	if ready != nil {
		if err := ready(); err != nil {
			return fmt.Errorf("report daemon readiness: %w", err)
		}
	}
	execution.waitSignal()
	return nil
}

func removeOwnedPID(path string, info os.FileInfo) error {
	current, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat pid file %q: %w", path, err)
	}
	if !os.SameFile(info, current) {
		return fmt.Errorf("pid file %q was replaced", path)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove pid file %q: %w", path, err)
	}
	return nil
}

func createPIDFile(path string, file *os.File) (func() error, error) {
	if path == "" {
		return func() error { return nil }, nil
	}
	if file == nil {
		var err error
		file, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return nil, fmt.Errorf("create pid file %q: %w", path, err)
		}
	}
	info, err := file.Stat()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("stat pid file %q: %w", path, err), file.Close())
	}
	remove := func() error {
		return removeOwnedPID(path, info)
	}
	_, writeErr := fmt.Fprintf(file, "%d", os.Getpid())
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return nil, errors.Join(fmt.Errorf("write pid file %q: %w", path, err), remove())
	}
	current, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("stat pid file %q: %w", path, err)
	}
	if !os.SameFile(info, current) {
		return nil, fmt.Errorf("pid file %q was replaced", path)
	}
	return remove, nil
}

type serverBootOptions struct {
	configDir      string
	configFile     string
	pname          string
	pidFile        string
	bootlogFile    string
	daemon         bool
	generateConfig bool
	help           bool
	preset         string
	arguments      []serverArgument
}

type serverArgument struct {
	name     string
	value    string
	hasValue bool
}

func scanServerArguments(args []string) []serverArgument {
	var arguments []serverArgument
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if len(arg) < 2 || arg[0] != '-' {
			continue
		}
		name, value, hasValue := strings.Cut(arg, "=")
		isBool := name == "--daemon" || name == "--gen-config" || name == "--help" || name == "-d" || name == "-h"
		if !hasValue && isBool {
			value, hasValue = "true", true
		} else if !hasValue && i+1 < len(args) && isServerOptionValue(args[i+1]) {
			i++
			value, hasValue = args[i], true
		}
		arguments = append(arguments, serverArgument{name: name, value: value, hasValue: hasValue})
	}
	return arguments
}

func isServerOptionValue(value string) bool {
	if value == "-" || !strings.HasPrefix(value, "-") {
		return true
	}
	return len(value) > 1 && (value[1] >= '0' && value[1] <= '9' || value[1] == '.')
}

func parseServerBootOptions(args []string) (serverBootOptions, error) {
	options := serverBootOptions{pname: DefaultFallbackPname, arguments: scanServerArguments(args)}
	flags := make(map[string]serverArgument)
	valueOf := func(flag serverArgument, allowEmpty bool) (string, error) {
		if !flag.hasValue {
			return "", fmt.Errorf("command line: %s requires a value", flag.name)
		}
		value := flag.value
		if value != "" {
			first := []rune(value)[0]
			if unicode.In(first, unicode.Quotation_Mark) {
				value = strings.Trim(value, string(first))
			}
		}
		if value == "" && !allowEmpty {
			return "", fmt.Errorf("command line: %s requires a value", flag.name)
		}
		return value, nil
	}
	for _, arg := range options.arguments {
		name := strings.TrimPrefix(arg.name, "-")
		if after, ok := strings.CutPrefix(name, "-"); ok {
			name = after
		}
		if name == "" || name[0] == '-' || name[0] == '=' {
			return options, fmt.Errorf("command line: bad flag syntax: %s", arg.name)
		}
		switch arg.name {
		case "--config", "-c", "--config-dir", "--pid", "--bootlog", "--preset", "--file", "--pname":
			if _, err := valueOf(arg, arg.name == "--pname"); err != nil {
				return options, err
			}
		case "--daemon", "-d", "--gen-config", "--help", "-h":
			if _, err := strconv.ParseBool(arg.value); err != nil {
				return options, fmt.Errorf("command line: invalid value %q for %s: %w", arg.value, arg.name, err)
			}
		}
		flags[arg.name] = arg
	}
	find := func(long, short string) (serverArgument, bool) {
		if flag, ok := flags["--"+long]; ok {
			return flag, true
		}
		flag, ok := flags["-"+short]
		return flag, ok && short != ""
	}
	setString := func(target *string, long, short string, allowEmpty bool) error {
		if flag, ok := find(long, short); ok {
			value, err := valueOf(flag, allowEmpty)
			if err != nil {
				return err
			}
			*target = value
		}
		return nil
	}
	setBool := func(target *bool, long, short string) error {
		if flag, ok := find(long, short); ok && flag.hasValue {
			value, err := strconv.ParseBool(flag.value)
			if err != nil {
				return fmt.Errorf("command line: invalid value %q for --%s: %w", flag.value, long, err)
			}
			*target = value
		}
		return nil
	}
	for _, flag := range []struct {
		target      *string
		long, short string
		allowEmpty  bool
	}{
		{&options.configDir, "config-dir", "", false},
		{&options.configFile, "config", "c", false},
		{&options.pname, "pname", "", true},
		{&options.pidFile, "pid", "", false},
		{&options.bootlogFile, "bootlog", "", false},
		{&options.preset, "preset", "", false},
	} {
		if err := setString(flag.target, flag.long, flag.short, flag.allowEmpty); err != nil {
			return options, err
		}
	}
	for _, flag := range []struct {
		target      *bool
		long, short string
	}{
		{&options.daemon, "daemon", "d"},
		{&options.generateConfig, "gen-config", ""},
		{&options.help, "help", "h"},
	} {
		if err := setBool(flag.target, flag.long, flag.short); err != nil {
			return options, err
		}
	}
	if options.pname == "" {
		options.pname = DefaultFallbackPname
		if options.pname == "" {
			options.pname = fmt.Sprintf("boot-%d", os.Getpid())
		}
	}
	return options, nil
}

func serverBootLogger(pname, bootlogPath string, daemon bool) (*log.Logger, *os.File, error) {
	var writer io.Writer = os.Stdout
	var file *os.File
	if bootlogPath != "" && !daemon {
		var err error
		file, err = os.OpenFile(bootlogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, nil, fmt.Errorf("open boot log %q: %w", bootlogPath, err)
		}
		writer = io.MultiWriter(os.Stdout, file)
	}
	return log.New(writer, fmt.Sprintf("boot-%s ", pname), log.LstdFlags|log.Lmsgprefix), file, nil
}

func doRestore(r *RestoreCmd) int {
	if err := Restore(r.DataDir, r.BackupDir); err != nil {
		fmt.Println("ERR", err.Error())
		return -1
	}
	return 0
}

type NeoCommand struct {
	Command string
	Serve   struct {
		Preset  string
		options serverBootOptions
	}
	GenConfig struct{}
	Version   struct{}
	Help      struct {
		Command    string
		SubCommand string
	}
	Restore RestoreCmd
	Service Service

	args []string
}

func ParseCommand(args []string) (*NeoCommand, error) {
	if len(args) <= 1 {
		return nil, errors.New("missing required command")
	}

	cli := &NeoCommand{}
	if args[1] == "serve" || args[1] == "serve-headless" {
		cli.Command = args[1]
		cli.args = args[2:]
		options, err := parseServerBootOptions(args)
		if err != nil {
			return cli, err
		}
		cli.Serve.Preset = options.preset
		cli.Serve.options = options
		return cli, nil
	}

	hasHelpFlag := false
	idxHelpFlag := -1
	if args[0] != "help" {
		for i, s := range args[1:] {
			if s == "--help" || s == "-h" {
				hasHelpFlag = true
				idxHelpFlag = i + 1
				break
			}
		}
	}
	if hasHelpFlag {
		cli.Command = "help"
		if idxHelpFlag == 2 {
			cli.Help.Command = args[1]
		} else if idxHelpFlag >= 3 {
			cli.Help.Command = args[1]
			cli.Help.SubCommand = args[2]
		}
		return cli, nil
	} else {
		cli.Command = args[1]
		cli.args = args[2:]
	}

	switch cli.Command {
	case "restore":
		return parseRestore(cli)
	case "help":
		return parseHelp(cli)
	case "gen-config":
		return parseGenConfig(cli)
	case "version":
		return parseVersion(cli)
	case "service":
		if runtime.GOOS == "windows" {
			return parseService(cli)
		} else {
			return nil, fmt.Errorf("command 'service' is only available on Windows")
		}
	default:
		return nil, fmt.Errorf("unknown command '%s'", cli.Command)
	}
}

func parseVersion(cli *NeoCommand) (*NeoCommand, error) {
	return cli, nil
}

func parseGenConfig(cli *NeoCommand) (*NeoCommand, error) {
	return cli, nil
}

func parseHelp(cli *NeoCommand) (*NeoCommand, error) {
	if len(cli.args) >= 1 {
		cli.Help.Command = cli.args[0]
	}
	if len(cli.args) >= 2 {
		cli.Help.SubCommand = cli.args[1]
	}
	return cli, nil
}

type RestoreCmd struct {
	DataDir   string
	BackupDir string
}

func parseRestore(cli *NeoCommand) (*NeoCommand, error) {
	backupArgs := []string{}
	for i := 0; i < len(cli.args); i++ {
		s := cli.args[i]
		if strings.HasPrefix(s, "--data=") {
			dataDir := strings.TrimPrefix(s, "--data=")
			if dataDir == "" {
				return nil, errors.New("flag '--data' requires a value")
			}
			cli.Restore.DataDir = dataDir
			continue
		}
		if s == "--data" {
			if i+1 >= len(cli.args) || strings.HasPrefix(cli.args[i+1], "-") {
				return nil, errors.New("flag '--data' requires a value")
			}
			cli.Restore.DataDir = cli.args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(s, "-") {
			return nil, fmt.Errorf("unknown flag '%s'", s)
		}
		backupArgs = append(backupArgs, s)
	}

	if len(backupArgs) == 0 {
		return nil, errors.New("missing required argument '<backup_dir>'")
	}
	if len(backupArgs) > 1 {
		return nil, fmt.Errorf("too many restore arguments: %v", backupArgs[1:])
	}
	cli.Restore.BackupDir = backupArgs[0]

	if cli.Restore.DataDir == "" {
		if ep, err := os.Executable(); err != nil {
			return nil, err
		} else {
			cli.Restore.DataDir = filepath.Join(filepath.Dir(ep), "machbase_home")
		}
	}
	return cli, nil
}

type Service struct {
	Args []string `arg:"" optional:"" name:"ARGS" passthrough:""`
}

func parseService(cli *NeoCommand) (*NeoCommand, error) {
	cli.Service.Args = cli.args
	return cli, nil
}

func doHelp(command string, args []string) error {
	showShellHelp := true
	showServeHelp := true
	bin := "machbase-neo"
	if len(args) > 0 && args[0] != "" {
		bin = args[0]
	} else if executable, err := os.Executable(); err == nil {
		bin = executable
	}

	switch command {
	case "serve":
		fmt.Println(bin + " serve [args...]")
		showShellHelp = false
	case "serve-headless":
		fmt.Println(bin + " serve-headless [args...]")
		showShellHelp = false
	case "shell":
		fmt.Println(bin + " shell [flags] <sub-command> [args...]")
		showServeHelp = false
	case "restore":
		fmt.Println(bin + " restore --data <machbase_home_dir> <backup_dir>")
		showShellHelp = false
		showServeHelp = false
	case "timeformat":
		fmt.Println("  timeformat:")
		fmt.Printf("%s\n", util.HelpTimeformats())
		return nil
	case "tz":
		fmt.Println("  timezones:")
		fmt.Printf("%s\n", util.HelpTimeZones())
		return nil
	default:
		fmt.Println(filepath.Base(bin) + helpRootText)
	}

	if showServeHelp {
		if runtime.GOOS == "windows" {
			fmt.Println(helpServeTextWindows)
		} else {
			fmt.Println(helpServeText)
		}
	}

	if showShellHelp {
		fmt.Println(helpShellText)
	}
	return nil
}

const helpRootText = ` <command> [args...]

Commands:
  serve <flags>               start machbase-neo server process
  serve-headless <flags>      start machbase-neo server process in headless mode
  shell <flags> <sub-command> run neoshell client
  gen-config                  show config template
  restore                     restore database from backup
  version                     show version`

const helpServeText = `
serve flags:
      --config-dir=<dir>  config directory path
  -c, --config=<file>     config file path
      --pname=<pname>     assign process name
      --pid=<path>        pid file path
      --bootlog=<path>    boot log file path
  -d, --daemon            run process in background, daemonize`

const helpServeTextWindows = `
serve flags:
      --config-dir=<dir>  config directory path
  -c, --config=<file>     config file path
      --pname=<pname>     assign process name`

const helpShellText = `Usage of shell:
  -C string
        command to execute
  -S string
        configured file to start from
  -e value
        environment variable (format: name=value)
  -password string
        password (default: manager)
  -server string
        machbase-neo host
  -user string
        user name (default: sys)
  -v value
        volume to mount (format: /mountpoint=source)`
