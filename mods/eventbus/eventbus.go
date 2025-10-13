package eventbus

import (
	"time"
)

var Default Bus

func init() {
	Default = New()
}

const (
	EVT_PING      = "ping"      // S -> C, C -> S
	EVT_LOG       = "log"       // S -> C
	EVT_OPEN_FILE = "open_file" // S -> C
	EVT_RPC_REQ   = "rpc_req"   // C -> S
	EVT_RPC_RSP   = "rpc_rsp"   // S -> C
	EVT_MSG       = "msg"       // S -> C, C -> S
)

type Event struct {
	Type     string    `json:"type"`
	Session  string    `json:"session,omitempty"`
	Ping     *Ping     `json:"ping,omitempty"`
	Log      *Log      `json:"log,omitempty"`
	OpenFile *OpenFile `json:"open_file,omitempty"`
	Rpc      *RPC      `json:"rpc,omitempty"`
	Message  *Message  `json:"msg,omitempty"`
}

type Ping struct {
	Tick int64 `json:"tick"`
}

type Log struct {
	Timestamp int64  `json:"timestamp"`
	Level     string `json:"level"`
	Task      string `json:"task,omitempty"`
	Message   string `json:"message"`
	Repeat    int    `json:"repeat,omitempty"`
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
			Timestamp: time.Now().UnixNano(),
			Level:     level,
			Message:   message,
		},
	}
}

func NewLogTask(level string, task string, message string) *Event {
	return &Event{
		Type: EVT_LOG,
		Log: &Log{
			Timestamp: time.Now().UnixNano(),
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

type OpenFile struct {
	Path string `json:"path"`
}

// topic = "console:%s:%s", user, id"
func PublishOpenFile(topic string, file string) {
	Default.Publish(topic, &Event{
		Type: EVT_OPEN_FILE,
		OpenFile: &OpenFile{
			Path: file,
		},
	})
}

type RPC struct {
	Ver    string        `json:"jsonrpc"` // "2.0"
	ID     int64         `json:"id"`
	Method string        `json:"method"` // method name
	Params []interface{} `json:"params,omitempty"`
}

func PublishMessage(topic string, session string, msg *Message) {
	Default.Publish(topic, &Event{
		Type:    EVT_MSG,
		Session: session,
		Message: msg,
	})
}
