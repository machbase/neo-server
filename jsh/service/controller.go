package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib"
	"github.com/machbase/neo-server/v8/jsh/root"
)

type ControllerConfig struct {
	Launcher  []string
	Mounts    engine.FSTabs
	ConfigDir string
	Address   string
	SharedFS  ControllerSharedFSConfig
}

type ControllerSharedFSConfig struct {
	BackendDir string
	MountPoint string
}

var errServiceMustBeStopped = errors.New("service must be stopped before uninstall")

func NewController(opt *ControllerConfig) (*Controller, error) {
	fs := engine.NewFS()
	if !opt.Mounts.HasMountPoint("/") {
		opt.Mounts = append(opt.Mounts, root.RootFSTab())
	}
	if !opt.Mounts.HasMountPoint("/lib") {
		opt.Mounts = append(opt.Mounts, lib.LibFSTab())
	}
	for _, tab := range opt.Mounts {
		if err := fs.Mount(tab.MountPoint, tab.FS); err != nil {
			return nil, fmt.Errorf("failed to mount %s: %w", tab.MountPoint, err)
		}
	}
	if opt.Address == "" {
		opt.Address = "tcp://127.0.0.1:0"
	}
	ctl := &Controller{
		fs:               fs,
		confDir:          opt.ConfigDir,
		services:         make(map[string]*Service),
		launcher:         opt.Launcher,
		rpcConfigAddr:    opt.Address,
		backendDir:       engine.CleanPath(opt.SharedFS.BackendDir),
		sharedMountPoint: engine.CleanPath(opt.SharedFS.MountPoint),
		sharedFS:         engine.NewVirtualFS(),
		sharedFDs:        map[int]*sharedFileHandle{},
		sharedNextFD:     3,
		jsonRpcHandlers:  map[string]any{},
		llmSessions:      map[string]*llmSession{},
	}
	if ctl.backendDir == "/" {
		ctl.backendDir = ""
	}
	if ctl.sharedMountPoint == "/" {
		ctl.sharedMountPoint = engine.DefaultControllerSharedMount
	}
	if err := ctl.loadSharedFS(); err != nil {
		return nil, err
	}

	return ctl, nil
}

type Controller struct {
	mu               sync.RWMutex
	sharedMu         sync.RWMutex
	llmMu            sync.RWMutex
	jsonRpcMu        sync.RWMutex
	jsonRpcInit      sync.Once
	services         map[string]*Service
	fs               *engine.FS
	confDir          string
	reread           *ServiceList
	launcher         []string
	backendDir       string
	sharedMountPoint string
	sharedFS         *engine.VirtualFS
	sharedFDs        map[int]*sharedFileHandle
	sharedNextFD     int
	rpcConfigAddr    string
	rpcListenAddr    string
	rpcWG            sync.WaitGroup
	rpcLn            net.Listener
	rpcConnSem       chan struct{}
	rpcConnMax       int
	jsonRpcHandlers  map[string]any
	llmSessions      map[string]*llmSession
}

