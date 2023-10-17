package scheduler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/bridge"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/model"
	"github.com/machbase/neo-server/mods/tql"
)

type SubscriberEntry struct {
	BaseEntry
	TaskTql string
	Bridge  string
	Topic   string
	QoS     int

	s   *svr
	log logging.Log

	didOnConnectSubscriber bool
	shouldSubscribe        bool
}

var _ Entry = &SubscriberEntry{}

func NewSubscriberEntry(s *svr, def *model.ScheduleDefinition) (*SubscriberEntry, error) {
	ret := &SubscriberEntry{
		BaseEntry: BaseEntry{name: def.Name, state: STOP, autoStart: def.AutoStart},
		TaskTql:   def.Task,
		Bridge:    def.Bridge,
		Topic:     def.Topic,
		QoS:       def.QoS,
		s:         s,
		log:       logging.GetLog(fmt.Sprintf("subscriber-%s", strings.ToLower(def.Name))),
	}

	return ret, nil
}

func (ent *SubscriberEntry) Start() error {
	ent.state = STARTING
	ent.err = nil
	ent.shouldSubscribe = true

	if ent.didOnConnectSubscriber {
		return nil
	}
	var br bridge.MqttBridge
	if br0, err := bridge.GetBridge(ent.Bridge); err != nil {
		ent.state = FAILED
		ent.err = err
		return err
	} else {
		if b, ok := br0.(bridge.MqttBridge); ok {
			br = b
		} else {
			ent.state = FAILED
			ent.err = fmt.Errorf("%s is not a bridge of subscriber type", br0.String())
			return ent.err
		}
	}
	if ent.Topic == "" {
		ent.state = FAILED
		ent.err = fmt.Errorf("empty topic is not allowed, subscribe to %s", br.String())
		return ent.err
	}

	ent.didOnConnectSubscriber = true
	br.OnConnect(func(bridge any) {
		if !ent.shouldSubscribe {
			return
		}
		if ok, err := br.Subscribe(ent.Topic, byte(ent.QoS), ent.doTask); err != nil {
			ent.state = FAILED
			ent.err = err
		} else {
			if !ok {
				ent.state = FAILED
				ent.err = fmt.Errorf("fail to subscribe %s %s", br.String(), ent.Topic)
			} else {
				ent.state = RUNNING
			}
		}
	})
	br.OnDisconnect(func(bridge any) {
		if ent.shouldSubscribe {
			ent.state = STARTING
		} else {
			ent.state = STOP
		}
	})

	return nil
}

func (ent *SubscriberEntry) Stop() error {
	ent.state = STOPPING
	ent.err = nil
	ent.shouldSubscribe = false

	var br bridge.MqttBridge
	if br0, err := bridge.GetBridge(ent.Bridge); err != nil {
		ent.state = FAILED
		ent.err = err
		return err
	} else {
		if b, ok := br0.(bridge.MqttBridge); ok {
			br = b
		} else {
			ent.state = FAILED
			ent.err = fmt.Errorf("%s is not a bridge of subscriber type", br0.String())
			return ent.err
		}
	}
	if ok, err := br.Unsubscribe(ent.Topic); err != nil {
		ent.state = FAILED
		ent.err = err
		return err
	} else {
		if !ok {
			ent.state = FAILED
			ent.err = fmt.Errorf("fail to unsubscribe %s %s", br.String(), ent.Topic)
			return ent.err
		} else {
			ent.state = STOP
		}
	}
	return nil
}

func (ent *SubscriberEntry) doTask(topic string, payload []byte, msgId int, dup bool, retain bool) {
	tick := time.Now()
	ent.log.Info(ent.name, ent.TaskTql, "topic =", topic, "msgid =", msgId, "dup =", dup, "retain =", retain)
	defer func() {
		if ent.err != nil {
			ent.log.Warn(ent.name, ent.TaskTql, ent.state.String(), ent.err.Error(), "elapsed", time.Since(tick).String())
		} else {
			ent.log.Info(ent.name, ent.TaskTql, ent.state.String(), "elapsed", time.Since(tick).String())
		}
	}()
	sc, err := ent.s.tqlLoader.Load(ent.TaskTql)
	if err != nil {
		ent.err = err
		ent.state = FAILED
		ent.Stop()
		return
	}
	params := map[string][]string{}
	params["TOPIC"] = []string{topic}
	params["MSGID"] = []string{fmt.Sprintf("%d", msgId)}
	params["DUP"] = []string{fmt.Sprintf("%t", dup)}
	params["RETAIN"] = []string{fmt.Sprintf("%t", retain)}
	task := tql.NewTaskContext(context.TODO())
	task.SetDatabase(ent.s.db)
	task.SetInputReader(bytes.NewBuffer(payload))
	task.SetOutputWriterJson(io.Discard, true)
	task.SetParams(params)
	if err := task.CompileScript(sc); err != nil {
		ent.err = err
		ent.state = FAILED
		ent.Stop()
		return
	}
	if result := task.Execute(); result == nil || result.Err != nil {
		ent.err = err
		ent.state = FAILED
		ent.Stop()
	}
}
