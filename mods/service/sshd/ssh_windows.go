//go:build windows

package sshsvr

import "os"

func setWinsize(f *os.File, w, h int) {
	// do nothing
}
