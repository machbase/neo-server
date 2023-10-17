package server

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/machbase/neo-server/mods/service/sshd"
	"github.com/machbase/neo-server/mods/util"
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
	s.models.ShellProvider().SetDefaultShellCommand(fmt.Sprintf(`"%s" shell --server %s`, shellCmd, candidates[0]))
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

	shell.Envs = map[string]string{}
	envs := os.Environ()
	for _, line := range envs {
		toks := strings.SplitN(line, "=", 2)
		if len(toks) != 2 {
			continue
		}
		shell.Envs[toks[0]] = toks[1]
	}
	if runtime.GOOS == "windows" {
		if _, ok := shell.Envs["USERPROFILE"]; !ok {
			userHomeDir, err := os.UserHomeDir()
			if err != nil {
				userHomeDir = "."
			}
			shell.Envs["USERPROFILE"] = userHomeDir
		}
	}
	return shell
}
