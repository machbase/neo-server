package model

import "strings"

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

type ScheduleProvider interface {
	LoadAllSchedules() ([]*ScheduleDefinition, error)
	LoadSchedule(name string) (*ScheduleDefinition, error)
	SaveSchedule(def *ScheduleDefinition) error
	RemoveSchedule(name string) error
	UpdateSchedule(def *ScheduleDefinition) error
}
