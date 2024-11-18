package server

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/machbase/neo-server/v8/mods/service/sshd"
	"github.com/machbase/neo-server/v8/mods/util"
)

func (s *svr) initShellProvider() {
	candidates := []string{}
	for _, addr := range s.conf.Grpc.Listeners {
		if runtime.GOOS == "windows" && strings.HasPrefix(addr, "unix://") {
			continue
		}
		candidates = append(candidates, addr)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if strings.HasPrefix(candidates[i], "unix://") {
			return true
		}
		if candidates[i] == "127.0.0.1" || candidates[i] == "localhost" {
			return true
		}
		return false
	})
	if len(candidates) == 0 {
		s.log.Warn("no port found for internal communication")
		return
	}

	shellCmd := ""
	if len(os.Args) > 0 {
		if exename, err := os.Executable(); err != nil {
			shellCmd = os.Args[0]
		} else {
			shellCmd = exename
		}
	}
	if s.conf.Grpc.Insecure {
		shellCmd = fmt.Sprintf(`"%s" shell --insecure --server %s`, shellCmd, candidates[0])
	} else {
		shellCmd = fmt.Sprintf(`"%s" shell --server %s`, shellCmd, candidates[0])
	}
	s.models.ShellProvider().SetDefaultShellCommand(shellCmd)
}

// sshd shell provider
func (s *svr) provideShellForSsh(user string, shellId string) *sshd.Shell {
	shellId = strings.ToUpper(shellId)
	shellDef, _ := s.models.ShellProvider().GetShell(shellId)
	if shellDef == nil {
		return nil
	}

	parsed := util.SplitFields(shellDef.Command, true)
	if len(parsed) == 0 {
		return nil
	}

	shell := &sshd.Shell{}

	shell.Cmd = parsed[0]
	if len(parsed) > 1 {
		shell.Args = parsed[1:]
	}

	shell.Envs = append([]string{}, os.Environ()...)
	if runtime.GOOS == "windows" {
		has := false
		for _, env := range shell.Envs {
			if strings.HasPrefix(env, "USERPROFILE=") {
				has = true
				break
			}
		}
		if !has {
			userHomeDir, err := os.UserHomeDir()
			if err != nil {
				userHomeDir = "."
			}
			shell.Envs = append(shell.Envs, "USERPROFILE="+userHomeDir)
		}
	}
	return shell
}
