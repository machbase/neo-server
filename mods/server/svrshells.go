package server

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/machbase/neo-server/v8/mods/util"
)

func (s *Server) initShellProvider() error {
	candidates := []string{}
	for _, addr := range s.Http.Listeners {
		if !strings.HasPrefix(addr, "tcp://") {
			continue
		}
		candidates = append(candidates, strings.TrimPrefix(addr, "tcp://"))
	}
	sort.Slice(candidates, func(i, j int) bool {
		iLoop := isLoopbackCandidate(candidates[i])
		jLoop := isLoopbackCandidate(candidates[j])
		if iLoop != jLoop {
			return iLoop
		}
		return candidates[i] < candidates[j]
	})
	if len(candidates) == 0 {
		s.log.Warn("no port found for internal communication")
		return nil
	}

	shellCmd := ""
	if len(os.Args) > 0 {
		if exename, err := os.Executable(); err != nil {
			shellCmd = os.Args[0]
		} else {
			shellCmd = exename
		}
	}
	shellCmd = fmt.Sprintf(`"%s" shell --server %s`, shellCmd, candidates[0])
	s.models.ShellProvider().SetDefaultShellCommand(shellCmd)
	return nil
}

func isLoopbackCandidate(candidate string) bool {
	host := candidate
	if parsedHost, _, err := net.SplitHostPort(candidate); err == nil {
		host = parsedHost
	}
	host = strings.Trim(host, "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

// sshd shell provider
func (s *Server) provideShellForSsh(user string, shellId string) *SshShell {
	shellId = strings.ToUpper(shellId)
	shellDef, _ := s.models.ShellProvider().GetShell(shellId)
	if shellDef == nil {
		return nil
	}

	parsed := util.SplitFields(shellDef.Command, true)
	if len(parsed) == 0 {
		return nil
	}

	shell := &SshShell{}

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
