//go:build !windows
// +build !windows

package booter

import (
	"fmt"
	"os"

	"github.com/sevlyar/go-daemon"
)

func Daemonize(bootlog string, pidfile string, proc func()) {
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	context := daemon.Context{
		LogFileName: bootlog,
		LogFilePerm: 0644,
		WorkDir:     workDir,
		Umask:       027,
	}

	child, err := context.Reborn()
	if err != nil {
		panic(err)
	}
	if child != nil {
		// post-work in parent
		if len(pidfile) > 0 {
			pfile, err := os.OpenFile(pidfile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Println(err.Error())
			}
			pfile.WriteString(fmt.Sprintf("%d", child.Pid))
			pfile.Close()
		}
		return
	} else {
		// post-work in child
		defer func() {
			if err := context.Release(); err != nil {
				fmt.Printf("Unable to release pid-file %s, %s\n", pidfile, err.Error())
			}
		}()
		proc()
	}
}
