package engine

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	pathpkg "path"
	"strconv"
	"strings"
	"time"
)

const procProcessRoot = "process"

type procProcessMeta struct {
	Pid                       int      `json:"pid"`
	Ppid                      int      `json:"ppid"`
	Pgid                      int      `json:"pgid"`
	Command                   string   `json:"command"`
	Args                      []string `json:"args"`
	Cwd                       string   `json:"cwd"`
	StartedAt                 string   `json:"started_at"`
	ServiceControllerClientID string   `json:"service_controller_client_id"`
	ExecPath                  string   `json:"exec_path,omitempty"`
}

type procProcessStatus struct {
	Pid       int    `json:"pid"`
	State     string `json:"state"`
	UpdatedAt string `json:"updated_at"`
	StartedAt string `json:"started_at"`
}

type procProcessEntry struct {
	jr        *JSRuntime
	baseDir   string
	pid       int
	startedAt time.Time
}

func (jr *JSRuntime) createProcessEntry(ex *exec.Cmd) (*procProcessEntry, error) {
	if ex == nil || ex.Process == nil {
		return nil, nil
	}
	meta := procProcessMeta{
		Pid:                       ex.Process.Pid,
		Ppid:                      procProcessCurrentPID(),
		Pgid:                      procProcessGroupID(ex.Process.Pid),
		Command:                   procProcessCommand(ex),
		Args:                      procProcessArgs(ex),
		Cwd:                       procProcessCwd(jr, ex),
		ServiceControllerClientID: envStringValue(jr.Env.vars, ControllerClientIDEnv),
		ExecPath:                  ex.Path,
	}
	return jr.createProcProcessEntry(meta)
}

func (jr *JSRuntime) createCurrentProcessEntry(command string, args []string) (*procProcessEntry, error) {
	meta := procProcessMeta{
		Pid:                       procProcessCurrentPID(),
		Ppid:                      os.Getppid(),
		Pgid:                      procProcessGroupID(procProcessCurrentPID()),
		Command:                   strings.TrimSpace(command),
		Args:                      append([]string{}, args...),
		Cwd:                       procProcessRuntimeCwd(jr),
		ServiceControllerClientID: envStringValue(jr.Env.vars, ControllerClientIDEnv),
		ExecPath:                  strings.TrimSpace(command),
	}
	return jr.createProcProcessEntry(meta)
}

func (jr *JSRuntime) createProcProcessEntry(meta procProcessMeta) (*procProcessEntry, error) {
	if jr == nil || jr.Env == nil || jr.filesystem == nil {
		return nil, nil
	}
	if meta.Pid <= 0 {
		return nil, nil
	}

	controllerAddr := envStringValue(jr.Env.vars, ControllerAddressEnv)
	if controllerAddr == "" {
		return nil, nil
	}

	mountPoint := CleanPath(envStringValue(jr.Env.vars, ControllerSharedMountEnv))
	if mountPoint == "/" {
		mountPoint = DefaultControllerSharedMount
	}

	entry := &procProcessEntry{
		jr:        jr,
		baseDir:   pathpkg.Join(mountPoint, procProcessRoot, strconv.Itoa(meta.Pid)),
		pid:       meta.Pid,
		startedAt: jr.Now(),
	}

	jr.procCleanupOnce.Do(func() {
		_ = jr.cleanupStaleProcessEntries(mountPoint)
	})

	if err := jr.filesystem.Mkdir(entry.baseDir); err != nil {
		return nil, err
	}

	meta.StartedAt = formatProcProcessTime(entry.startedAt)
	if err := entry.writeMeta(meta); err != nil {
		_ = entry.remove()
		return nil, err
	}
	if err := entry.writeStatus("running"); err != nil {
		_ = entry.remove()
		return nil, err
	}
	return entry, nil
}

func (jr *JSRuntime) cleanupStaleProcessEntries(mountPoint string) error {
	rootDir := pathpkg.Join(mountPoint, procProcessRoot)
	entries, err := jr.filesystem.ReadDir(rootDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry == nil || !entry.IsDir() {
			continue
		}
		pidDir := pathpkg.Join(rootDir, entry.Name())
		metaBytes, metaErr := jr.filesystem.ReadFile(pathpkg.Join(pidDir, "meta.json"))
		statusBytes, statusErr := jr.filesystem.ReadFile(pathpkg.Join(pidDir, "status.json"))
		if metaErr != nil || statusErr != nil {
			_ = jr.removeProcessEntryDir(pidDir)
			continue
		}

		var meta procProcessMeta
		var status procProcessStatus
		if json.Unmarshal(metaBytes, &meta) != nil || json.Unmarshal(statusBytes, &status) != nil {
			_ = jr.removeProcessEntryDir(pidDir)
			continue
		}
		if meta.Pid <= 0 || status.Pid != meta.Pid || !procProcessExists(meta.Pid) {
			_ = jr.removeProcessEntryDir(pidDir)
		}
	}
	return nil
}

