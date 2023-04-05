//go:build windows

package sshd

import "os"

func setWinsize(f *os.File, w, h int) {
	// do nothing
}
