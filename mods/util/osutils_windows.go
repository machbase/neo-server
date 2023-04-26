// go:build windows
package util

import "golang.org/x/sys/windows"

func GetWindowsVersion() (majorVersion, miorVersion, buildNumber uint32) {
	return windows.RtlGetNtVersionNumbers()
}