func (entry *procProcessEntry) writeMeta(meta procProcessMeta) error {
	return entry.writeJSONAtomic("meta.json", meta)
}

func (entry *procProcessEntry) writeStatus(state string) error {
	status := procProcessStatus{
		Pid:       entry.pid,
		State:     state,
		UpdatedAt: formatProcProcessTime(entry.jr.Now()),
		StartedAt: formatProcProcessTime(entry.startedAt),
	}
	return entry.writeJSONAtomic("status.json", status)
}

func (entry *procProcessEntry) finish(exitCode int) {
	state := "exited"
	if exitCode != 0 {
		state = "failed"
	}
	_ = entry.writeStatus(state)
	_ = entry.remove()
}

func (entry *procProcessEntry) remove() error {
	return entry.jr.removeProcessEntryDir(entry.baseDir)
}

func (entry *procProcessEntry) writeJSONAtomic(name string, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')

	targetPath := pathpkg.Join(entry.baseDir, name)
	tmpPath := pathpkg.Join(entry.baseDir, "."+name+".tmp")
	if err := entry.jr.filesystem.WriteFile(tmpPath, body); err != nil {
		return err
	}
	if _, err := entry.jr.filesystem.Stat(targetPath); err == nil {
		if err := entry.jr.filesystem.Remove(targetPath); err != nil {
			_ = entry.jr.filesystem.Remove(tmpPath)
			return err
		}
	}
	if err := entry.jr.filesystem.Rename(tmpPath, targetPath); err != nil {
		_ = entry.jr.filesystem.Remove(tmpPath)
		return err
	}
	return nil
}

func (jr *JSRuntime) removeProcessEntryDir(name string) error {
	for _, child := range []string{
		"meta.json",
		"status.json",
		".meta.json.tmp",
		".status.json.tmp",
	} {
		childPath := pathpkg.Join(name, child)
		if err := jr.filesystem.Remove(childPath); err != nil && err != fs.ErrNotExist {
			return err
		}
	}
	if err := jr.filesystem.Remove(name); err != nil && err != fs.ErrNotExist {
		return err
	}
	return nil
}

func formatProcProcessTime(ts time.Time) string {
	return ts.Format(time.RFC3339Nano)
}

func procProcessCurrentPID() int {
	return procProcessOSPid()
}

func procProcessCommand(ex *exec.Cmd) string {
	if ex == nil {
		return ""
	}
	if strings.TrimSpace(ex.Path) != "" {
		return ex.Path
	}
	if len(ex.Args) > 0 {
		return ex.Args[0]
	}
	return ""
}

func procProcessArgs(ex *exec.Cmd) []string {
	if ex == nil || len(ex.Args) <= 1 {
		return []string{}
	}
	return append([]string{}, ex.Args[1:]...)
}

func procProcessCwd(jr *JSRuntime, ex *exec.Cmd) string {
	if ex != nil && strings.TrimSpace(ex.Dir) != "" {
		return ex.Dir
	}
	return procProcessRuntimeCwd(jr)
}

func procProcessRuntimeCwd(jr *JSRuntime) string {
	if jr != nil && jr.Env != nil {
		if cwd, ok := jr.Env.Get("PWD").(string); ok && cwd != "" {
			return cwd
		}
	}
	return "/"
}

func parseProcProcessEntry(data []byte) (procProcessMeta, error) {
	var meta procProcessMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return procProcessMeta{}, err
	}
	if meta.Pid <= 0 {
		return procProcessMeta{}, fmt.Errorf("invalid pid")
	}
	return meta, nil
}

func readProcProcessMetaFromFS(filesystem fs.ReadFileFS, path string) (procProcessMeta, error) {
	data, err := filesystem.ReadFile(path)
	if err != nil {
		return procProcessMeta{}, err
	}
	return parseProcProcessEntry(data)
}