func (ctl *Controller) loadSharedFS() error {
	if ctl.sharedFS == nil {
		ctl.sharedFS = engine.NewVirtualFS()
	}
	if ctl.backendDir == "" {
		return nil
	}
	info, err := ctl.fs.Stat(ctl.backendDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("load shared filesystem: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("load shared filesystem: backend %s is not a directory", ctl.backendDir)
	}
	return ctl.loadSharedDir("/", ctl.backendDir)
}

func (ctl *Controller) loadSharedDir(dstPath string, srcPath string) error {
	if dstPath != "/" {
		if err := ctl.sharedFS.Mkdir(dstPath); err != nil {
			return err
		}
	}
	entries, err := ctl.fs.ReadDir(srcPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "." || name == ".." {
			continue
		}
		nextDst := engine.CleanPath(path.Join(dstPath, name))
		nextSrc := engine.CleanPath(path.Join(srcPath, name))
		if entry.IsDir() {
			if err := ctl.loadSharedDir(nextDst, nextSrc); err != nil {
				return err
			}
			continue
		}
		data, err := ctl.fs.ReadFile(nextSrc)
		if err != nil {
			return err
		}
		if err := ctl.sharedFS.WriteFile(nextDst, data); err != nil {
			return err
		}
	}
	return nil
}

func (ctl *Controller) sharedBackendPath(name string) string {
	if ctl.backendDir == "" {
		return ""
	}
	name = engine.CleanPath(name)
	if name == "/" {
		return ctl.backendDir
	}
	return engine.CleanPath(path.Join(ctl.backendDir, strings.TrimPrefix(name, "/")))
}

func (ctl *Controller) persistSharedMkdir(name string) error {
	if ctl.backendDir == "" {
		return nil
	}
	return ctl.fs.Mkdir(ctl.sharedBackendPath(name))
}

func (ctl *Controller) persistSharedWriteFile(name string, data []byte) error {
	if ctl.backendDir == "" {
		return nil
	}
	parent := path.Dir(ctl.sharedBackendPath(name))
	if parent != "/" {
		if err := ctl.fs.Mkdir(parent); err != nil {
			return err
		}
	}
	return ctl.fs.WriteFile(ctl.sharedBackendPath(name), data)
}

func (ctl *Controller) persistSharedAppendFile(name string, data []byte) error {
	if ctl.backendDir == "" {
		return nil
	}
	parent := path.Dir(ctl.sharedBackendPath(name))
	if parent != "/" {
		if err := ctl.fs.Mkdir(parent); err != nil {
			return err
		}
	}
	return ctl.fs.AppendFile(ctl.sharedBackendPath(name), data)
}

func (ctl *Controller) persistSharedRemove(name string) error {
	if ctl.backendDir == "" {
		return nil
	}
	target := ctl.sharedBackendPath(name)
	info, err := ctl.fs.Stat(target)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if info.IsDir() {
		hostPath, pathErr := ctl.fs.OSPath(target)
		if pathErr == nil {
			return os.RemoveAll(hostPath)
		}
		return ctl.fs.Remove(target)
	}
	return ctl.fs.Remove(target)
}

func (ctl *Controller) persistSharedRename(oldName string, newName string) error {
	if ctl.backendDir == "" {
		return nil
	}
	parent := path.Dir(ctl.sharedBackendPath(newName))
	if parent != "/" {
		if err := ctl.fs.Mkdir(parent); err != nil {
			return err
		}
	}
	return ctl.fs.Rename(ctl.sharedBackendPath(oldName), ctl.sharedBackendPath(newName))
}

func (ctl *Controller) Start(callback func(sc *Config, action string, err error)) error {
	if err := ctl.Read(); err != nil {
		return err
	}
	if err := ctl.startRPC(); err != nil {
		return err
	}
	ctl.Update(callback)
	return nil
}

func (ctl *Controller) Stop(callback func(sc *Config, action string, err error)) {
	ctl.stopRPC()
	ctl.mu.Lock()
	configs := make([]*Config, 0, len(ctl.services))
	for _, svc := range ctl.services {
		configs = append(configs, &svc.Config)
	}
	for _, sc := range configs {
		ctl.stopService(sc)
		if callback != nil {
			callback(sc, "STOP", sc.StopError)
		}
	}
	ctl.mu.Unlock()
}

func (ctl *Controller) Status(filter func(*Service) bool) []*Service {
	ctl.mu.RLock()
	defer ctl.mu.RUnlock()
	result := make([]*Service, 0, len(ctl.services))

	// get keys first to order the output consistently
	keys := make([]string, 0, len(ctl.services))
	for k := range ctl.services {
		keys = append(keys, k)
	}
	// sort keys alphabetically
	slices.Sort(keys)

	for _, name := range keys {
		svc := ctl.services[name]
		if filter == nil || filter(svc) {
			result = append(result, cloneServiceSnapshot(svc))
		}
	}
	return result
}

func (ctl *Controller) StatusOf(name string) *Service {
	ctl.mu.RLock()
	defer ctl.mu.RUnlock()
	if svc, exists := ctl.services[name]; exists {
		return cloneServiceSnapshot(svc)
	}
	return nil
}

func cloneServiceSnapshot(svc *Service) *Service {
	if svc == nil {
		return nil
	}
	clone := &Service{
		Config:   svc.Config,
		Status:   svc.Status,
		ExitCode: svc.ExitCode,
		Error:    svc.Error,
		cmd:      svc.cmd,
	}
	clone.Runtime.output = svc.outputSnapshot()
	clone.Runtime.details = svc.detailsSnapshot()
	return clone
}

func (ctl *Controller) command(sc *Config) *exec.Cmd {
	if len(ctl.launcher) == 0 {
		// unit tests can set launcher to empty
		return nil
	}
	name := ctl.launcher[0]
	args := []string{}
	if len(ctl.launcher) > 1 {
		args = append(args, ctl.launcher[1:]...)
	}
	for k, v := range sc.Environment {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	if ctl.rpcListenAddr != "" {
		args = append(args, "-e", fmt.Sprintf("%s=%s", engine.ControllerAddressEnv, ctl.rpcListenAddr))
		args = append(args, "-e", fmt.Sprintf("%s=%s", engine.ControllerSharedMountEnv, ctl.sharedMountPoint))
		if sc.Name != "" {
			if svc, exists := ctl.services[sc.Name]; exists && svc.sharedClientID != "" {
				args = append(args, "-e", fmt.Sprintf("%s=%s", engine.ControllerClientIDEnv, svc.sharedClientID))
			}
		}
	}
	args = append(args, sc.Executable)
	args = append(args, sc.Args...)
	cmd := exec.Command(name, args...)
	return cmd
}

func (ctl *Controller) startService(sc *Config) {
	svc, exists := ctl.services[sc.Name]
	if !exists {
		sc.StartError = fmt.Errorf("service %s not found", sc.Name)
		return
	}
	ctl.startServiceInstance(svc, sc)
}

func (ctl *Controller) stopService(sc *Config) {
	svc, exists := ctl.services[sc.Name]
	if !exists {
		sc.StopError = fmt.Errorf("service %s not found", sc.Name)
		return
	}
	ctl.stopServiceInstance(svc, sc)
}

func (ctl *Controller) startServiceInstance(svc *Service, sc *Config) {
	sc.StartError = nil
	svc.Config = *sc
	svc.Config.StartError = nil
	svc.sharedClientID = newSharedClientID(sc.Name)
	svc.Status = ServiceStatusRunning
	svc.Error = nil
	svc.ExitCode = 0
	svc.resetRuntime()

	svc.cmd = ctl.command(sc)
	if svc.cmd == nil {
		return
	}
	stdoutWriter := newServiceOutputWriter(svc)
	stderrWriter := newServiceOutputWriter(svc)
	svc.cmd.Stdout = stdoutWriter
	svc.cmd.Stderr = stderrWriter
	if err := svc.cmd.Start(); err != nil {
		sc.StartError = fmt.Errorf("failed to start service: %w", err)
		svc.Config.StartError = sc.StartError
		svc.Status = ServiceStatusFailed
		svc.Error = sc.StartError
		svc.sharedClientID = ""
		return
	}

	svc.startCh = make(chan struct{})
	svc.stopCh = make(chan struct{})
	clientID := svc.sharedClientID
	go func() {
		close(svc.startCh)
		defer close(svc.stopCh)
		err := svc.cmd.Wait()
		stdoutWriter.Flush()
		stderrWriter.Flush()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		}
		ctl.mu.Lock()
		defer ctl.mu.Unlock()
		ctl.cleanupSharedFDsByOwner(clientID)
		svc.ExitCode = exitCode
		svc.Status = ServiceStatusStopped
		svc.sharedClientID = ""
	}()
	<-svc.startCh
}

func (ctl *Controller) stopServiceInstance(svc *Service, sc *Config) {
	sc.StopError = nil
	svc.Config.StopError = nil

	if svc.Status != ServiceStatusRunning && svc.Status != ServiceStatusStarting {
		if svc.cmd == nil {
			svc.Status = ServiceStatusStopped
			ctl.cleanupSharedFDsByOwner(svc.sharedClientID)
			svc.sharedClientID = ""
		}
		return
	}

	if svc.cmd == nil || svc.cmd.Process == nil {
		svc.Status = ServiceStatusStopped
		svc.Error = nil
		ctl.cleanupSharedFDsByOwner(svc.sharedClientID)
		svc.sharedClientID = ""
		return
	}

	svc.Status = ServiceStatusStopping
	if err := svc.cmd.Process.Kill(); err != nil {
		sc.StopError = fmt.Errorf("failed to stop service: %w", err)
		svc.Config.StopError = sc.StopError
		svc.Status = ServiceStatusFailed
		svc.Error = sc.StopError
		return
	}
	<-svc.stopCh
	svc.Error = nil
}

func newSharedClientID(serviceName string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%s-%d", serviceName, time.Now().UnixNano())
	}
	if serviceName == "" {
		return hex.EncodeToString(buf)
	}
	return fmt.Sprintf("%s-%s", serviceName, hex.EncodeToString(buf))
}

