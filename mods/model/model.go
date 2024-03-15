package model

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/util"
	"github.com/pkg/errors"
)

func NewService(opts ...Option) Service {
	ret := &svr{
		log: logging.GetLog("scheduler"),
	}
	for _, o := range opts {
		o(ret)
	}
	return ret
}

type Service interface {
	ShellProvider() ShellProvider
	BridgeProvider() BridgeProvider
	ScheduleProvider() ScheduleProvider
	Start() error
	Stop()
}

type Option func(*svr)

var _ Service = &svr{}

type svr struct {
	log       logging.Log
	configDir string

	schedDir  string
	bridgeDir string
	shellDir  string

	experimentMode func() bool
}

func WithConfigDirPath(path string) Option {
	return func(s *svr) {
		s.configDir = path
	}
}

func WithExperimentModeProvider(provider func() bool) Option {
	return func(s *svr) {
		s.experimentMode = provider
	}
}

func (s *svr) Start() error {
	s.bridgeDir = filepath.Join(s.configDir, "bridges")
	if err := s.mkDirIfNotExists(s.bridgeDir, 0755); err != nil {
		return errors.Wrap(err, "bridge defs")
	}
	s.schedDir = filepath.Join(s.configDir, "schedules")
	if err := s.mkDirIfNotExists(s.schedDir, 0755); err != nil {
		return errors.Wrap(err, "schedule defs")
	}
	s.shellDir = filepath.Join(s.configDir, "shell")
	if err := s.mkDirIfNotExists(s.shellDir, 0700); err != nil {
		return errors.Wrap(err, "shell defs")
	}
	return nil
}

func (s *svr) Stop() {
}

func (s *svr) ShellProvider() ShellProvider {
	return s
}

func (s *svr) BridgeProvider() BridgeProvider {
	return s
}

func (s *svr) ScheduleProvider() ScheduleProvider {
	return s
}

func (s *svr) LoadAllSchedules() ([]*ScheduleDefinition, error) {
	ret := []*ScheduleDefinition{}
	err := s.iterateScheduleDefs(func(define *ScheduleDefinition) bool {
		ret = append(ret, define)
		return true
	})
	return ret, err
}

func (s *svr) LoadSchedule(name string) (*ScheduleDefinition, error) {
	name = strings.ToUpper(name)
	path := filepath.Join(s.schedDir, fmt.Sprintf("%s.json", name))
	content, err := os.ReadFile(path)
	if err != nil {
		s.log.Warnf("bridge def file", err.Error())
		return nil, err
	}
	def := &ScheduleDefinition{}
	if err := json.Unmarshal(content, def); err != nil {
		s.log.Warnf("bridge def format", err.Error())
		return nil, err
	}
	def.Name = name
	return def, nil
}

func (s *svr) SaveSchedule(def *ScheduleDefinition) error {
	buf, err := json.MarshalIndent(def, "", "\t")
	if err != nil {
		s.log.Warnf("bridge def file", err.Error())
		return err
	}
	name := strings.ToUpper(def.Name)
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "'", "_")
	name = strings.ReplaceAll(name, "$", "_")
	name = strings.ReplaceAll(name, "*", "_")
	name = strings.ReplaceAll(name, "?", "_")
	path := filepath.Join(s.schedDir, fmt.Sprintf("%s.json", name))
	return os.WriteFile(path, buf, 00600)
}

func (s *svr) RemoveSchedule(name string) error {
	name = strings.ToUpper(name)
	path := filepath.Join(s.schedDir, fmt.Sprintf("%s.json", name))
	return os.Remove(path)
}

func (s *svr) LoadAllBridges() ([]*BridgeDefinition, error) {
	ret := []*BridgeDefinition{}
	err := s.iterateBridgeDefs(func(define *BridgeDefinition) bool {
		ret = append(ret, define)
		return true
	})
	return ret, err
}

func (s *svr) LoadBridge(name string) (*BridgeDefinition, error) {
	path := filepath.Join(s.bridgeDir, fmt.Sprintf("%s.json", name))
	content, err := os.ReadFile(path)
	if err != nil {
		s.log.Warnf("bridge def file", err.Error())
		return nil, err
	}
	def := &BridgeDefinition{}
	if err := json.Unmarshal(content, def); err != nil {
		s.log.Warnf("bridge def format", err.Error())
		return nil, err
	}
	return def, nil
}

