package server

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
)

func (s *Server) initShellProvider() error {
	tcpCandidates := []string{}
	for _, addr := range s.Http.Listeners {
		if !strings.HasPrefix(addr, "tcp://") {
			continue
		}
		tcpCandidates = append(tcpCandidates, strings.TrimPrefix(addr, "tcp://"))
	}
	if len(tcpCandidates) == 0 {
		s.log.Warn("no port found for internal communication")
		return nil
	}

	candidate := ""
	for _, addr := range tcpCandidates {
		if isLoopbackCandidate(addr) {
			candidate = addr
			break
		}
	}
	if candidate == "" {
		for _, addr := range tcpCandidates {
			if !isAnyIfaceCandidate(addr) {
				continue
			}
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				continue
			}
			// Use IPv6 loopback for IPv6 any address, IPv4 loopback for IPv4 any address
			loopbackIP := "127.0.0.1"
			if strings.Contains(host, ":") {
				loopbackIP = "::1"
			}
			candidate = net.JoinHostPort(loopbackIP, port)
			break
		}
	}
	if candidate == "" {
		candidate = tcpCandidates[0]
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
	shellCmd = append(shellCmd, "--server", candidate)
	s.models.ShellProvider().SetDefaultShellCommand(strings.Join(shellCmd, " "))
	s.log.Trace("Set shell command:", strings.Join(shellCmd, " "))
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

// isAnyIfaceCandidate checks if the candidate address is bound to any interface (
func isAnyIfaceCandidate(candidate string) bool {
	host := candidate
	if parsedHost, _, err := net.SplitHostPort(candidate); err == nil {
		host = parsedHost
	}
	host = strings.Trim(host, "[]")
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsUnspecified()
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