func (ctl *Controller) configPath(name string) string {
	return fmt.Sprintf("%s/%s.json", ctl.confDir, name)
}

func (ctl *Controller) Install(sc *Config) error {
	ctl.mu.Lock()
	defer ctl.mu.Unlock()

	if strings.Contains(sc.Name, "/") {
		return fmt.Errorf("service name cannot contain '/'")
	}
	if _, err := ctl.fs.Stat(ctl.configPath(sc.Name)); err == nil {
		return fmt.Errorf("service %s already exists", sc.Name)
	}
	data, err := json.MarshalIndent(sc, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal service config: %w", err)
	}
	if err := ctl.fs.WriteFile(ctl.configPath(sc.Name), data); err != nil {
		return fmt.Errorf("failed to write service config: %w", err)
	}
	if svc, exists := ctl.services[sc.Name]; exists {
		ctl.stopService(&svc.Config)
		delete(ctl.services, sc.Name)
	}
	ctl.services[sc.Name] = &Service{Config: *sc, Status: ServiceStatusStopped}
	if sc.Enable {
		ctl.startService(sc)
	}
	return nil
}

func (ctl *Controller) Uninstall(name string) error {
	ctl.mu.Lock()
	defer ctl.mu.Unlock()
	if _, err := ctl.fs.Stat(ctl.configPath(name)); err != nil {
		return fmt.Errorf("service %s does not exist", name)
	}
	if svc, exists := ctl.services[name]; exists {
		if svc.Status == ServiceStatusRunning || svc.Status == ServiceStatusStarting || svc.Status == ServiceStatusStopping {
			return fmt.Errorf("service %s is running; stop it before uninstall: %w", name, errServiceMustBeStopped)
		}
	}
	if err := ctl.fs.Remove(ctl.configPath(name)); err != nil {
		return fmt.Errorf("failed to remove service config: %w", err)
	}
	if svc, exists := ctl.services[name]; exists {
		ctl.stopService(&svc.Config)
		delete(ctl.services, name)
	}
	return nil
}

