//go:build !windows

package util

import "fmt"

func GetWindowsVersion() (majorVersion, miorVersion, buildNumber uint32) {
	return 0, 0, 0
}

func MakeUnixDomainSocketPath(name string) string {
	return fmt.Sprintf("unix:///tmp/%s", name)
}
