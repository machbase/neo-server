package service

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

const serviceOutputMaxLines = 100

// Service represents a process that running a Engine session.
// A http server javascript process can be an example of this.
type Service struct {
	Config   Config         `json:"config"`
	Status   ServiceStatus  `json:"status"`
	ExitCode int            `json:"exit_code"`
	Error    error          `json:"error"`
	Runtime  ServiceRuntime `json:"runtime"`
	cmd      *exec.Cmd
	startCh  chan struct{}
	stopCh   chan struct{}
}

type ServiceRuntime struct {
	mu      sync.Mutex
	output  []string
	details map[string]any
}

func (s *Service) resetRuntime() {
	s.Runtime.mu.Lock()
	defer s.Runtime.mu.Unlock()
	s.Runtime.output = s.Runtime.output[:0]
	s.Runtime.details = nil
}

func (s *Service) resetOutput() {
	s.Runtime.mu.Lock()
	defer s.Runtime.mu.Unlock()
	s.Runtime.output = s.Runtime.output[:0]
}

func (s *Service) appendOutput(line string) {
	s.Runtime.mu.Lock()
	defer s.Runtime.mu.Unlock()

	s.Runtime.output = append(s.Runtime.output, line)
	if len(s.Runtime.output) <= serviceOutputMaxLines {
		return
	}
	overflow := len(s.Runtime.output) - serviceOutputMaxLines
	copy(s.Runtime.output, s.Runtime.output[overflow:])
	s.Runtime.output = s.Runtime.output[:serviceOutputMaxLines]
}

func (s *Service) outputSnapshot() []string {
	s.Runtime.mu.Lock()
	defer s.Runtime.mu.Unlock()

	out := make([]string, len(s.Runtime.output))
	copy(out, s.Runtime.output)
	return out
}

func (s *Service) detailsSnapshot() map[string]any {
	s.Runtime.mu.Lock()
	defer s.Runtime.mu.Unlock()
	return cloneDetails(s.Runtime.details)
}

func (s *Service) addDetail(key string, value any) error {
	if key == "" {
		return fmt.Errorf("service runtime detail key is required")
	}
	s.Runtime.mu.Lock()
	defer s.Runtime.mu.Unlock()
	if s.Runtime.details == nil {
		s.Runtime.details = map[string]any{}
	}
	if _, exists := s.Runtime.details[key]; exists {
		return fmt.Errorf("service runtime detail %q already exists", key)
	}
	s.Runtime.details[key] = value
	return nil
}

func (s *Service) updateDetail(key string, value any) error {
	if key == "" {
		return fmt.Errorf("service runtime detail key is required")
	}
	s.Runtime.mu.Lock()
	defer s.Runtime.mu.Unlock()
	if _, exists := s.Runtime.details[key]; !exists {
		return fmt.Errorf("service runtime detail %q not found", key)
	}
	s.Runtime.details[key] = value
	return nil
}

func (s *Service) setDetail(key string, value any) error {
	if key == "" {
		return fmt.Errorf("service runtime detail key is required")
	}
	s.Runtime.mu.Lock()
	defer s.Runtime.mu.Unlock()
	if s.Runtime.details == nil {
		s.Runtime.details = map[string]any{}
	}
	s.Runtime.details[key] = value
	return nil
}

func (s *Service) deleteDetail(key string) error {
	if key == "" {
		return fmt.Errorf("service runtime detail key is required")
	}
	s.Runtime.mu.Lock()
	defer s.Runtime.mu.Unlock()
	if _, exists := s.Runtime.details[key]; !exists {
		return fmt.Errorf("service runtime detail %q not found", key)
	}
	delete(s.Runtime.details, key)
	if len(s.Runtime.details) == 0 {
		s.Runtime.details = nil
	}
	return nil
}

func cloneDetails(details map[string]any) map[string]any {
	if len(details) == 0 {
		return nil
	}
	clone := make(map[string]any, len(details))
	for key, value := range details {
		clone[key] = value
	}
	return clone
}

type serviceOutputWriter struct {
	svc     *Service
	mu      sync.Mutex
	pending []byte
}

func newServiceOutputWriter(svc *Service) *serviceOutputWriter {
	return &serviceOutputWriter{svc: svc}
}

func (w *serviceOutputWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.pending = append(w.pending, p...)
	for {
		idx := bytes.IndexByte(w.pending, '\n')
		if idx < 0 {
			break
		}
		line := strings.TrimSuffix(string(w.pending[:idx]), "\r")
		w.svc.appendOutput(line)
		w.pending = w.pending[idx+1:]
	}
	return len(p), nil
}

func (w *serviceOutputWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.pending) == 0 {
		return
	}
	line := strings.TrimSuffix(string(w.pending), "\r")
	w.pending = nil
	w.svc.appendOutput(line)
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
	if s.Status == ServiceStatusRunning && s.cmd != nil && s.cmd.Process != nil {
		b.WriteString(fmt.Sprintf("  pid: %d\n", s.cmd.Process.Pid))
	}
	b.WriteString(fmt.Sprintf("  start: %s [", s.Config.Executable))
	b.WriteString(fmt.Sprintf(" %v", strings.Join(s.Config.Args, ", ")))
	b.WriteString(" ]\n")
	b.WriteString("  output:\n")
	for _, line := range s.outputSnapshot() {
		b.WriteString(fmt.Sprintf("    %s\n", line))
	}
	return b.String()
}

func (lc Config) Equal(rc Config) bool {
	if lc.Name != rc.Name {
		return false
	}
	if lc.Enable != rc.Enable {
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

type Config struct {
	Name        string            `json:"name"`           // Unique name of the service
	Enable      bool              `json:"enable"`         // Whether the service is enabled or not
	WorkingDir  string            `json:"working_dir"`    // The working directory of the service
	Environment map[string]string `json:"environment"`    // Environment variables for the service
	Executable  string            `json:"executable"`     // The executable file for the service
	Args        []string          `json:"args,omitempty"` // Arguments for the executable

	ReadError  error `json:"-"`
	StartError error `json:"-"`
	StopError  error `json:"-"`
}

type ServiceList struct {
	Unchanged []*Config `json:"unchanged"` // Services that are unchanged
	Added     []*Config `json:"added"`     // Services that are added
	Removed   []*Config `json:"removed"`   // Services that are removed
	Updated   []*Config `json:"updated"`   // Services that are updated (changed configuration)
	Errored   []*Config `json:"errored"`   // Services that have errors in their configuration
}

func NewServiceList() *ServiceList {
	return &ServiceList{
		Unchanged: make([]*Config, 0),
		Added:     make([]*Config, 0),
		Removed:   make([]*Config, 0),
		Updated:   make([]*Config, 0),
		Errored:   make([]*Config, 0),
	}
}
