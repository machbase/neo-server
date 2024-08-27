package pkgs

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-pkgdev/pkgs"
	"github.com/machbase/neo-server/mods/logging"
	"gopkg.in/yaml.v3"
)

type PkgBackend struct {
	sync.RWMutex
	log          logging.Log
	StartScripts PkgBackendScripts `yaml:"start"`
	StopScripts  PkgBackendScripts `yaml:"stop"`
	AutoStart    bool              `yaml:"auto_start,omitempty"`
	StdoutLog    string            `yaml:"stdout_log,omitempty"`
	StderrLog    string            `yaml:"stderr_log,omitempty"`
	Env          []string          `yaml:"env,omitempty"`
	EnvFile      string            `yaml:"env_file,omitempty"`

	dir         string
	cmd         *exec.Cmd
	stdoutLevel logging.Level
	stderrLevel logging.Level
}

type PkgBackendScripts []PkgScript

func (pss PkgBackendScripts) Find() PkgScript {
	var ret PkgScript
	for _, ps := range pss {
		if ps.Platform == "" {
			ret = ps
			continue
		}
		if runtime.GOOS == ps.Platform {
			ret = ps
			break
		}
	}
	return ret
}

type PkgScript struct {
	Run      string `yaml:"run"`
	Platform string `yaml:"on,omitempty"`
}

type PkgStatus string

const (
	Stopped PkgStatus = "stopped"
	Running PkgStatus = "running"
)

func LoadPkgBackend(pkgsDir string, pkgName string, installEnv []string) (*PkgBackend, error) {
	current := filepath.Join(pkgsDir, "dist", pkgName, "current")
	if _, err := os.Stat(current); err != nil {
		return nil, err
	}
	path, err := pkgs.Readlink(current)
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(pkgsDir, "dist", pkgName, path)
	}
	baseName := filepath.Base(path)

	backendFile := filepath.Join(pkgsDir, "dist", pkgName, baseName, ".backend.yml")
	if _, err := os.Stat(backendFile); err != nil {
		return nil, err
	}
	backendContent, err := os.ReadFile(backendFile)
	if err != nil {
		return nil, err
	}
	backend := &PkgBackend{
		log: logging.GetLog(pkgName),
		dir: path,
	}
	if err := yaml.Unmarshal(backendContent, backend); err != nil {
		return nil, err
	}
	if backend.EnvFile != "" {
		envFile := filepath.Join(path, backend.EnvFile)
		envContent, err := os.ReadFile(envFile)
		if err != nil {
			// ignore external env file error
			backend.log.Debugf("failed to read env file %s: %v", envFile, err)
		} else {
			envLines := []string{}
			for _, line := range strings.Split(string(envContent), "\n") {
				line = strings.TrimSpace(line)
				if len(line) > 0 && line[0] != '#' {
					envLines = append(envLines, line)
				}
			}
			backend.Env = append(backend.Env, envLines...)
		}
	}
	if len(installEnv) > 0 {
		backend.Env = append(installEnv, backend.Env...)
	}
	return backend, nil
}

func (ps *PkgBackend) Start() {
	ps.Lock()
	defer func() {
		ps.Unlock()
		if r := recover(); r != nil {
			ps.log.Errorf("panic: %v", r)
		}
	}()
	if ps.StdoutLog == "" {
		ps.StdoutLog = "NONE"
	}
	if ps.StderrLog == "" {
		ps.StderrLog = "NONE"
	}
	ps.stdoutLevel = logging.ParseLogLevel(ps.StdoutLog)
	ps.stderrLevel = logging.ParseLogLevel(ps.StderrLog)

	ps.log.Infof("start")
	ps.start0()
}

func (ps *PkgBackend) Stop() {
	ps.Lock()
	defer func() {
		ps.Unlock()
		if r := recover(); r != nil {
			ps.log.Errorf("panic: %v", r)
		}
	}()
	ps.log.Infof("stop")
	ps.stop0()
}

