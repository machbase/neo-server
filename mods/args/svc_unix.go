//go:build !windows
// +build !windows

package args

import "fmt"

func doService(_ *Service) {
	fmt.Println("command 'service' is only available on Windows")
}
