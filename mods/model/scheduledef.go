package model

import "strings"

type ScheduleType string

const (
	SCHEDULE_UNDEFINED ScheduleType = ""
	SCHEDULE_TIMER     ScheduleType = "timer"
	SCHEDULE_LISTENER  ScheduleType = "listener"
)

func (typ ScheduleType) String() string {
	switch typ {
	default:
		return "UNDEFINED"
	case SCHEDULE_TIMER:
		return "TIMER"
	case SCHEDULE_LISTENER:
		return "LISTENER"
	}
}

func ParseScheduleType(typ string) ScheduleType {
	switch strings.ToUpper(typ) {
	default:
		return SCHEDULE_UNDEFINED
	case "TIMER":
		return SCHEDULE_TIMER
	case "LISTENER":
		return SCHEDULE_LISTENER
	}
}

type ScheduleDefinition struct {
	Name      string       `json:"-"`
	Type      ScheduleType `json:"type"`
	AutoStart bool         `json:"autoStart"`
	Task      string       `json:"task"`

	// timer task
	Schedule string `json:"schedule,omitempty"`
	// listener task
	Bridge string `json:"bridge,omitempty"`
	Topic  string `json:"topic,omitempty"`
	QoS    int    `json:"qos,omitempty"`
}

type ScheduleProvider interface {
	LoadAllSchedules() ([]*ScheduleDefinition, error)
	LoadSchedule(name string) (*ScheduleDefinition, error)
	SaveSchedule(def *ScheduleDefinition) error
	RemoveSchedule(name string) error
}
