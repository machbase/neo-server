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

	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
)

var jshServices = make(map[string]*Service)
var jshServicesLock = sync.RWMutex{}

type Service struct {
	Config *ServiceConfig
	pid    JshPID
	log    logging.Log
}

type ServiceConfig struct {
	Name      string   `json:"name,omitempty"`
	Enable    bool     `json:"enable"`
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
	if svc.Config.Enable {
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

func (lc *ServiceConfig) Equal(rc *ServiceConfig) bool {
	return lc.Name == rc.Name &&
		lc.Enable == rc.Enable &&
		lc.StartCmd == rc.StartCmd &&
		slices.Equal(lc.StartArgs, rc.StartArgs) &&
		lc.StopCmd == rc.StopCmd &&
		slices.Equal(lc.StopArgs, rc.StopArgs)
}

type ServiceList struct {
	Unchanged []*ServiceConfig `json:"unchanged"`
	Updated   []*ServiceConfig `json:"updated"`
	Removed   []*ServiceConfig `json:"removed"`
	Added     []*ServiceConfig `json:"added"`
	Errors    []*ServiceConfig `json:"errors"`
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
	for name, newConf := range newList {
		if oldSvc, exists := jshServices[name]; exists {
			if newConf.Equal(oldSvc.Config) {
				// no change
				ret.Unchanged = append(ret.Unchanged, newList[name])
			} else {
				// changed
				ret.Updated = append(ret.Updated, newList[name])
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
	log := logging.GetLog(fmt.Sprintf("services.%s", s.Name))
	return NewJsh(
		&JshDaemonContext{ctx},
		WithNativeModules(NativeModuleNames()...),
		WithParent(nil),
		WithReader(nil),
		WithWriter(log),
		WithEcho(false),
		WithUserName("sys"),
	)
}

func (result ServiceList) Update(cb func(*ServiceConfig, string, error)) {
	if cb == nil {
		cb = func(s *ServiceConfig, act string, err error) {}
	}
	for _, rm := range result.Removed {
		rm.Stop()
		jshServicesLock.Lock()
		delete(jshServices, rm.Name)
		jshServicesLock.Unlock()
		cb(rm, "REMOVE stop", rm.StopError)
	}

	for _, up := range result.Updated {
		up.Stop()
		cb(up, "UPDATE stop", up.StopError)
		if up.StopError == nil {
			if up.Enable {
				up.Start()
				cb(up, "UPDATE start", up.StartError)
			}
		}
	}
	for _, add := range result.Added {
		jshServicesLock.Lock()
		jshServices[add.Name] = &Service{Config: add}
		jshServicesLock.Unlock()
		if add.Enable {
			add.Start()
			cb(add, "ADD start", add.StartError)
		}
	}
	for _, fl := range result.Errors {
		cb(fl, "CONF", fl.ReadError)
	}
}

func (s *ServiceConfig) Start() {
	go func() {
		jshServicesLock.RLock()
		defer jshServicesLock.RUnlock()

		ctx := context.Background()
		j := s.createJsh(ctx)
		obj, exists := jshServices[s.Name]
		if !exists {
			s.StopError = fmt.Errorf("service %s not found", s.Name)
			return
		}
		j.onStatusChanged = func(_ *Jsh, status JshStatus) {
			if status == JshStatusRunning {
				obj.pid = j.pid
			} else if status == JshStatusStopped {
				obj.pid = 0
			}
		}
		s.StartError = j.ExecBackground(append([]string{s.StartCmd}, s.StartArgs...))
	}()
}

func (s *ServiceConfig) Stop() {
	jshServicesLock.RLock()
	defer jshServicesLock.RUnlock()

	obj, exists := jshServices[s.Name]
	if !exists {
		s.StopError = fmt.Errorf("service %s not found", s.Name)
		return
	}
	if obj.Config.StopCmd == "" {
		if p, ok := jshProcesses[obj.pid]; ok {
			p.Interrupt()
			obj.Config.StopError = nil
		}
	} else {
		ctx := context.Background()
		j := s.createJsh(ctx)
		obj.Config.StopError = j.Exec(append([]string{obj.Config.StopCmd}, obj.Config.StopArgs...))
	}
}
