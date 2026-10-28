package server

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/machbase/neo-server/v8/jsh/engine"
)

// WorkMountPoint is the JSH virtual filesystem path that corresponds to
// the root ("/") of the server side file system.
const WorkMountPoint = "/work"

// MountSpec is a normalized --file mount entry.
// MountPoint is relative to the workspace root ("/" means WorkMountPoint),
// Dir is the absolute host directory.
type MountSpec struct {
	MountPoint string
	Dir        string
}

// String returns the "mountPoint=dir" form accepted by NewWorkspaceFileSystem.
func (ms MountSpec) String() string {
	return ms.MountPoint + "=" + ms.Dir
}

// WorkMountPoint returns the JSH virtual filesystem mount point of the spec.
func (ms MountSpec) WorkMountPoint() string {
	return WorkPath(ms.MountPoint)
}

// WorkPath converts a server side file system path ("/a/b") into
// the JSH virtual filesystem path ("/work/a/b").
func WorkPath(p string) string {
	p = path.Clean("/" + strings.TrimPrefix(filepath.ToSlash(p), "/"))
	if p == "/" {
		return WorkMountPoint
	}
	return WorkMountPoint + p
}

// ParseFileDirs parses the values of the --file flag (FileDirs) into mount specs.
//
// Each value may contain multiple entries separated by ','.
// An entry is either "mountPoint=hostDir" or a bare "hostDir" whose mount point
// is the base name of the directory. Every mount point is placed under the
// workspace root, so "/=/dir,/a=/other" means "/work" -> "/dir" and
// "/work/a" -> "/other" in the JSH virtual filesystem.
//
// The server configuration wraps the --file flag value as "/=<value>",
// so "/=/=/dir,/a=/other" and "/=/a=/other" are unwrapped as well.
func ParseFileDirs(values []string) ([]MountSpec, error) {
	ret := []MountSpec{}
	for _, value := range values {
		for _, tok := range strings.Split(value, ",") {
			tok = strings.TrimSpace(tok)
			if tok == "" {
				continue
			}
			spec, err := parseMountSpec(tok)
			if err != nil {
				return nil, err
			}
			if slices.ContainsFunc(ret, func(s MountSpec) bool { return s.MountPoint == spec.MountPoint }) {
				return nil, fmt.Errorf("duplicate file mount point %q", spec.MountPoint)
			}
			ret = append(ret, spec)
		}
	}
	return ret, nil
}

func parseMountSpec(entry string) (MountSpec, error) {
	// unwrap "/=" added by the config template when the flag value itself has mount specs
	for strings.HasPrefix(entry, "/=") && strings.Contains(entry[2:], "=") {
		entry = entry[2:]
	}
	mnt, dir := "", entry
	if idx := strings.Index(entry, "="); idx >= 0 {
		mnt, dir = entry[:idx], entry[idx+1:]
	} else {
		mnt = filepath.Base(dir)
	}
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return MountSpec{}, fmt.Errorf("invalid file mount %q, empty directory", entry)
	}
	mnt = filepath.ToSlash(strings.TrimSpace(mnt))
	for _, seg := range strings.Split(mnt, "/") {
		if seg == ".." {
			return MountSpec{}, fmt.Errorf("invalid file mount %q, mount point should be under %s", entry, WorkMountPoint)
		}
	}
	mnt = path.Clean("/" + strings.TrimPrefix(mnt, "/"))
	abs, err := filepath.Abs(dir)
	if err != nil {
		return MountSpec{}, fmt.Errorf("invalid file mount %q, %w", entry, err)
	}
	return MountSpec{MountPoint: mnt, Dir: abs}, nil
}

// NormalizeFileDirs parses values with ParseFileDirs and returns them
// in the canonical "mountPoint=absDir" form.
func NormalizeFileDirs(values []string) ([]string, error) {
	specs, err := ParseFileDirs(values)
	if err != nil {
		return nil, err
	}
	ret := make([]string, len(specs))
	for i, s := range specs {
		ret[i] = s.String()
	}
	return ret, nil
}

