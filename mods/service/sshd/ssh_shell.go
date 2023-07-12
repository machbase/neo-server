package sshd

import (
	"fmt"
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
	return shell
}
