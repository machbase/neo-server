package sshd

import (
	"fmt"
	"strings"

	"github.com/gliderlabs/ssh"
)

type Shell struct {
	Cmd  string
	Args []string
	Envs map[string]string
}

type ShellDefinition struct {
	Name string   `json:"-"`
	Args []string `json:"args"`

	Attributes map[string]string `json:"attributes,omitempty"`
}

func (def *ShellDefinition) Clone() *ShellDefinition {
	fmt.Println("---1")
	ret := &ShellDefinition{}
	ret.Name = def.Name
	fmt.Println("---2")
	copy(ret.Args, def.Args)
	fmt.Println("---3")
	if len(def.Attributes) > 0 {
		ret.Attributes = map[string]string{}
	}
	fmt.Println("---4")
	for k, v := range def.Attributes {
		ret.Attributes[k] = v
	}
	fmt.Println("---5")
	return ret
}

func (svr *sshd) motdProvider(user string) string {
	return fmt.Sprintf("Greetings, %s\r\n%s\r\n",
		strings.ToUpper(user), svr.motdMessage)
}

func (svr *sshd) findShell(ss ssh.Session) (string, *Shell) {
	user := ss.User()
	var shell *Shell
	if strings.Contains(user, ":") {
		userShell := ""
		toks := strings.SplitN(user, ":", 2)
		user = toks[0]
		userShell = toks[1]
		if userShell == "SHELL" {
			shell = svr.shellProvider(user)
		} else {
			shell = svr.customShellProvider(userShell)
		}
		if shell == nil {
			return user, nil
		}
	} else {
		shell = svr.shellProvider(user)
	}
	return user, shell
}
