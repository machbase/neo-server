package main

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
func main() {
	var fstabs engine.FSTabs
	var envVars engine.EnvVars = make(map[string]any)

	src := flag.String("C", "", "command to execute")
	scf := flag.String("S", "", "configured file to start from")
	flag.Var(&fstabs, "v", "volume to mount (format: /mountpoint=source)")
	flag.Var(&envVars, "e", "environment variable (format: name=value)")
	flag.Parse()

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
		conf.FSTabs = fstabs
		conf.Args = flag.Args()
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
	conf.ExecBuilder = func(code string, args []string, env map[string]any) (*exec.Cmd, error) {
		self, err := os.Executable()
		if err != nil {
			return nil, err
		}
		conf := engine.Config{
			Code:   code,
			Args:   args,
			FSTabs: fstabs,
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

	os.Exit(engine.Main())
}
