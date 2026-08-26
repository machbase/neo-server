package model

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ScheduleType string

const (
	SCHEDULE_UNDEFINED  ScheduleType = ""
	SCHEDULE_TIMER      ScheduleType = "timer"
	SCHEDULE_SUBSCRIBER ScheduleType = "subscriber"
)

func (typ ScheduleType) String() string {
	switch typ {
	default:
		return "UNDEFINED"
	case SCHEDULE_TIMER:
		return "TIMER"
	case SCHEDULE_SUBSCRIBER:
		return "SUBSCRIBER"
	}
}

func ParseScheduleType(typ string) ScheduleType {
	switch strings.ToUpper(typ) {
	default:
		return SCHEDULE_UNDEFINED
	case "TIMER":
		return SCHEDULE_TIMER
	case "SUBSCRIBER":
		return SCHEDULE_SUBSCRIBER
	}
}

type ScheduleDefinition struct {
	Name      string       `json:"-"`
	Type      ScheduleType `json:"type"`
	AutoStart bool         `json:"autoStart"`
	Task      string       `json:"task"`

	// timer task
	Schedule string `json:"schedule,omitempty"`
	// subscriber task
	Bridge string `json:"bridge,omitempty"`
	Topic  string `json:"topic,omitempty"`
	// mqtt subscriber only
	QoS int `json:"qos,omitempty"`
	// nats subscriber only
	QueueName  string `json:"queue,omitempty"`
	StreamName string `json:"stream,omitempty"`
}

func (s *Provider) LoadAllSchedules() ([]*ScheduleDefinition, error) {
	ret := []*ScheduleDefinition{}
	err := s.iterateScheduleDefs(func(define *ScheduleDefinition) bool {
		ret = append(ret, define)
		return true
	})
	return ret, err
}

func (s *Provider) LoadSchedule(name string) (*ScheduleDefinition, error) {
	name = strings.ToUpper(name)
	path := filepath.Join(s.schedDir, fmt.Sprintf("%s.json", name))
	content, err := os.ReadFile(path)
	if err != nil {
		s.log.Warn("schedule load def file", err.Error())
		return nil, err
	}
	def := &ScheduleDefinition{}
	if err := json.Unmarshal(content, def); err != nil {
		s.log.Warn("schedule load def format", err.Error())
		return nil, err
	}
	def.Name = name
	return def, nil
}

func (s *Provider) SaveSchedule(def *ScheduleDefinition) error {
	buf, err := json.MarshalIndent(def, "", "\t")
	if err != nil {
		s.log.Warn("schedule save def file", err.Error())
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

func (s *Provider) RemoveSchedule(name string) error {
	name = strings.ToUpper(name)
	path := filepath.Join(s.schedDir, fmt.Sprintf("%s.json", name))
	return os.Remove(path)
}

func (s *Provider) UpdateSchedule(def *ScheduleDefinition) error {
	model, err := s.LoadSchedule(def.Name)
	if err != nil {
		return err
	}
	model.AutoStart = def.AutoStart
	model.Task = def.Task
	model.Schedule = def.Schedule
	return s.SaveSchedule(model)
}

func (s *Provider) iterateScheduleDefs(cb func(*ScheduleDefinition) bool) error {
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
			s.log.Warn("schedule iterate def file", err.Error())
			continue
		}
		def := &ScheduleDefinition{}
		if err = json.Unmarshal(content, def); err != nil {
			s.log.Warn("schedule iterate def format", err.Error())
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
