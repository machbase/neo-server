package session

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib"
	"github.com/machbase/neo-server/v8/jsh/root"
)

// JSH options:
//  1. -C "script" : command to execute
//     ex: jsh -C "console.println(require('/lib/process').argv[2])" helloworld
//  2. script file : execute script file
//     ex: jsh script.js arg1 arg2
//  3. no args : start interactive shell
//     ex: jsh
func JshMain(flags *flag.FlagSet, executable []string, args []string) int {
	engine, exitCode, err := entry(flags, executable, args, &ExtConfig{
		callback: func(conf *engine.Config, extFlags ExtFlags) error {
			conf.Default = "/sbin/shell.js"
			conf.ProcRecord = true
			if len(executable) > 0 {
				conf.ProcCommand = executable[0]
			}
			conf.ProcArgs = append([]string{"jsh"}, args...)
			return nil
		},
	})
	if err != nil {
		fmt.Println(err.Error())
		return exitCode
	}
	if exitCode != 0 {
		return exitCode
	}
	lib.Enable(engine)
	return engine.Main()
}

func entry(flags *flag.FlagSet, executable []string, args []string, ext *ExtConfig) (*engine.JSRuntime, int, error) {
	var fsTabs engine.FSTabs
	var envVars engine.EnvVars = make(map[string]any)

	src := flags.String("C", "", "command to execute")
	scf := flags.String("S", "", "configured file to start from")
	flags.Var(&fsTabs, "v", "volume to mount (format: /mountpoint=source)")
	flags.Var(&envVars, "e", "environment variable (format: name=value)")
	// if extFlags have envKey, add flag for it
	for i := range ext.flags {
		flags.StringVar(&ext.flags[i].value, ext.flags[i].flag, "", ext.flags[i].comment)
	}

	if err := flags.Parse(args); err != nil {
		return nil, 1, fmt.Errorf("Error parsing flags: %s", err.Error())
	}

	conf := engine.Config{
		Env:     map[string]any{},
		Aliases: map[string]string{},
	}

	if *scf != "" {
		// when it starts with "-S", read secret box
		if err := engine.ReadSecretBox(*scf, &conf); err != nil {
			return nil, 1, fmt.Errorf("Error reading secret file: %s", err.Error())
		}
	} else {
		// otherwise, use command args to build ExecPass
		if strings.HasPrefix(*src, "@") {
			// when it starts with "@", read script file
			codeBytes, err := os.ReadFile((*src)[1:])
			if err != nil {
				fmt.Println("Error reading script file:", err.Error())
				os.Exit(1)
			}
			conf.Code = string(codeBytes)
		} else {
			conf.Code = *src
		}
		conf.FSTabs = fsTabs
		if ext.argsNormalizer != nil {
			conf.Args = ext.argsNormalizer(flags.Args())
		} else {
			conf.Args = flags.Args()
		}
		conf.Env["PATH"] = "/sbin:/work"
		conf.Env["HOME"] = "/work"
		conf.Env["PWD"] = "/work"
		conf.Env["LIBRARY_PATH"] = "./node_modules:/lib"
		conf.Aliases["ll"] = "ls -l"
	}

	// if `-e name=value` is set, split it and set to conf.Env
	for k, v := range envVars {
		conf.Env[k] = v
	}
	// if extFlags have envKey, get value from conf.Env and set to flagValue
	for _, ef := range ext.flags {
		if ef.envKey != "" {
			if val, ok := conf.Env[ef.envKey]; ok {
				if sec, ok := val.(engine.SecureString); ok {
					ef.value = sec.Value()
				} else {
					ef.value = val.(string)
				}
			}
		}
	}
	if !conf.FSTabs.HasMountPoint("/") {
		conf.FSTabs = append([]engine.FSTab{root.RootFSTab()}, conf.FSTabs...)
	}
	if !conf.FSTabs.HasMountPoint("/lib") {
		conf.FSTabs = append(conf.FSTabs, lib.LibFSTab())
	}
	if !conf.FSTabs.HasMountPoint("/work") {
		fsDir, _ := engine.DirFS(".")
		conf.FSTabs = append(conf.FSTabs, engine.FSTab{MountPoint: "/work", FS: fsDir})
	}
	conf.ExecBuilder = func(code string, args []string, env map[string]any) (*exec.Cmd, error) {
		conf := engine.Config{
			Code:   code,
			Args:   args,
			FSTabs: conf.FSTabs,
			Env:    env,
		}
		secretBox, err := engine.NewSecretBox(conf)
		if err != nil {
			return nil, err
		}
		execArgs := []string{"-S", secretBox.FilePath(), args[0]}
		if len(executable) > 1 {
			execArgs = append(executable[1:], execArgs...)
		}
		execCmd := exec.Command(executable[0], execArgs...)
		return execCmd, nil
	}
	// if callback is set, call it with conf and extFlags
	if ext != nil && ext.callback != nil {
		if err := ext.callback(&conf, ext.flags); err != nil {
			return nil, 1, fmt.Errorf("Error in callback: %s", err.Error())
		}
	}

	engine, err := engine.New(conf)
	if err != nil {
		return nil, 1, fmt.Errorf("Error creating engine: %s", err.Error())
	}
	return engine, 0, nil
}

type ExtConfig struct {
	flags          ExtFlags
	callback       ExtFlagCallback
	argsNormalizer func([]string) []string
}

type ExtFlag struct {
	flag    string
	value   string
	comment string
	envKey  string
}

type ExtFlags []*ExtFlag

type ExtFlagCallback func(*engine.Config, ExtFlags) error

func (efs ExtFlags) Get(flag string) *ExtFlag {
	for _, ef := range efs {
		if ef.flag == flag {
			return ef
		}
	}
	return nil
}
