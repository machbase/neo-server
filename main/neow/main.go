package main

import (
	"os"
	"path/filepath"
	"runtime"
)

func main() {
	neoExePath, err := getMachbaseNeoPath()
	if err != nil {
		panic(err)
	}

	na := &neoAgent{
		exePath:     neoExePath,
		stateC:      make(chan NeoState, 1),
		outputLimit: 500,
	}
	na.Start()
}

func getMachbaseNeoPath() (string, error) {
	selfPath, _ := os.Executable()
	selfDir := filepath.Dir(selfPath)
	neoExePath := ""
	if runtime.GOOS == "windows" {
		neoExePath = filepath.Join(selfDir, "machbase-neo.exe")
	} else {
		neoExePath = filepath.Join(selfDir, "machbase-neo")
	}

	if _, err := os.Stat(neoExePath); err != nil {
		return "", err
	}

	return neoExePath, nil
}