func (ps *PkgBackend) Status() PkgStatus {
	ps.RLock()
	defer ps.RUnlock()
	if ps.cmd == nil || ps.cmd.Process == nil {
		return Stopped
	}
	return Running
}

func (ps *PkgBackend) start0() {
	if ps.cmd != nil {
		ps.log.Errorf("already running")
		return
	}
	sc := ps.StartScripts.Find()
	if sc.Run == "" {
		ps.log.Errorf("start script not found")
		return
	}
	ps.cmd = makeCmd(sc.Run)
	ps.cmd.Dir = ps.dir
	ps.cmd.Env = append(os.Environ(), ps.Env...)
	ps.cmd.Stdout = &LevelWriter{log: ps.log, level: ps.stdoutLevel}
	ps.cmd.Stderr = &LevelWriter{log: ps.log, level: ps.stderrLevel}
	startWg := sync.WaitGroup{}
	startWg.Add(1)
	go func(cmd *exec.Cmd) {
		err := cmd.Start()
		if err != nil {
			ps.cmd = nil
			ps.log.Errorf("fail to start: cmd:%q error:%v", sc.Run, err)
			startWg.Done()
			return
		} else {
			startWg.Done()
		}
		err = cmd.Wait()
		if err != nil {
			ps.log.Errorf("fail to run: %v", err)
		} else {
			if ps.cmd != nil && ps.cmd.Process != nil {
				ps.log.Info("exit code", ps.cmd.ProcessState.ExitCode())
			}
		}
		ps.cmd = nil
	}(ps.cmd)
	startWg.Wait()
}

func (ps *PkgBackend) stop0() {
	if ps.cmd == nil || ps.cmd.Process == nil {
		ps.log.Errorf("not running")
		return
	}
	sc := ps.StopScripts.Find()
	if sc.Run == "" {
		ps.log.Errorf("stop script not found")
		return
	}
	cmd := makeCmd(sc.Run)
	cmd.Dir = ps.dir
	cmd.Env = append(os.Environ(), ps.Env...)
	cmd.Stdout = &LevelWriter{log: ps.log, level: ps.stdoutLevel}
	cmd.Stderr = &LevelWriter{log: ps.log, level: ps.stderrLevel}
	err := cmd.Start()
	if err != nil {
		ps.log.Errorf("fail to stop: cmd:%q error:%v", sc.Run, err)
		return
	}
	err = cmd.Wait()
	if err != nil {
		ps.log.Errorf("fail to run: %v", err)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		count := 0
		dur := 100 * time.Millisecond
		tick := time.NewTimer(dur)
		for range tick.C {
			if ps.cmd == nil {
				break
			}
			count++
			if time.Duration(count)*dur > 5*time.Second {
				ps.log.Errorf("timeout")
				break
			}
		}
	}()
	wg.Wait()
}

func makeCmd(script string) *exec.Cmd {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		lines := strings.Split(script, "\n")
		for i, line := range lines {
			lines[i] = strings.TrimSuffix(strings.TrimSpace(line), "^")
		}
		cmdLine := strings.Join(lines, " ")
		cmd = exec.Command("cmd", "/c", cmdLine)
	} else {
		lines := strings.Split(script, "\n")
		for i, line := range lines {
			lines[i] = strings.TrimSpace(strings.TrimSuffix(line, "\\"))
		}
		cmdLine := strings.Join(lines, " ")
		cmd = exec.Command("sh", "-c", cmdLine)
	}
	return cmd
}

type LevelWriter struct {
	log   logging.Log
	level logging.Level
}

func (lw *LevelWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	if p[len(p)-1] == '\n' {
		p = p[:len(p)-1]
	}
	if p[len(p)-1] == '\r' {
		p = p[:len(p)-1]
	}
	lw.log.Log(lw.level, string(p))
	return
}
