//go:build windows
// +build windows

package server

import "os"

func daemonizeWithError(_, _ string, proc func(func() error, *os.File) error) error {
	return proc(func() error { return nil }, nil)
}
