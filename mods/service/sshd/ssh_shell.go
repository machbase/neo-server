package sshd

import (
	"fmt"
	"strings"

	"github.com/gliderlabs/ssh"
	"github.com/machbase/neo-server/mods/model"
)

type Shell struct {
	Cmd  string
	Args []string
	Envs map[string]string
}

func (svr *sshd) motdProvider(user string) string {
	return fmt.Sprintf("Greetings, %s\r\n%s\r\n",
		strings.ToUpper(user), svr.motdMessage)
}

func (svr *sshd) findShell(ss ssh.Session) (string, *Shell) {
	user := ss.User()
	var shell *Shell
	var shellId string

	toks := strings.SplitN(user, ":", 2)
	user = toks[0]
	if len(toks) == 2 {
		shellId = toks[1]
	} else {
		shellId = model.SHELLID_SHELL
	}
	shell = svr.shell(user, shellId)
	if shell == nil {
		return user, nil
	}
	if shellId == model.SHELLID_SHELL {
		shell.Envs["NEOSHELL_USER"] = strings.ToLower(user)
		shell.Envs["NEOSHELL_PASSWORD"] = svr.neoShellAccount[strings.ToLower(user)]
	}

	return user, shell
}