// MountSpecs returns the current mounts sorted by mount point,
// so that a parent mount always precedes its nested mounts.
func (service *WorkspaceFileSystem) MountSpecs() []MountSpec {
	if service == nil {
		return nil
	}
	ret := make([]MountSpec, 0, len(service.mounts))
	for _, bd := range service.mounts {
		ret = append(ret, MountSpec{MountPoint: bd.mountPoint, Dir: bd.abspath})
	}
	slices.SortFunc(ret, func(a, b MountSpec) int { return strings.Compare(a.MountPoint, b.MountPoint) })
	return ret
}

// RootDir returns the host directory mounted at "/" (JSH "/work").
func (service *WorkspaceFileSystem) RootDir() (string, bool) {
	if service == nil {
		return "", false
	}
	if bd := service.mounts[WorkMountPoint]; bd != nil {
		return bd.abspath, true
	}
	return "", false
}

// WorkFSTabs returns the JSH mount table that maps every server side file system
// mount under WorkMountPoint. Mounts whose host directory is not accessible are skipped
// so that a single broken mount does not prevent the runtime from starting.
func (service *WorkspaceFileSystem) WorkFSTabs() engine.FSTabs {
	ret := engine.FSTabs{}
	for _, spec := range service.MountSpecs() {
		if info, err := os.Stat(spec.Dir); err != nil || !info.IsDir() {
			continue
		}
		ret = append(ret, engine.FSTab{MountPoint: spec.WorkMountPoint(), Source: spec.Dir})
	}
	return ret
}

// ProcessWorkFSTabs returns WorkFSTabs for a child jsh/shell process.
// When no host directory is mounted at WorkMountPoint, a virtual "/work" tab
// (empty Source) is prepended so the child does not fall back to its current directory.
func (service *WorkspaceFileSystem) ProcessWorkFSTabs() engine.FSTabs {
	tabs := service.WorkFSTabs()
	if tabs.HasMountPoint(WorkMountPoint) {
		return tabs
	}
	return append(engine.FSTabs{{MountPoint: WorkMountPoint}}, tabs...)
}

// WorkMountArgs returns the "-v" command line arguments of the jsh/shell commands
// equivalent to ProcessWorkFSTabs.
func (service *WorkspaceFileSystem) WorkMountArgs() []string {
	ret := []string{}
	for _, tab := range service.ProcessWorkFSTabs() {
		ret = append(ret, "-v", tab.MountPoint+"="+tab.Source)
	}
	return ret
}

// CheckWorkMounts reports mounts whose host directory is not accessible.
// Such mounts are skipped by WorkFSTabs, so they are reported as warnings.
func (service *WorkspaceFileSystem) CheckWorkMounts() []string {
	ret := []string{}
	for _, spec := range service.MountSpecs() {
		info, err := os.Stat(spec.Dir)
		if err != nil {
			ret = append(ret, fmt.Sprintf("file mount %q -> %q is not accessible, %s", spec.WorkMountPoint(), spec.Dir, err.Error()))
		} else if !info.IsDir() {
			ret = append(ret, fmt.Sprintf("file mount %q -> %q is not a directory", spec.WorkMountPoint(), spec.Dir))
		}
	}
	return ret
}

// ValidateWorkMounts rechecks the configured mounts against current host entries.
// The engine owns conflict detection, including changes since construction.
func (service *WorkspaceFileSystem) ValidateWorkMounts() error {
	validationFS := engine.NewFS()
	for _, spec := range service.MountSpecs() {
		if err := validationFS.MountStrict(spec.WorkMountPoint(), os.DirFS(spec.Dir)); err != nil {
			return fmt.Errorf("nested file mount conflict: %w", err)
		}
	}
	return nil
}