func (s *svr) SaveBridge(def *BridgeDefinition) error {
	buf, err := json.MarshalIndent(def, "", "\t")
	if err != nil {
		s.log.Warnf("bridge def file", err.Error())
		return err
	}

	path := filepath.Join(s.bridgeDir, fmt.Sprintf("%s.json", def.Name))
	return os.WriteFile(path, buf, 00600)
}

func (s *svr) RemoveBridge(name string) error {
	path := filepath.Join(s.bridgeDir, fmt.Sprintf("%s.json", name))
	return os.Remove(path)
}

func (s *svr) SetDefaultShellCommand(cmd string) {
	reservedWebShellDef[SHELLID_SHELL].Command = cmd
}

func (s *svr) GetShell(id string) (*ShellDefinition, error) {
	id = strings.ToUpper(id)
	ret := reservedWebShellDef[id]
	if ret != nil {
		return ret, nil
	}
	s.iterateShellDefs(func(sd *ShellDefinition) bool {
		if strings.ToUpper(sd.Id) == id {
			ret = sd
			return false
		}
		return true
	})
	return ret, nil
}

func (s *svr) GetAllShells(includesWebShells bool) []*ShellDefinition {
	var ret []*ShellDefinition
	if includesWebShells {
		ret = append(ret, reservedWebShellDef[SHELLID_SQL])
		ret = append(ret, reservedWebShellDef[SHELLID_TQL])
		ret = append(ret, reservedWebShellDef[SHELLID_TAZ])
		ret = append(ret, reservedWebShellDef[SHELLID_DSH])
		ret = append(ret, reservedWebShellDef[SHELLID_WRK])
		ret = append(ret, reservedWebShellDef[SHELLID_SHELL])
	}
	s.iterateShellDefs(func(def *ShellDefinition) bool {
		ret = append(ret, def)
		return true
	})
	return ret
}

func (s *svr) CopyShell(id string) (*ShellDefinition, error) {
	id = strings.ToUpper(id)
	var ret *ShellDefinition
	if _, ok := reservedWebShellDef[id]; ok {
		ret = &ShellDefinition{}
		ret.Type = SHELL_TERM
		ret.Attributes = &ShellAttributes{Removable: true, Editable: true, Cloneable: true}
		if exename, err := os.Executable(); err != nil {
			ret.Command = fmt.Sprintf(`"%s" shell`, os.Args[0])
		} else {
			ret.Command = fmt.Sprintf(`"%s" shell`, exename)
		}
	} else {
		d, err := s.GetShell(id)
		if err != nil {
			return nil, err
		}
		ret = d.Clone()
	}
	if ret == nil {
		s.log.Warnf("shell def not found '%s'", id)
		return nil, fmt.Errorf("shell definition not found '%s'", id)
	}
	uid, err := uuid.DefaultGenerator.NewV4()
	if err != nil {
		s.log.Warnf("shell def new id, %s", err.Error())
		return nil, err
	}
	ret.Id = uid.String()
	ret.Label = "CUSTOM SHELL"
	if err := s.SaveShell(ret); err != nil {
		s.log.Warnf("shell def not saved", err.Error())
		return nil, err
	}
	return ret, nil
}

func (s *svr) RemoveShell(id string) error {
	path := filepath.Join(s.shellDir, fmt.Sprintf("%s.json", strings.ToUpper(id)))
	return os.Remove(path)
}

// Deprecated
func (s *svr) RenameWebShell(name string, newName string) error {
	oldPath := filepath.Join(s.shellDir, fmt.Sprintf("%s.json", strings.ToUpper(name)))
	newPath := filepath.Join(s.shellDir, fmt.Sprintf("%s.json", strings.ToUpper(newName)))
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("'%s' already exists", newName)
	}
	return os.Rename(oldPath, newPath)
}

