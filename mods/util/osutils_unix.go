//go:build !windows

package util

func GetWindowsVersion() (majorVersion, miorVersion, buildNumber uint32) {
	return 0, 0, 0
}
