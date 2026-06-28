package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	pathpkg "path"
	"strconv"
	"strings"
	"time"
)

// procDebugLog prints a timestamped debug line to stderr when PROC_ENTRY_DEBUG=1.
var procEntryDebug = os.Getenv("PROC_ENTRY_DEBUG") == "1"

func procDebugLog(format string, args ...any) {
	if !procEntryDebug {
		return
	}
	fmt.Fprintf(os.Stderr, "[proc-entry-debug] %s %s\n", time.Now().Format("15:04:05.000000"), fmt.Sprintf(format, args...))
}

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
	if _, bestMount := jr.filesystem.bestMatch(mountPoint); bestMount != mountPoint {
		return nil, nil
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
	procDebugLog("pid=%d Mkdir done: %s", meta.Pid, entry.baseDir)

	meta.StartedAt = formatProcProcessTime(entry.startedAt)
	if err := entry.writeMeta(meta); err != nil {
		procDebugLog("pid=%d writeMeta failed: %v", meta.Pid, err)
		_ = entry.remove()
		return nil, err
	}
	procDebugLog("pid=%d writeMeta done", meta.Pid)
	if err := entry.writeStatus("running"); err != nil {
		procDebugLog("pid=%d writeStatus(running) failed: %v", meta.Pid, err)
		_ = entry.remove()
		return nil, err
	}
	procDebugLog("pid=%d writeStatus(running) done", meta.Pid)
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
		entryName := strings.TrimSpace(entry.Name())
		// Accept only numeric PID directories to avoid path traversal or synthetic entries like "."/"..".
		if entryName == "" || entryName == "." || entryName == ".." || strings.Contains(entryName, "/") {
			continue
		}
		entryPid, convErr := strconv.Atoi(entryName)
		if convErr != nil {
			continue
		}
		pidDir := pathpkg.Join(rootDir, entryName)
		metaBytes, metaErr := jr.filesystem.ReadFile(pathpkg.Join(pidDir, "meta.json"))
		statusBytes, statusErr := jr.filesystem.ReadFile(pathpkg.Join(pidDir, "status.json"))
		if metaErr != nil || statusErr != nil {
			// meta.json or status.json is missing. This can happen when the creator
			// is still in the middle of writing the entry (Mkdir completed but
			// WriteFile/Rename not yet done). Since the directory name IS the PID,
			// skip the entry if that process is still alive to avoid a race where
			// we delete files that are being actively written. Only remove if the
			// PID is definitely gone.
			alive := procProcessExists(entryPid)
			procDebugLog("cleanup: pid=%d meta/status missing (metaErr=%v statusErr=%v) alive=%v", entryPid, metaErr, statusErr, alive)
			if alive {
				continue
			}
			_ = jr.removeProcessEntryDir(pidDir)
			continue
		}

		var meta procProcessMeta
		var status procProcessStatus
		if json.Unmarshal(metaBytes, &meta) != nil || json.Unmarshal(statusBytes, &status) != nil {
			procDebugLog("cleanup: pid=%d unmarshal failed, remove", entryPid)
			_ = jr.removeProcessEntryDir(pidDir)
			continue
		}
		if meta.Pid <= 0 || status.Pid != meta.Pid || !procProcessExists(meta.Pid) {
			procDebugLog("cleanup: pid=%d stale (meta.Pid=%d status.Pid=%d exists=%v), remove", entryPid, meta.Pid, status.Pid, procProcessExists(meta.Pid))
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
	procDebugLog("pid=%d finish(%s) start", entry.pid, state)
	if err := entry.writeStatus(state); err != nil {
		procDebugLog("pid=%d finish writeStatus failed: %v", entry.pid, err)
	}
	if err := entry.remove(); err != nil {
		procDebugLog("pid=%d finish remove failed: %v", entry.pid, err)
	}
	procDebugLog("pid=%d finish done", entry.pid)
}

func (entry *procProcessEntry) remove() error {
	return entry.removeOwnedFiles()
}

func (entry *procProcessEntry) removeOwnedFiles() error {
	for _, child := range []string{"meta.json", "status.json"} {
		childPath := pathpkg.Join(entry.baseDir, child)
		if !entry.ownsProcessEntryFile(childPath) {
			continue
		}
		if err := entry.jr.filesystem.Remove(childPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (entry *procProcessEntry) ownsProcessEntryFile(name string) bool {
	body, err := entry.jr.filesystem.ReadFile(name)
	if err != nil {
		return false
	}
	var probe struct {
		StartedAt string `json:"started_at"`
	}
	if json.Unmarshal(body, &probe) != nil {
		return false
	}
	return probe.StartedAt == formatProcProcessTime(entry.startedAt)
}

func (entry *procProcessEntry) writeJSONAtomic(name string, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')

	targetPath := pathpkg.Join(entry.baseDir, name)
	tmpPath := pathpkg.Join(entry.baseDir, "."+name+".tmp")
	procDebugLog("pid=%d writeJSONAtomic(%s): WriteFile %s", entry.pid, name, tmpPath)
	if err := entry.jr.filesystem.WriteFile(tmpPath, body); err != nil {
		procDebugLog("pid=%d writeJSONAtomic(%s): WriteFile failed: %v", entry.pid, name, err)
		return err
	}
	_, statErr := entry.jr.filesystem.Stat(targetPath)
	procDebugLog("pid=%d writeJSONAtomic(%s): Stat(%s) err=%v", entry.pid, name, targetPath, statErr)
	if statErr == nil {
		procDebugLog("pid=%d writeJSONAtomic(%s): Remove %s", entry.pid, name, targetPath)
		if err := entry.jr.filesystem.Remove(targetPath); err != nil {
			procDebugLog("pid=%d writeJSONAtomic(%s): Remove failed: %v", entry.pid, name, err)
			_ = entry.jr.filesystem.Remove(tmpPath)
			return err
		}
		procDebugLog("pid=%d writeJSONAtomic(%s): Remove done", entry.pid, name)
	}
	procDebugLog("pid=%d writeJSONAtomic(%s): Rename %s -> %s", entry.pid, name, tmpPath, targetPath)
	if err := entry.jr.filesystem.Rename(tmpPath, targetPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			procDebugLog("pid=%d writeJSONAtomic(%s): Rename source missing, fallback WriteFile %s", entry.pid, name, targetPath)
			if writeErr := entry.jr.filesystem.WriteFile(targetPath, body); writeErr == nil {
				procDebugLog("pid=%d writeJSONAtomic(%s): fallback WriteFile done", entry.pid, name)
				return nil
			} else {
				procDebugLog("pid=%d writeJSONAtomic(%s): fallback WriteFile failed: %v", entry.pid, name, writeErr)
				_ = entry.jr.filesystem.Remove(tmpPath)
				return writeErr
			}
		}
		procDebugLog("pid=%d writeJSONAtomic(%s): Rename FAILED: %v", entry.pid, name, err)
		_ = entry.jr.filesystem.Remove(tmpPath)
		return err
	}
	procDebugLog("pid=%d writeJSONAtomic(%s): Rename done", entry.pid, name)
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
		if err := jr.filesystem.Remove(childPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	if err := jr.filesystem.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
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
