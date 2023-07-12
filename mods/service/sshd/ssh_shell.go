package sshd

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/gliderlabs/ssh"
)

func (svr *sshd) motdProvider(user string) string {
	return fmt.Sprintf("Greetings, %s\r\n%s\r\n",
		strings.ToUpper(user), svr.motdMessage)
}

func (svr *sshd) findShellDefinition(ss ssh.Session) (string, *ShellDefinition) {
	user := ss.User()
	var shellDef *ShellDefinition
	if strings.Contains(user, ":") {
		userShell := ""
		toks := strings.SplitN(user, ":", 2)
		user = toks[0]
		userShell = toks[1]
		shellDef = svr.shellDefinitionProvider(userShell)
	}
	return user, shellDef
}

func (svr *sshd) buildShell(user string, shellDef *ShellDefinition) *Shell {
	var shell *Shell
	if shellDef == nil {
		shell = svr.shellProvider(user)
	} else {
		shell = &Shell{}
		shell.Cmd = shellDef.Args[0]
		if len(shellDef.Args) > 1 {
			shell.Args = shellDef.Args[1:]
		}
	}

	shell.Envs = map[string]string{}
	if runtime.GOOS == "windows" {
		envs := os.Environ()
		for _, line := range envs {
			if !strings.Contains(line, "=") {
				continue
			}
			toks := strings.SplitN(line, "=", 2)
			if len(toks) != 2 {
				continue
			}
			shell.Envs[strings.TrimSpace(toks[0])] = strings.TrimSpace(toks[1])
		}
		if _, ok := shell.Envs["USERPROFILE"]; !ok {
			userHomeDir, err := os.UserHomeDir()
			if err != nil {
				userHomeDir = "."
			}
			shell.Envs["USERPROFILE"] = userHomeDir
		}
	} else {
		if _, ok := shell.Envs["HOME"]; !ok {
			if userHomeDir, err := os.UserHomeDir(); err == nil {
				shell.Envs["HOME"] = userHomeDir
			}
		}
	}

	return shell
}
