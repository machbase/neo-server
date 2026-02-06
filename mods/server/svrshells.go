package server

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
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

	shellCmd := []string{}
	if len(os.Args) > 0 {
		if executable, err := os.Executable(); err != nil {
			shellCmd = append(shellCmd, fmt.Sprintf("%q", os.Args[0]), "shell")
		} else {
			shellCmd = append(shellCmd, fmt.Sprintf("%q", executable), "shell")
		}
	}
	fsMgr := ssfs.Default()
	lst := fsMgr.ListMounts()
	for _, mnt := range lst {
		if mnt == "/" {
			if root, err := fsMgr.RealPath("/"); err == nil {
				shellCmd = append(shellCmd, "-v", fmt.Sprintf(`"/work=%s"`, root))
			}
		} else {
			if realPath, err := fsMgr.RealPath(mnt); err == nil {
				shellCmd = append(shellCmd, "-v", fmt.Sprintf(`"/work%s=%s"`, mnt, realPath))
			}
		}
	}
	shellCmd = append(shellCmd, "--server", candidates[0])
	s.models.ShellProvider().SetDefaultShellCommand(strings.Join(shellCmd, " "))
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
