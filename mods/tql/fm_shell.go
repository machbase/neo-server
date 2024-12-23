package tql

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/util"
)

var _grpcServer string

func SetGrpcAddresses(addrs []string) {
	for _, addr := range addrs {
		if strings.HasPrefix(addr, "unix://") && runtime.GOOS != "windows" {
			_grpcServer = addr
			break
		}
		if strings.HasPrefix(addr, "tcp://127.0.0.1:") {
			_grpcServer = addr
		} else {
			_grpcServer = addr
		}
	}
}

func (node *Node) fmShell(cmd0 string, args0 ...string) {
	stripQuote := false
	subCmdList := []string{}
	subArgs := [][]string{}
	if len(args0) == 0 {
		buff := []string{}
		for _, line := range strings.Split(cmd0, "\n") {
			line = strings.TrimSpace(line)
			buff = append(buff, line)
			if !strings.HasSuffix(line, ";") {
				continue
			}
			line = strings.TrimSuffix(strings.Join(buff, " "), ";")
			buff = []string{}

			toks := util.SplitFields(line, stripQuote)
			if len(toks) == 0 || toks[0] == "" {
				continue
			}
			subCmdList = append(subCmdList, toks[0])
			subArgs = append(subArgs, toks[1:])
		}
		if len(buff) > 0 {
			line := strings.TrimSuffix(strings.Join(buff, " "), ";")
			toks := util.SplitFields(line, stripQuote)
			if len(toks) > 0 {
				subCmdList = append(subCmdList, toks[0])
				subArgs = append(subArgs, toks[1:])
			}
		}
	} else {
		subCmdList = append(subCmdList, cmd0)
		subArgs = append(subArgs, args0)
	}

	tmpFile, err := os.CreateTemp("", "runner")
	if err != nil {
		ErrorRecord(err).Tell(node.next)
		return
	}
	defer os.Remove(tmpFile.Name())
	for i, subCmd := range subCmdList {
		args := subArgs[i]

		switch strings.ToLower(subCmd) {
		case "exit", "quit", "set", "help", "clear", "shutdown":
			ErrorRecord(fmt.Errorf("command %q is not supported", subCmd)).Tell(node.next)
			continue
		default:
			line := strings.Join(append([]string{subCmd}, args...), " ")
			fmt.Fprintln(tmpFile, line+";")
		}
	}
	tmpFile.Close()

	var cmd *exec.Cmd
	if args, err := ShellExecutable(_grpcServer, tmpFile.Name()); err != nil {
		ErrorRecord(err).Tell(node.next)
		return
	} else {
		cmd = exec.Command(args[0], args[1:]...)
		cmd.Env = append(os.Environ(), "NEOSHELL_USER="+node.task.consoleUser)
		cmd.Env = append(cmd.Env, "NEOSHELL_PASSWORD="+node.task.consoleOtp)

		if _, ok := node.GetValue("shell"); !ok {
			cols := []*api.Column{
				api.MakeColumnRownum(),
				api.MakeColumnString("RESULT"),
			}
			node.task.SetResultColumns(cols)
		}
		if output, err := cmd.Output(); err != nil {
			node.task.LogError(err.Error())
			ErrorRecord(err).Tell(node.next)
		} else {
			var rowNum = 1
			for _, ln := range strings.Split(string(output), "\n") {
				NewRecord(rowNum, ln).Tell(node.next)
				rowNum++
			}
		}
	}
}

// intended expose this var to be replaced in shell test cases
var ShellExecutable = func(serverAddr string, scriptPath string) ([]string, error) {
	ex, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return []string{
		ex, "shell", "--server", serverAddr, "run", scriptPath,
	}, nil
}
