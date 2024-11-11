// go:build windows
package util

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func GetWindowsVersion() (majorVersion, minorVersion, buildNumber uint32) {
	return windows.RtlGetNtVersionNumbers()
}

func GetTempDirPath() string {
	if tempDir := os.Getenv("TEMP"); tempDir == "" || len(tempDir) >= 100 {
		if drive := os.Getenv("SYSTEMDRIVE"); drive != "" {
			return drive + "\\"
		} else {
			return "C:\\"
		}
	} else {
		return tempDir
	}
}

func MakeUnixDomainSocketPath(name string) string {
	if tempDir := os.Getenv("TEMP"); tempDir == "" || len(tempDir)+len(name) >= 100 {
		if drive := os.Getenv("SYSTEMDRIVE"); drive != "" {
			return fmt.Sprintf("unix://%s\\%s", drive, name)
		} else {
			return fmt.Sprintf("unix://C:\\%s", name)
		}
	} else {
		return fmt.Sprintf("unix://%s", filepath.Join(tempDir, name))
	}
}
