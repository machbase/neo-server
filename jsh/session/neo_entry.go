package session

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib"
	"github.com/nyaosorg/go-readline-ny"
	"golang.org/x/term"
)

func NeoShellMain(flags *flag.FlagSet, executable []string, args []string) int {
	extFlags := []*ExtFlag{
		{flag: "server", comment: "machbase-neo host", envKey: "NEOSHELL_HOST"},
		{flag: "user", comment: "user name (default: sys)", envKey: "NEOSHELL_USER"},
		{flag: "password", comment: "password (default: manager)", envKey: "NEOSHELL_PASSWORD"},
	}
	extConfig := &ExtConfig{
		flags:          extFlags,
		callback:       neoShellConfigure(executable, flags.Args()),
		argsNormalizer: normalizeShellArgs,
	}
	engine, exitCode, err := entry(flags, executable, args, extConfig)
	if err != nil {
		fmt.Println(err.Error())
		return exitCode
	}
	if exitCode != 0 {
		return exitCode
	}
	lib.Enable(engine)
	engine.RegisterNativeModule("@jsh/session", Module)
	return engine.Main()
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

func neoShellConfigure(executables []string, args []string) func(conf *engine.Config, extFlags ExtFlags) error {
	return func(conf *engine.Config, extFlags ExtFlags) error {
		var ef *ExtFlag
		var err error

		if ef = extFlags.Get("server"); ef == nil {
			return fmt.Errorf("server flag is not configured")
		}
		if ef.value == "" {
			ef.value = os.Getenv("NEOSHELL_HOST")
		}
		if ef.value == "" {
			ef.value, err = readLine("Server", "127.0.0.1:5654")
			if err != nil {
				return fmt.Errorf("Error reading Server: %w", err)
			}
		}
		if !strings.HasPrefix(ef.value, "unix://") {
			ef.value = strings.TrimPrefix(ef.value, "http://")
			ef.value = strings.TrimPrefix(ef.value, "https://")
			ef.value = strings.TrimPrefix(ef.value, "tcp://")
			if _, port, err := net.SplitHostPort(ef.value); err != nil {
				port, err = readLine("Port", "5654")
				if err != nil {
					return fmt.Errorf("Error reading Port: %w", err)
				}
				ef.value = net.JoinHostPort(ef.value, port)
			}
		}
		if ef.value == "" {
			return errors.New("Server is required")
		}

		if ef = extFlags.Get("user"); ef == nil {
			return fmt.Errorf("user flag is not configured")
		}
		if ef.value == "" {
			ef.value = os.Getenv("NEOSHELL_USER")
		}
		if ef.value == "" {
			ef.value, err = readLine("User", "SYS")
			if err != nil {
				return fmt.Errorf("Error reading User: %w", err)
			}
		}
		if ef.value == "" {
			return errors.New("User is required")
		}

		if ef = extFlags.Get("password"); ef == nil {
			return fmt.Errorf("password flag is not configured")
		}
		if ef.value == "" {
			ef.value = os.Getenv("NEOSHELL_PASSWORD")
		}
		if ef.value == "" {
			ef.value, err = readPassword("Password", "manager")
			if err != nil {
				return fmt.Errorf("Error reading Password: %w", err)
			}
		}
		if ef.value == "" {
			return errors.New("Password is required")
		}

		conf.Default = "/usr/bin/neo-shell.js"
		conf.ProcRecord = true
		if len(executables) > 0 {
			conf.ProcCommand = executables[0]
		}
		conf.ProcArgs = append([]string{"shell"}, args...)

		conf.Env["PATH"] = "/usr/bin:/usr/lib:/sbin:/lib:/work"
		conf.Env["NEOSHELL_HOST"] = extFlags.Get("server").value
		conf.Env["NEOSHELL_USER"] = extFlags.Get("user").value
		conf.Env["NEOSHELL_PASSWORD"] = engine.SecureString(extFlags.Get("password").value)
		conf.Aliases["jsh"] = "/sbin/shell.js"
		conf.Aliases["describe"] = "show table"
		conf.Aliases["desc"] = "show table"

		// configure default session before engine creation so SERVICE_CONTROLLER
		// is available during filesystem mount setup.
		if err := Configure(Config{
			Server:   extFlags.Get("server").value,
			User:     extFlags.Get("user").value,
			Password: extFlags.Get("password").value,
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
		return nil
	}
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
