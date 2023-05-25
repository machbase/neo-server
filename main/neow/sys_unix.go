//go:build !windows

package main

import "os/exec"

func sysProcAttr(cmd *exec.Cmd) {
	// do nothing
}
