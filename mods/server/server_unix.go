//go:build !windows
// +build !windows

package server

import "fmt"

func doService(_ *Service) {
	fmt.Println("command 'service' is only available on Windows")
}
