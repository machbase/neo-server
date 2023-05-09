package main

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/machbase/neo-server/mods/util"
	"github.com/machbase/neo-server/mods/util/ini"
)

func main() {
	neoExePath, err := getMachbaseNeoPath()
	if err != nil {
		panic(err)
	}

	neoExeArgs := []string{"serve"}
	autoStart := false

	iniPath, err := getStartupIni()
	if err == nil {
		cfg := ini.Load(iniPath)
		sect, err := cfg.Section(cfg.DefaultSectionName())
		if err == nil {
			valueString := sect.GetValueWithDefault("args", "")
			values := util.SplitFields(valueString, true)
			neoExeArgs = append(neoExeArgs, values...)

			autoStart = sect.GetBoolWithDefault("auto-start", false)
		}
	}

	na := &neoAgent{
		exePath:     neoExePath,
		exeArgs:     neoExeArgs,
		autoStart:   autoStart,
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

func getStartupIni() (string, error) {
	selfPath := os.Args[0]
	selfDir := filepath.Dir(selfPath)
	iniPath := filepath.Join(selfDir, "neow.ini")
	if _, err := os.Stat(iniPath); err != nil {
		return "", err
	}
	return iniPath, nil
}