func (s *svr) SaveShell(def *ShellDefinition) error {
	id := strings.ToUpper(def.Id)
	for _, n := range reservedShellNames {
		if id == n {
			return fmt.Errorf("'%s' is not allowed for the custom shell name", id)
		}
	}
	if len(def.Command) == 0 {
		return errors.New("invalid command for the custom shell")
	}
	args := util.SplitFields(def.Command, true)
	if len(args) == 0 {
		return errors.New("invalid command for the custom shell")
	}
	binpath := args[0]
	if fi, err := os.Stat(binpath); err != nil {
		return errors.Wrapf(err, "'%s' is not accessible", binpath)
	} else {
		if fi.IsDir() {
			return fmt.Errorf("'%s' is not executable", binpath)
		}
		if runtime.GOOS == "windows" {
			if !strings.HasSuffix(strings.ToLower(binpath), ".exe") && !strings.HasSuffix(strings.ToLower(binpath), ".com") {
				return fmt.Errorf("'%s' is not executable", binpath)
			}
		} else {
			if fi.Mode().Perm()&0111 == 0 {
				return fmt.Errorf("'%s' is not executable", binpath)
			}
		}
	}
	content, err := json.Marshal(def)
	if err != nil {
		return err
	}
	path := filepath.Join(s.shellDir, fmt.Sprintf("%s.json", id))
	return os.WriteFile(path, content, 0600)
}

type OldShellDef struct {
	Args []string `json:"args,omitempty"`
}

func (s *svr) mkDirIfNotExists(path string, mode fs.FileMode) error {
	_, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		if err := os.Mkdir(path, mode); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func (s *svr) iterateShellDefs(cb func(*ShellDefinition) bool) error {
	if cb == nil {
		return nil
	}
	entries, err := os.ReadDir(s.shellDir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") || entry.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(s.shellDir, entry.Name()))
		if err != nil {
			s.log.Errorf("ERR file access, %s", err.Error())
			continue
		}
		def := &ShellDefinition{}
		if err := json.Unmarshal(content, def); err != nil {
			s.log.Warnf("ERR invalid shell conf, %s", err.Error())
			continue
		}
		def.Id = strings.ToUpper(strings.TrimSuffix(entry.Name(), ".json"))
		// compatibility old version
		if def.Type == "" {
			def.Type = SHELL_TERM
			def.Label = def.Id
			old := &OldShellDef{}
			if err := json.Unmarshal(content, old); err == nil && len(old.Args) > 0 {
				def.Command = strings.Join(old.Args, " ")
			}
			if def.Attributes == nil {
				def.Attributes = &ShellAttributes{
					Cloneable: true, Removable: true, Editable: true,
				}
			}
		}
		if def.Icon == "" {
			def.Icon = "console-network-outline"
		}
		if def.Label == "" {
			def.Label = "CUSTOM SHELL"
		}
		shouldContinue := cb(def)
		if !shouldContinue {
			break
		}
	}
	return nil
}

func (s *svr) iterateBridgeDefs(cb func(*BridgeDefinition) bool) error {
	if cb == nil {
		return nil
	}
	entries, err := os.ReadDir(s.bridgeDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") || entry.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(s.bridgeDir, entry.Name()))
		if err != nil {
			s.log.Warnf("bridge def file", err.Error())
			continue
		}
		def := &BridgeDefinition{}
		if err = json.Unmarshal(content, def); err != nil {
			s.log.Warnf("bridge def format", err.Error())
			continue
		}
		flag := cb(def)
		if !flag {
			break
		}
	}
	return nil
}

func (s *svr) iterateScheduleDefs(cb func(*ScheduleDefinition) bool) error {
	if cb == nil {
		return nil
	}
	entries, err := os.ReadDir(s.schedDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") || entry.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(s.schedDir, entry.Name()))
		if err != nil {
			s.log.Warnf("schedule def file", err.Error())
			continue
		}
		def := &ScheduleDefinition{}
		if err = json.Unmarshal(content, def); err != nil {
			s.log.Warnf("schedule def format", err.Error())
			continue
		}
		def.Name = strings.TrimSuffix(entry.Name(), ".json")
		def.Type = ScheduleType(strings.ToLower(string(def.Type)))
		flag := cb(def)
		if !flag {
			break
		}
	}
	return nil
}
