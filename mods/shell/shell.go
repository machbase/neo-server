package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/machbase/neo-server/mods"
	"github.com/machbase/neo-server/mods/shell/internal/client"
	_ "github.com/machbase/neo-server/mods/shell/internal/cmd"
	"github.com/machbase/neo-server/mods/util"
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
	ServerCertPath string   `name:"server-cert" help:"path to server certificate"`
	ClientCertPath string   `name:"client-cert" help:"path to client certificate"`
	ClientKeyPath  string   `name:"client-key" help:"path to client key"`
	User           string   `name:"user" short:"u" default:"sys" help:"user name"`
	Password       string   `name:"password" short:"p" default:"manager" help:"password"`
}

var DefaultServerAddress = "tcp://127.0.0.1:5655"

func Shell(cmd *ShellCmd) {
	if cmd.Version {
		fmt.Fprintf(os.Stdout, "neoshell %s\n", mods.VersionString())
		return
	}

	for _, f := range cmd.Args {
		if f == "--help" || f == "-h" {
			targetCmd := client.FindCmd(strings.ToLower(cmd.Args[0]))
			if targetCmd == nil {
				fmt.Fprintf(os.Stdout, "unknown sub-command %s\n\n", cmd.Args[0])
				return
			}
			fmt.Fprintf(os.Stdout, "%s\n", targetCmd.Usage)
			return
		}
	}

	pref, _ := client.LoadPref()

	if cmd.ServerAddr == "" {
		if pref != nil {
			cmd.ServerAddr = pref.Server().Value()
		} else {
			cmd.ServerAddr = "tcp://127.0.0.1:5655"
		}
	}
	if cmd.ServerCertPath == "" && pref != nil {
		cmd.ServerCertPath = pref.ServerCert().Value()
	}
	if cmd.ClientCertPath == "" && pref != nil {
		cmd.ClientCertPath = pref.ClientCert().Value()
	}
	if cmd.ClientKeyPath == "" && pref != nil {
		cmd.ClientKeyPath = pref.ClientKey().Value()
	}
	clientConf := client.DefaultConfig()
	clientConf.ServerAddr = cmd.ServerAddr
	clientConf.ServerCertPath = cmd.ServerCertPath
	clientConf.ClientCertPath = cmd.ClientCertPath
	clientConf.ClientKeyPath = cmd.ClientKeyPath
	clientConf.User = cmd.User
	clientConf.Password = cmd.Password

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

	client := client.New(clientConf, interactive)
	if err := client.Start(); err != nil {
		fmt.Fprintln(os.Stdout, "ERR", err.Error())
		return
	}
	defer client.Stop()

	client.Run(command)
}

func PrintHelp(subcommand string, helpShellText string) {
	serverAddr := DefaultServerAddress
	if shellPref, err := client.LoadPref(); err == nil {
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
			targetCmd := client.FindCmd(subcommand)
			if targetCmd == nil {
				fmt.Printf("unknown sub-command %s\n\n", subcommand)
				return
			}
			fmt.Printf("%s shell %s\n", filepath.Base(os.Args[0]), targetCmd.Usage)
		}
	} else {
		fmt.Printf("\nshell "+helpShellText, serverAddr)
		fmt.Println("shell sub-commands:")
		cmds := client.Commands()
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
	if pref, err := client.LoadPref(); err == nil {
		serverAddr = pref.Server().Value()
	}
	fmt.Printf(`Usage: neoshell [<flags>] [<args>...]
  Flags:
    -h, --help             Show context-sensitive help.
        --version          show version
    -s, --server=<addr>    server address (default %s)
    -u, --user=<user name> user name (default 'sys')
    -p, --password=<pass>  password (default 'manager')
`, serverAddr)
	return nil
}
