package eventbus

import (
	"time"

	evbus "github.com/asaskevich/EventBus"
)

type EventBus evbus.Bus

var Default EventBus

func init() {
	Default = EventBus(evbus.New())
}

const (
	EVT_PING = "ping"
	EVT_LOG  = "log"
)

type Event struct {
	Type string `json:"type"`
	Ping *Ping  `json:"ping,omitempty"`
	Log  *Log   `json:"log,omitempty"`
}

type Ping struct {
	Tick int64 `json:"tick"`
}

type Log struct {
	Timestmap int64  `json:"timestamp"`
	Level     string `json:"level"`
	Task      string `json:"task,omitempty"`
	Message   string `json:"message"`
}

func NewPingTime(tick time.Time) *Event {
	return NewPing(tick.UnixNano())
}

func NewPing(tick int64) *Event {
	return &Event{
		Type: EVT_PING,
		Ping: &Ping{Tick: tick},
	}
}

func PublishPing(topic string, tick time.Time) {
	Default.Publish(topic, NewPingTime(tick))
}

func NewLog(level string, message string) *Event {
	return &Event{
		Type: EVT_LOG,
		Log: &Log{
			Timestmap: time.Now().UnixNano(),
			Level:     level,
			Message:   message,
		},
	}
}

func NewLogTask(level string, task string, message string) *Event {
	return &Event{
		Type: EVT_LOG,
		Log: &Log{
			Timestmap: time.Now().UnixNano(),
			Level:     level,
			Task:      task,
			Message:   message,
		},
	}
}

func PublishLog(topic string, level string, message string) {
	Default.Publish(topic, NewLog(level, message))
}

func PublishLogTask(topic string, level string, task string, message string) {
	Default.Publish(topic, NewLogTask(level, task, message))
}
