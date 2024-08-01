package tql

import (
	"os"
	"os/exec"
	"strings"
)

func (x *Node) fmShell(subCmd string, args ...string) {
	var cmd *exec.Cmd
	if ex, err := os.Executable(); err != nil {
		ErrorRecord(err).Tell(x.next)
		return
	} else {
		cmd = exec.Command(ex, append([]string{"shell", subCmd}, args...)...)
	}
	x.task.LogInfo("machbase-neo shell", subCmd, strings.Join(args, " "))

	cmd.Env = append(os.Environ(), "NEOSHELL_USER="+x.task.consoleUser)
	cmd.Env = append(cmd.Env, "NEOSHELL_PASSWORD="+x.task.consoleOtp)
	if output, err := cmd.Output(); err != nil {
		x.task.LogError(err.Error())
		ErrorRecord(err).Tell(x.next)
	} else {
		for i, ln := range strings.Split(string(output), "\n") {
			NewRecord(i+1, ln).Tell(x.next)
		}
	}
}
