package test

import (
	"os"
	"path/filepath"
	"runtime"
)

func SupportDockerTest() bool {
	if os.Getenv("CI") == "true" {
		return false
	}
	if runtime.GOOS == "linux" {
		return runtime.GOARCH == "amd64"
	}
	if runtime.GOOS == "windows" {
		return false
	}
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err == nil {
			// new docker path for mac docker desktop
			path := filepath.Join(home, ".docker", "run", "docker.sock")
			_, err = os.Stat(path)
			if err == nil {
				os.Setenv("DOCKER_HOST", "unix://"+path)
				return true
			}
		}
		// fallback to old docker path for mac docker desktop
		_, err = os.Stat("/var/run/docker.sock")
		if err != nil {
			return false
		}
	}
	return true
}
