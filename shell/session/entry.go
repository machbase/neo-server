//go:generate go run sql_verbs_generate.go

package session

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib"
	"github.com/machbase/neo-server/v8/jsh/root"
	"github.com/nyaosorg/go-readline-ny"
	"golang.org/x/term"
)

// JSH options:
//  1. -C "script" : command to execute
//     ex: neo-shell -C "console.println(require('process').argv[2])" helloworld
//  2. script file : execute script file
//     ex: neo-shell script.js arg1 arg2
//  3. no args : start interactive shell
//     ex: neo-shell
func Main(flags *flag.FlagSet, executable []string, args []string) {
	var fsTabs engine.FSTabs
	var envVars engine.EnvVars = make(map[string]any)
	var neoHost string
	var neoUser string
	var neoPassword string
	var err error

	src := flags.String("C", "", "command to execute")
	scf := flags.String("S", "", "configured file to start from")
	flags.Var(&fsTabs, "v", "volume to mount (format: /mountpoint=source)")
	flags.Var(&envVars, "e", "environment variable (format: name=value)")
	flags.StringVar(&neoHost, "server", "", "machbase-neo host")
	flags.StringVar(&neoUser, "user", "", "user name (default: sys)")
	flags.StringVar(&neoPassword, "password", "", "password (default: manager)")
	if err := flags.Parse(args); err != nil {
		fmt.Println("Error parsing flags:", err.Error())
		os.Exit(1)
	}

	conf := engine.Config{}
	if *scf != "" {
		// when it starts with "-S", read secret box
		if err := engine.ReadSecretBox(*scf, &conf); err != nil {
			fmt.Println("Error reading secret file:", err.Error())
			os.Exit(1)
		}
		if host, ok := conf.Env["NEOSHELL_HOST"]; ok {
			neoHost = host.(string)
		}
		if user, ok := conf.Env["NEOSHELL_USER"]; ok {
			neoUser = user.(string)
		}
		if pass, ok := conf.Env["NEOSHELL_PASSWORD"]; ok {
			if sec, ok := pass.(engine.SecureString); ok {
				neoPassword = sec.Value()
			} else {
				neoPassword = pass.(string)
			}
		}
		if neoUser == "" {
			neoUser, err = readLine("User", "SYS")
			if err != nil {
				fmt.Println("Error reading User:", err.Error())
				os.Exit(1)
			}
			conf.Env["NEOSHELL_USER"] = neoUser
		}
		if neoPassword == "" {
			neoPassword, err = readPassword("Password", "manager")
			if err != nil {
				fmt.Println("Error reading Password:", err.Error())
				os.Exit(1)
			}
			conf.Env["NEOSHELL_PASSWORD"] = engine.SecureString(neoPassword)
		}
	} else {
		if neoHost == "" {
			neoHost = os.Getenv("NEOSHELL_HOST")
		}
		if neoHost == "" {
			neoHost, err = readLine("Server", "127.0.0.1:5654")
			if err != nil {
				fmt.Println("Error reading Server:", err.Error())
				os.Exit(1)
			}
		}
		if !strings.HasPrefix(neoHost, "unix://") {
			neoHost = strings.TrimPrefix(neoHost, "http://")
			neoHost = strings.TrimPrefix(neoHost, "https://")
			neoHost = strings.TrimPrefix(neoHost, "tcp://")
			if _, port, err := net.SplitHostPort(neoHost); err != nil {
				port, err = readLine("Port", "5654")
				if err != nil {
					fmt.Println("Error reading Port:", err.Error())
					os.Exit(1)
				}
				neoHost = net.JoinHostPort(neoHost, port)
			}
		}
		if neoUser == "" {
			neoUser = os.Getenv("NEOSHELL_USER")
		}
		if neoUser == "" {
			neoUser, err = readLine("User", "SYS")
			if err != nil {
				fmt.Println("Error reading User:", err.Error())
				os.Exit(1)
			}
		}
		if neoPassword == "" {
			neoPassword = os.Getenv("NEOSHELL_PASSWORD")
		}
		if neoPassword == "" {
			neoPassword, err = readPassword("Password", "manager")
			if err != nil {
				fmt.Println("Error reading Password:", err.Error())
				os.Exit(1)
			}
		}
		// otherwise, use command args to build ExecPass
		if strings.HasPrefix(*src, "@") {
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
		conf.Args = normalizeShellArgs(flags.Args())
		conf.Default = "/usr/bin/neo-shell.js" // default script to run if no args
		conf.Env = map[string]any{
			"PATH":              "/usr/bin:/usr/lib:/sbin:/lib:/work",
			"HOME":              "/work",
			"PWD":               "/work",
			"NEOSHELL_HOST":     neoHost,
			"NEOSHELL_USER":     neoUser,
			"NEOSHELL_PASSWORD": engine.SecureString(neoPassword),
		}
		conf.Aliases = map[string]string{
			"jsh":      "/sbin/shell.js",
			"describe": "show table",
			"desc":     "show table",
		}
	}
	for k, v := range envVars {
		conf.Env[k] = v
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
	// setup ExecBuilder to enable re-execution
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
	conf.ProcRecord = true
	if len(executable) > 0 {
		conf.ProcCommand = executable[0]
	}
	conf.ProcArgs = append([]string{"shell"}, args...)

	// configure default session before engine creation so SERVICE_CONTROLLER
	// is available during filesystem mount setup.
	if err := Configure(Config{
		Server:   neoHost,
		User:     neoUser,
		Password: neoPassword,
		env:      map[string]any{},
	}); err != nil {
		if err == ErrUserOrPasswordIncorrect {
			fmt.Println("Login failed: user or password is incorrect")
		} else {
			fmt.Println("Error configuring session:", err.Error())
		}
		os.Exit(1)
	}
	for k, v := range defaultSession.env {
		if _, ok := conf.Env[k]; !ok {
			conf.Env[k] = v
		}
	}

	eng, err := engine.New(conf)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	lib.Enable(eng)
	eng.RegisterNativeModule("@jsh/session", Module)
	os.Exit(eng.Main())
}

func readPassword(prompt string, defaultValue string) (string, error) {
	if defaultValue != "" {
		prompt = fmt.Sprintf("%s [%s]", prompt, defaultValue)
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		// terminal (stdin) is not available (e.g. connected via pipe)
		return "", errors.New("terminal stdin is not available")
	}
	fmt.Fprintf(os.Stdout, "%s: ", prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	fmt.Println()
	if len(b) == 0 && defaultValue != "" {
		return defaultValue, err
	}
	return string(b), err
}

func readLine(prompt string, defaultValue string) (string, error) {
	var ctx = context.Background()
	var editor = &readline.Editor{
		PromptWriter: func(w io.Writer) (int, error) {
			if defaultValue != "" {
				return io.WriteString(w, fmt.Sprintf("%s [%s]: ", prompt, defaultValue))
			} else {
				return io.WriteString(w, fmt.Sprintf("%s: ", prompt))
			}
		},
	}
	text, err := editor.ReadLine(ctx)
	if err == nil && text == "" {
		text = defaultValue
	}
	return text, err
}

func normalizeShellArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}
	firstToken := args[0]
	if fields := strings.Fields(firstToken); len(fields) > 0 {
		firstToken = fields[0]
	}
	if _, ok := implicitSQLVerbs[strings.ToUpper(firstToken)]; !ok {
		return args
	}
	return append([]string{"sql"}, args...)
}
