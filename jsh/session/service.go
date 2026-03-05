package session

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/machbase/neo-server/v8/jsh/engine"
)

func StartServices(callback func(sc *ServiceConfig, action string, err error)) error {
	result, err := ReadServices()
	if err != nil {
		return err
	}
	result.Update(callback)
	return nil
}

func StopServices(callback func(sc *ServiceConfig, action string, err error)) {
	jshServiceMu.Lock()
	configs := make([]*ServiceConfig, 0, len(jshServices))
	for _, svc := range jshServices {
		configs = append(configs, &svc.Config)
	}
	jshServiceMu.Unlock()

	for _, sc := range configs {
		sc.Stop()
		if callback != nil {
			callback(sc, "STOP", sc.StopError)
		}
	}
}

// Service represents a process that running a Engine session.
// A http server javascript process can be an example of this.
type Service struct {
	Config   ServiceConfig
	Status   ServiceStatus
	Cmd      *exec.Cmd
	ExitCode int
	Error    error
}

type ServiceStatus string

const (
	ServiceStatusStarting ServiceStatus = "starting"
	ServiceStatusRunning  ServiceStatus = "running"
	ServiceStatusStopping ServiceStatus = "stopping"
	ServiceStatusStopped  ServiceStatus = "stopped"
	ServiceStatusFailed   ServiceStatus = "failed"
)

func (s *Service) String() string {
	b := &strings.Builder{}
	enable := "disabled"
	if s.Config.Enable {
		enable = "ENABLED"
	}
	b.WriteString(fmt.Sprintf("[%s] %s\n", s.Config.Name, enable))
	b.WriteString(fmt.Sprintf("  status: %s\n", s.Status))

	b.WriteString(fmt.Sprintf("  start: %s [", s.Config.Executable))
	b.WriteString(fmt.Sprintf(" %v", strings.Join(s.Config.Args, ", ")))
	b.WriteString(" ]\n")

	return b.String()
}

func (lc ServiceConfig) Equal(rc ServiceConfig) bool {
	if lc.Name != rc.Name {
		return false
	}
	if lc.Enable != rc.Enable {
		return false
	}
	if lc.AutoStart != rc.AutoStart {
		return false
	}
	if lc.AutoRestart != rc.AutoRestart {
		return false
	}
	if lc.WorkingDir != rc.WorkingDir {
		return false
	}
	if lc.Executable != rc.Executable {
		return false
	}
	if len(lc.Args) != len(rc.Args) {
		return false
	}
	for i := range lc.Args {
		if lc.Args[i] != rc.Args[i] {
			return false
		}
	}
	if len(lc.Environment) != len(rc.Environment) {
		return false
	}
	for k, v := range lc.Environment {
		if rv, ok := rc.Environment[k]; !ok || rv != v {
			return false
		}
	}
	return true
}

type ServiceConfig struct {
	Name        string            `json:"name"`           // Unique name of the service
	Enable      bool              `json:"enable"`         // Whether the service is enabled or not
	AutoStart   bool              `json:"auto_start"`     // Start the service automatically when the server starts
	AutoRestart bool              `json:"auto_restart"`   // Restart the service automatically if it exits unexpectedly
	WorkingDir  string            `json:"working_dir"`    // The working directory of the service
	Environment map[string]string `json:"environment"`    // Environment variables for the service
	Executable  string            `json:"executable"`     // The executable file for the service
	Args        []string          `json:"args,omitempty"` // Arguments for the executable

	ReadError  error `json:"-"`
	StartError error `json:"-"`
	StopError  error `json:"-"`
}

func (s *ServiceConfig) Start() {
	jshServiceMu.Lock()
	defer jshServiceMu.Unlock()

	svc, exists := jshServices[s.Name]
	if !exists {
		s.StartError = fmt.Errorf("service %s not found", s.Name)
		return
	}
	_ = svc
}

func (s *ServiceConfig) Stop() {
	jshServiceMu.Lock()
	defer jshServiceMu.Unlock()
	svc, exists := jshServices[s.Name]
	if !exists {
		s.StopError = fmt.Errorf("service %s not found", s.Name)
		return
	}
	_ = svc
}

type ServiceList struct {
	Unchanged []*ServiceConfig `json:"unchanged"` // Services that are unchanged
	Added     []*ServiceConfig `json:"added"`     // Services that are added
	Removed   []*ServiceConfig `json:"removed"`   // Services that are removed
	Updated   []*ServiceConfig `json:"updated"`   // Services that are updated (changed configuration)
	Errored   []*ServiceConfig `json:"errored"`   // Services that have errors in their configuration
}

var jshServices = make(map[string]*Service)
var jshServiceMu sync.Mutex

func ReadServices() (*ServiceList, error) {
	var env *engine.Env
	fs, ok := env.Filesystem().(*engine.FS)
	if !ok {
		return nil, fmt.Errorf("filesystem is not of type *engine.FS")
	}
	entries, err := fs.ReadDir("/etc/services")
	if err != nil {
		return nil, err
	}
	result := &ServiceList{
		Unchanged: make([]*ServiceConfig, 0),
		Added:     make([]*ServiceConfig, 0),
		Removed:   make([]*ServiceConfig, 0),
		Updated:   make([]*ServiceConfig, 0),
		Errored:   make([]*ServiceConfig, 0),
	}
	newList := make(map[string]*ServiceConfig)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		sc := &ServiceConfig{Name: name}
		data, err := fs.ReadFile("/etc/services/" + entry.Name())
		if err != nil {
			sc.ReadError = err
			result.Errored = append(result.Errored, sc)
			continue
		}
		sc.ReadError = json.Unmarshal(data, &sc)
		if sc.ReadError != nil {
			result.Errored = append(result.Errored, sc)
			continue
		}
		newList[sc.Name] = sc
	}

	jshServiceMu.Lock()
	defer jshServiceMu.Unlock()

	for name, sc := range newList {
		if existing, exists := jshServices[name]; exists {
			if sc.Equal(existing.Config) {
				result.Unchanged = append(result.Unchanged, sc)
			} else {
				result.Updated = append(result.Updated, sc)
			}
		} else {
			result.Added = append(result.Added, sc)
		}
	}
	for name, existing := range jshServices {
		if _, exists := newList[name]; !exists {
			result.Removed = append(result.Removed, &existing.Config)
		}
	}
	return result, nil
}

func (result ServiceList) Update(cb func(*ServiceConfig, string, error)) {
	if cb == nil {
		cb = func(_ *ServiceConfig, _ string, _ error) {}
	}
	for _, sc := range result.Removed {
		sc.Stop()
		jshServiceMu.Lock()
		delete(jshServices, sc.Name)
		jshServiceMu.Unlock()
		cb(sc, "REMOVE stop", sc.StopError)
	}
	for _, sc := range result.Updated {
		sc.Stop()
		cb(sc, "UPDATE stop", sc.StopError)
		if sc.StopError == nil {
			if sc.Enable {
				sc.Start()
				cb(sc, "UPDATE start", sc.StartError)
			}
		}
	}
	for _, sc := range result.Added {
		jshServiceMu.Lock()
		jshServices[sc.Name] = &Service{Config: *sc, Status: ServiceStatusStopped}
		jshServiceMu.Unlock()
		if sc.Enable {
			sc.Start()
			cb(sc, "ADD start", sc.StartError)
		}
	}
	for _, sc := range result.Errored {
		cb(sc, "CONF", sc.ReadError)
	}
}
