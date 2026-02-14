package session

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/native"
	"github.com/machbase/neo-server/v8/jsh/root"
)

// JSH options:
//  1. -C "script" : command to execute
//     ex: jsh -C "console.println(require('/lib/process').argv[2])" helloworld
//  2. script file : execute script file
//     ex: jsh script.js arg1 arg2
//  3. no args : start interactive shell
//     ex: jsh
func Main(flags *flag.FlagSet, executable []string, args []string) int {
	var fsTabs engine.FSTabs
	var envVars engine.EnvVars = make(map[string]any)

	src := flags.String("C", "", "command to execute")
	scf := flags.String("S", "", "configured file to start from")
	flags.Var(&fsTabs, "v", "volume to mount (format: /mountpoint=source)")
	flags.Var(&envVars, "e", "environment variable (format: name=value)")
	if err := flags.Parse(args); err != nil {
		fmt.Println("Error parsing flags:", err.Error())
		return 1
	}

	conf := engine.Config{}
	if *scf != "" {
		// when it starts with "-s", read secret box
		if err := engine.ReadSecretBox(*scf, &conf); err != nil {
			fmt.Println("Error reading secret file:", err.Error())
			os.Exit(1)
		}
	} else {
		// otherwise, use command args to build ExecPass
		conf.Code = *src
		conf.FSTabs = fsTabs
		conf.Args = flags.Args()
		conf.Default = "/sbin/shell.js" // default script to run if no args
		conf.Env = map[string]any{
			"PATH":         "/sbin:/work",
			"HOME":         "/work",
			"PWD":          "/work",
			"LIBRARY_PATH": "./node_modules:/lib",
		}
		conf.Aliases = map[string]string{
			"ll": "ls -l",
		}
	}
	for k, v := range envVars {
		conf.Env[k] = v
	}
	if !conf.FSTabs.HasMountPoint("/") {
		conf.FSTabs = append([]engine.FSTab{root.RootFSTab()}, conf.FSTabs...)
	}
	if !conf.FSTabs.HasMountPoint("/work") {
		fsDir, _ := engine.DirFS(".")
		conf.FSTabs = append(conf.FSTabs, engine.FSTab{MountPoint: "/work", FS: fsDir})
	}
	conf.ExecBuilder = func(code string, args []string, env map[string]any) (*exec.Cmd, error) {
		self, err := os.Executable()
		if err != nil {
			return nil, err
		}
		conf := engine.Config{
			Code:   code,
			Args:   args,
			FSTabs: fsTabs,
			Env:    env,
		}
		secretBox, err := engine.NewSecretBox(conf)
		if err != nil {
			return nil, err
		}
		execCmd := exec.Command(self, "-S", secretBox.FilePath(), args[0])
		return execCmd, nil
	}

	engine, err := engine.New(conf)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	native.Enable(engine)

	return engine.Main()
}
