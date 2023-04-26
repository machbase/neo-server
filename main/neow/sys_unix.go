//go:build !windows

package main

import "os/exec"

func winMain(na *neoAgent) {
	// do nothing
}

func sysProcAttr(cmd *exec.Cmd) {
	// do nothing
}
