package service

import (
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"slices"
	"strings"
	"sync"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib"
	"github.com/machbase/neo-server/v8/jsh/root"
)

type ControllerConfig struct {
	Launcher  []string
	Mounts    engine.FSTabs
	ConfigDir string
	Address   string
}

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

	return &Controller{
		fs:            fs,
		confDir:       opt.ConfigDir,
		services:      make(map[string]*Service),
		launcher:      opt.Launcher,
		rpcConfigAddr: opt.Address,
	}, nil
}

type Controller struct {
	mu            sync.RWMutex
	services      map[string]*Service
	fs            *engine.FS
	confDir       string
	reread        *ServiceList
	launcher      []string
	rpcConfigAddr string
	rpcListenAddr string
	rpcWG         sync.WaitGroup
	rpcLn         net.Listener
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
			result = append(result, svc)
		}
	}
	return result
}

func (ctl *Controller) StatusOf(name string) *Service {
	ctl.mu.RLock()
	defer ctl.mu.RUnlock()
	if svc, exists := ctl.services[name]; exists {
		return svc
	}
	return nil
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
		return
	}

	svc.startCh = make(chan struct{})
	svc.stopCh = make(chan struct{})
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
		svc.ExitCode = exitCode
		svc.Status = ServiceStatusStopped
	}()
	<-svc.startCh
}

func (ctl *Controller) stopServiceInstance(svc *Service, sc *Config) {
	sc.StopError = nil
	svc.Config.StopError = nil

	if svc.Status != ServiceStatusRunning && svc.Status != ServiceStatusStarting {
		if svc.cmd == nil {
			svc.Status = ServiceStatusStopped
		}
		return
	}

	if svc.cmd == nil || svc.cmd.Process == nil {
		svc.Status = ServiceStatusStopped
		svc.Error = nil
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
	ctl.mu.RLock()
	svc, exists := ctl.services[name]
	ctl.mu.RUnlock()
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
	ctl.mu.RLock()
	svc, exists := ctl.services[name]
	ctl.mu.RUnlock()
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