func (ctl *Controller) Read() error {
	reread := NewServiceList()
	entries, err := ctl.fs.ReadDir(ctl.confDir)
	if err != nil {
		return err
	}
	newList := make(map[string]*Config)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		sc := &Config{Name: name}
		data, err := ctl.fs.ReadFile(ctl.configPath(name))
		if err != nil {
			sc.ReadError = err
			reread.Errored = append(reread.Errored, sc)
			continue
		}
		sc.ReadError = json.Unmarshal(data, &sc)
		if sc.ReadError != nil {
			reread.Errored = append(reread.Errored, sc)
			continue
		}
		newList[sc.Name] = sc
	}

	ctl.mu.Lock()
	defer ctl.mu.Unlock()

	for name, sc := range newList {
		if existing, exists := ctl.services[name]; exists {
			if sc.Equal(existing.Config) {
				reread.Unchanged = append(reread.Unchanged, sc)
			} else {
				reread.Updated = append(reread.Updated, sc)
			}
		} else {
			reread.Added = append(reread.Added, sc)
		}
	}
	for name, existing := range ctl.services {
		if _, exists := newList[name]; !exists {
			reread.Removed = append(reread.Removed, &existing.Config)
		}
	}
	ctl.reread = reread
	return nil
}

func (ctl *Controller) StartService(name string) (*Service, error) {
	ctl.mu.Lock()
	defer ctl.mu.Unlock()
	svc, exists := ctl.services[name]
	if !exists {
		return nil, fmt.Errorf("service %s not found", name)
	}
	ctl.startServiceInstance(svc, &svc.Config)
	if svc.Config.StartError != nil {
		return svc, svc.Config.StartError
	}
	return svc, nil
}

