package tql

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/util"
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

	rowNum := 1
	for i, subCmd := range subCmdList {
		args := subArgs[i]
		var cmd *exec.Cmd
		if ex, err := os.Executable(); err != nil {
			ErrorRecord(err).Tell(node.next)
			return
		} else {
			cmd = exec.Command(ex, append([]string{"shell", "--server", _grpcServer, subCmd}, args...)...)
		}
		node.task.LogInfo("machbase-neo shell", subCmd, strings.Join(args, " "))

		cmd.Env = append(os.Environ(), "NEOSHELL_USER="+node.task.consoleUser)
		cmd.Env = append(cmd.Env, "NEOSHELL_PASSWORD="+node.task.consoleOtp)

		if _, ok := node.GetValue("shell"); !ok {
			node.SetValue("shell", subCmd)
			cols := []*api.Column{
				{Name: "ROWNUM", Type: "int"},
				{Name: "RESULT", Type: "string"},
			}
			node.task.SetResultColumns(cols)
		}
		if output, err := cmd.Output(); err != nil {
			node.task.LogError(err.Error())
			ErrorRecord(err).Tell(node.next)
		} else {
			NewRecord(rowNum, strings.Join(append([]string{subCmd}, args...), " ")).Tell(node.next)
			rowNum++
			for _, ln := range strings.Split(string(output), "\n") {
				NewRecord(rowNum, ln).Tell(node.next)
				rowNum++
			}
		}
	}
}
