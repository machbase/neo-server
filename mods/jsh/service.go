package jsh

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/machbase/neo-server/v8/mods/util/ssfs"
)

var jshServices = make(map[string]*Service)
var jshServicesLock = sync.RWMutex{}

type Service struct {
	Config *ServiceConfig
	pid    JshPID
}

type ServiceConfig struct {
	Name      string   `json:"-"`
	Enabled   bool     `json:"enabled"`
	StartCmd  string   `json:"start_cmd"`
	StartArgs []string `json:"start_args,omitempty"`

	StopCmd  string   `json:"stop_cmd"`
	StopArgs []string `json:"stop_args,omitempty"`

	ReadError  error `json:"-"`
	StartError error `json:"-"`
	StopError  error `json:"-"`
}

func (svc *Service) String() string {
	b := &strings.Builder{}
	enable := "Disabled"
	if svc.Config.Enabled {
		enable = "ENABLED"
	} else {
		enable = "disabled"
	}
	b.WriteString(fmt.Sprintf("[%s] %s\n", svc.Config.Name, enable))

	b.WriteString(fmt.Sprintf("  start: %s [", svc.Config.StartCmd))
	b.WriteString(fmt.Sprintf(" %v", strings.Join(svc.Config.StartArgs, ", ")))
	b.WriteString("]\n")
	b.WriteString(fmt.Sprintf("  stop: %s [", svc.Config.StopCmd))
	b.WriteString(fmt.Sprintf(" %v", strings.Join(svc.Config.StopArgs, ", ")))
	b.WriteString("]\n")

	return b.String()
}

func (lc *ServiceConfig) Diff(rc *ServiceConfig) bool {
	return lc.Name == rc.Name &&
		lc.Enabled == rc.Enabled &&
		lc.StartCmd == rc.StartCmd &&
		slices.Equal(lc.StartArgs, rc.StartArgs) &&
		lc.StopCmd == rc.StopCmd &&
		slices.Equal(lc.StopArgs, rc.StopArgs)
}

type ServiceList struct {
	NoChanged []*ServiceConfig
	Updated   []*ServiceConfig
	Removed   []*ServiceConfig
	Added     []*ServiceConfig
	Errors    []*ServiceConfig
}

func ReadServices() (ServiceList, error) {
	ret := ServiceList{}
	rootFs := ssfs.Default()
	real, err := rootFs.FindRealPath("/etc/services")
	if err != nil {
		return ret, err
	}

	dirEntries, err := os.ReadDir(real.AbsPath)
	if err != nil {
		return ret, err
	}
	newList := make(map[string]*ServiceConfig)
	for _, ent := range dirEntries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(ent.Name(), ".json")
		var conf = &ServiceConfig{Name: name}
		content, err := os.ReadFile(filepath.Join(real.AbsPath, ent.Name()))
		if err != nil {
			conf.ReadError = err
			ret.Errors = append(ret.Errors, conf)
			continue
		}
		if err := json.Unmarshal(content, conf); err != nil {
			conf.ReadError = err
			ret.Errors = append(ret.Errors, conf)
			continue
		}
		newList[conf.Name] = conf
	}

	jshServicesLock.Lock()
	defer jshServicesLock.Unlock()

	// Check if the new service is different from the existing one
	ret.Updated = make([]*ServiceConfig, 0, len(jshServices))
	ret.Added = make([]*ServiceConfig, 0, len(jshServices))
	ret.Removed = make([]*ServiceConfig, 0, 8)

	for name, newConf := range newList {
		if oldSvc, exists := jshServices[name]; exists {
			if newConf.Diff(oldSvc.Config) {
				// changed
				ret.Updated = append(ret.Updated, newList[name])
			} else {
				// no change
				ret.NoChanged = append(ret.NoChanged, newList[name])
			}
		} else {
			// new service
			ret.Added = append(ret.Added, newList[name])
		}
	}
	for name, oldSvc := range jshServices {
		if _, exists := newList[name]; !exists {
			ret.Removed = append(ret.Removed, oldSvc.Config)
		}
	}
	return ret, nil
}

func (s *ServiceConfig) createJsh(ctx context.Context) *Jsh {
	return NewJsh(
		&JshDaemonContext{ctx},
		WithNativeModules(NativeModuleNames()...),
		WithParent(nil),
		WithReader(nil),
		WithWriter(os.Stdout), // TODO: change to logger
		WithEcho(false),
		WithUserName("sys"),
	)
}

func (result ServiceList) Update(cb func(*ServiceConfig, string, error)) {
	if cb == nil {
		cb = func(s *ServiceConfig, act string, err error) {
			if err != nil {
				fmt.Println("----", s.Name, act, err)
			}
		}
	}
	for _, rm := range result.Removed {
		rm.Stop()
		cb(rm, "REMOVE stop", rm.StopError)
	}

	for _, up := range result.Updated {
		up.Stop()
		cb(up, "UPDATE stop", up.StopError)
		if up.StopError == nil {
			up.Start()
			cb(up, "UPDATE start", up.StartError)
		}
	}
	for _, add := range result.Added {
		add.Start()
		cb(add, "ADD start", add.StartError)
	}
	for _, fl := range result.Errors {
		cb(fl, "CONF", fl.ReadError)
	}
}

func (s *ServiceConfig) Start() {
	jshServicesLock.Lock()
	defer jshServicesLock.Unlock()

	ctx := context.Background()
	j := s.createJsh(ctx)
	s.StartError = j.Exec(append([]string{s.StartCmd}, s.StartArgs...))
	if s.StartError == nil {
		jshServices[s.Name] = &Service{
			Config: s,
		}
	}
}

func (s *ServiceConfig) Stop() {
	jshServicesLock.Lock()
	defer jshServicesLock.Unlock()

	svc, exists := jshServices[s.Name]
	if !exists {
		s.StopError = fmt.Errorf("service %s not found", s.Name)
		return
	}
	if svc.Config.StopCmd == "" {
		if p, ok := jshProcesses[svc.pid]; ok {
			p.Interrupt()
			svc.Config.StopError = nil
		}
	} else {
		ctx := context.Background()
		j := s.createJsh(ctx)
		svc.Config.StopError = j.Exec(append([]string{svc.Config.StopCmd}, svc.Config.StopArgs...))
	}
	delete(jshServices, s.Name)
}