func (ctl *Controller) StopService(name string) (*Service, error) {
	ctl.mu.Lock()
	defer ctl.mu.Unlock()
	svc, exists := ctl.services[name]
	if !exists {
		return nil, fmt.Errorf("service %s not found", name)
	}
	ctl.stopServiceInstance(svc, &svc.Config)
	if svc.Config.StopError != nil {
		return svc, svc.Config.StopError
	}
	return svc, nil
}

func (ctl *Controller) Update(cb func(*Config, string, error)) {
	ctl.mu.Lock()
	defer ctl.mu.Unlock()
	if cb == nil {
		cb = func(_ *Config, _ string, _ error) {}
	}
	if ctl.reread == nil {
		return
	}
	defer func() { ctl.reread = nil }()

	for _, sc := range ctl.reread.Removed {
		if svc, exists := ctl.services[sc.Name]; exists {
			ctl.stopService(&svc.Config)
			sc.StopError = svc.Config.StopError
			delete(ctl.services, sc.Name)
			cb(sc, "REMOVE stop", sc.StopError)
		}
	}
	for _, sc := range ctl.reread.Updated {
		if svc, exists := ctl.services[sc.Name]; exists {
			svc.Config = *sc
			ctl.stopService(sc)
			cb(sc, "UPDATE stop", sc.StopError)
			if sc.StopError == nil {
				if sc.Enable {
					ctl.startService(sc)
					cb(sc, "UPDATE start", sc.StartError)
				}
			}
		}
	}
	for _, sc := range ctl.reread.Added {
		ctl.services[sc.Name] = &Service{Config: *sc, Status: ServiceStatusStopped}
		if sc.Enable {
			ctl.startService(sc)
			cb(sc, "ADD start", sc.StartError)
		}
	}
	for _, sc := range ctl.reread.Errored {
		cb(sc, "CONF", sc.ReadError)
	}
}

func (ctl *Controller) Reload(cb func(*Config, string, error)) {
	ctl.mu.Lock()
	defer ctl.mu.Unlock()
	if cb == nil {
		cb = func(_ *Config, _ string, _ error) {}
	}
	if ctl.reread == nil {
		return
	}
	defer func() { ctl.reread = nil }()

	stoppedNames := map[string]error{}
	keys := make([]string, 0, len(ctl.services))
	for name := range ctl.services {
		keys = append(keys, name)
	}
	slices.Sort(keys)
	for _, name := range keys {
		svc := ctl.services[name]
		if svc.Status != ServiceStatusRunning && svc.Status != ServiceStatusStarting {
			continue
		}
		ctl.stopService(&svc.Config)
		stoppedNames[name] = svc.Config.StopError
		cb(&svc.Config, "RELOAD stop", svc.Config.StopError)
	}

	for _, sc := range ctl.reread.Removed {
		delete(ctl.services, sc.Name)
	}
	for _, sc := range ctl.reread.Updated {
		if svc, exists := ctl.services[sc.Name]; exists {
			svc.Config = *sc
			svc.Status = ServiceStatusStopped
			svc.Error = nil
			svc.ExitCode = 0
		}
	}
	for _, sc := range ctl.reread.Added {
		ctl.services[sc.Name] = &Service{Config: *sc, Status: ServiceStatusStopped}
	}
	for _, sc := range ctl.reread.Errored {
		cb(sc, "CONF", sc.ReadError)
	}

	keys = keys[:0]
	for name := range ctl.services {
		keys = append(keys, name)
	}
	slices.Sort(keys)
	for _, name := range keys {
		svc := ctl.services[name]
		if !svc.Config.Enable {
			continue
		}
		if stopErr, exists := stoppedNames[name]; exists && stopErr != nil {
			continue
		}
		ctl.startService(&svc.Config)
		cb(&svc.Config, "RELOAD start", svc.Config.StartError)
	}
}
