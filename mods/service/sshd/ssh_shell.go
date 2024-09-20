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
	Envs []string
}

func (svr *sshd) motdProvider(user string) string {
	return fmt.Sprintf("Greetings, %s\r\n%s\r\n",
		strings.ToUpper(user), svr.motdMessage)
}

// splitUserAndShell splits USER and SHELL_ID and CMD from the user string.
func (svr *sshd) splitUserAndShell(user string) (string, string, string) {
	if strings.HasPrefix(strings.ToLower(user), "sys+") {
		// only sys user can use this feature
		toks := strings.SplitN(user, "+", 2)
		return toks[0], "", toks[1]
	} else if strings.Contains(user, ":") {
		toks := strings.SplitN(user, ":", 2)
		return toks[0], toks[1], ""
	} else {
		return user, model.SHELLID_SHELL, ""
	}
}

func (svr *sshd) findShell(ss ssh.Session) (string, *Shell, string) {
	user := ss.User()
	var shell *Shell
	var shellId string
	var command string

	user, shellId, command = svr.splitUserAndShell(user)
	if command != "" {
		shell = &Shell{
			Cmd:  command,
			Envs: make([]string, 0),
		}
		return user, shell, ""
	}

	shell = svr.shell(user, shellId)
	if shell == nil {
		return user, nil, shellId
	}

	return user, shell, shellId
}
