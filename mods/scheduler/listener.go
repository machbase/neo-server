package scheduler

import (
	"fmt"

	"github.com/machbase/neo-server/mods/bridge"
)

type ListenerEntry struct {
	BaseEntry
	TaskTql string
	Bridge  string
	Topic   string
	QoS     int
}

var _ Entry = &ListenerEntry{}

func (ent *ListenerEntry) Start(s *svr) error {
	ent.state = STARTING
	defer func() {
		if ent.state != RUNNING {
			ent.state = STOP
		}
	}()

	var br bridge.MqttBridge
	if br0, err := bridge.GetBridge(ent.Bridge); err != nil {
		return err
	} else {
		if b, ok := br0.(bridge.MqttBridge); ok {
			br = b
		} else {
			return fmt.Errorf("%s is not a listenable bridge", br0.String())
		}
	}
	if ent.Topic == "" {
		return fmt.Errorf("empty topic is not allowed, listen to %s", br.String())
	}
	if ok, err := br.Subscribe(ent.Topic, byte(ent.QoS), s.listenTask(ent)); err != nil {
		return err
	} else {
		if !ok {
			return fmt.Errorf("fail listening to %s", br.String())
		} else {
			ent.state = RUNNING
		}
	}
	return nil
}

func (s *svr) listenTask(ent *ListenerEntry) func(topic string, payload []byte, msgId int, dup bool, retain bool) {
	return func(topic string, payload []byte, msgId int, dup bool, retain bool) {
	}
}
