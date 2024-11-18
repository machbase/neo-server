package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/machbase/neo-server/v8/mods"
	"github.com/machbase/neo-server/v8/mods/shell/internal/action"
	_ "github.com/machbase/neo-server/v8/mods/shell/internal/cmd"
	"github.com/machbase/neo-server/v8/mods/util"
)

func Main() int {
	var cli ShellCmd
	_ = kong.Parse(&cli,
		kong.HelpOptions{NoAppSummary: false, Compact: true, FlagsLast: true},
		kong.UsageOnError(),
		kong.Help(HelpKong),
	)
	Shell(&cli)
	return 0
}

type ShellCmd struct {
	Args           []string `arg:"" optional:"" name:"ARGS" passthrough:""`
	Version        bool     `name:"version" default:"false" help:"show version"`
	ServerAddr     string   `name:"server" short:"s" help:"server address"`
	Insecure       bool     `name:"insecure" help:"use insecure plain socket"`
	ServerCertPath string   `name:"server-cert" help:"path to server certificate"`
	ClientCertPath string   `name:"client-cert" help:"path to client certificate"`
	ClientKeyPath  string   `name:"client-key" help:"path to client key"`
	User           string   `name:"user" default:"sys" help:"user name"`
	Password       string   `name:"password" default:"manager" help:"password"`
}

var DefaultServerAddress = "tcp://127.0.0.1:5655"

func Shell(cmd *ShellCmd) {
	if cmd.Version {
		fmt.Fprintf(os.Stdout, "neoshell %s\n", mods.VersionString())
		return
	}

	for _, f := range cmd.Args {
		if f == "--help" || f == "-h" {
			targetCmd := action.FindCmd(strings.ToLower(cmd.Args[0]))
			if targetCmd == nil {
				fmt.Fprintf(os.Stdout, "unknown sub-command %s\n\n", cmd.Args[0])
				return
			}
			fmt.Fprintf(os.Stdout, "%s\n", targetCmd.Usage)
			return
		}
	}

	pref, _ := action.LoadPref()

	if cmd.ServerAddr == "" {
		if pref != nil {
			cmd.ServerAddr = pref.Server().Value()
		} else {
			cmd.ServerAddr = "tcp://127.0.0.1:5655"
		}
	}
	if !cmd.Insecure && cmd.ServerCertPath == "" && pref != nil {
		cmd.ServerCertPath = pref.ServerCert().Value()
	}
	if !cmd.Insecure && cmd.ClientCertPath == "" && pref != nil {
		cmd.ClientCertPath = pref.ClientCert().Value()
	}
	if !cmd.Insecure && cmd.ClientKeyPath == "" && pref != nil {
		cmd.ClientKeyPath = pref.ClientKey().Value()
	}
	actorConf := action.DefaultConfig()
	actorConf.ServerAddr = cmd.ServerAddr
	actorConf.Insecure = cmd.Insecure
	if !cmd.Insecure {
		actorConf.ServerCertPath = cmd.ServerCertPath
		actorConf.ClientCertPath = cmd.ClientCertPath
		actorConf.ClientKeyPath = cmd.ClientKeyPath
	}
	actorConf.User = cmd.User
	actorConf.Password = cmd.Password
	if actorConf.User == "" {
		if user, ok := os.LookupEnv("NEOSHELL_USER"); ok {
			actorConf.User = strings.ToLower(user)
		} else {
			actorConf.User = "sys"
		}
	}
	if actorConf.Password == "" {
		if pass, ok := os.LookupEnv("NEOSHELL_PASSWORD"); ok {
			actorConf.Password = pass
		} else {
			actorConf.Password = "manager"
		}
	}

	var command = ""
	if len(cmd.Args) > 0 {
		for i := range cmd.Args {
			if strings.Contains(cmd.Args[i], "\"") {
				cmd.Args[i] = strings.ReplaceAll(cmd.Args[i], "\"", "\\\"")
			}
			if strings.Contains(cmd.Args[i], " ") || strings.Contains(cmd.Args[i], "\t") {
				cmd.Args[i] = "\"" + cmd.Args[i] + "\""
			}
		}
		command = strings.TrimSpace(strings.Join(cmd.Args, " "))
	}
	interactive := len(command) == 0

	actor := action.NewActor(actorConf, interactive)
	if err := actor.Start(); err != nil {
		fmt.Fprintln(os.Stdout, "ERR", err.Error())
		return
	}
	defer actor.Stop()

	actor.Run(command)
}

func PrintHelp(subcommand string, helpShellText string) {
	serverAddr := DefaultServerAddress
	if shellPref, err := action.LoadPref(); err == nil {
		serverAddr = shellPref.Server().Value()
	}

	if subcommand != "" {
		fmt.Printf(helpShellText, serverAddr)
		if subcommand == "timeformat" {
			fmt.Println("  timeformats:")
			fmt.Printf("%s\n", util.HelpTimeformats())
		} else if subcommand == "tz" {
			fmt.Println("  timezones:")
			fmt.Printf("%s\n", util.HelpTimeZones())
		} else {
			targetCmd := action.FindCmd(subcommand)
			if targetCmd == nil {
				fmt.Printf("unknown sub-command %s\n\n", subcommand)
				return
			}
			fmt.Printf("%s shell %s\n", filepath.Base(os.Args[0]), targetCmd.Usage)
		}
	} else {
		fmt.Printf("\nshell "+helpShellText, serverAddr)
		fmt.Println("shell sub-commands:")
		cmds := action.Commands()
		for _, cmd := range cmds {
			lns := strings.Split(cmd.Usage, "\n")
			for i, l := range lns {
				if i == 0 {
					fmt.Printf("%s\n", l)
				} else {
					fmt.Printf("      %s\n", l)
				}
			}
		}
	}
}

func HelpKong(options kong.HelpOptions, ctx *kong.Context) error {
	serverAddr := "tcp://127.0.0.1:5655"
	if pref, err := action.LoadPref(); err == nil {
		serverAddr = pref.Server().Value()
	}
	fmt.Printf(`Usage: neoshell [<flags>] [<args>...]
  Flags:
    -h, --help             Show context-sensitive help.
        --version          show version
    -s, --server=<addr>    server address (default %s)
        --user=<user name> user name (default 'sys')
        --password=<pass>  password (default 'manager')
`, serverAddr)
	return nil
}
