package timer

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/robfig/cron/v3"
)

type TimerEntry struct {
	BaseEntry
	TaskTql  string
	Schedule string
	// ExecUser is the schedule's creator, applied as the TQL task's execution
	// identity via task.SetConsole so that spi.Connect/bridge lookups inside
	// the task run under this user's scope instead of always defaulting to sys.
	ExecUser        string
	id              int64
	continueOnError bool
	lifecycleMu     sync.Mutex
	errorMu         sync.Mutex
	generation      uint64
	entryId         cron.EntryID
	s               *Service
	log             logging.Log
}

var _ Entry = (*TimerEntry)(nil)

func NewTimerEntry(s *Service, def *model.TimerDefinition) (*TimerEntry, error) {
	ret := &TimerEntry{
		BaseEntry:       NewBaseEntry(def.Name, STOP, def.AutoStart),
		TaskTql:         def.Task,
		Schedule:        def.Schedule,
		ExecUser:        def.ExecUser,
		id:              def.Id,
		continueOnError: def.ContinueOnError,
		log:             logging.GetLog(fmt.Sprintf("timer-%s", strings.ToLower(def.Name))),
		s:               s,
	}

	return ret, nil
}

func (ent *TimerEntry) Start() error {
	ent.lifecycleMu.Lock()
	defer ent.lifecycleMu.Unlock()
	if ent.Status() == RUNNING {
		return nil
	}
	ent.generation++
	ent.setStateError(STARTING, nil)

	if len(ent.Schedule) == 0 {
		err := fmt.Errorf("invalid configure - missing Schedule")
		ent.setStateError(FAILED, err)
		return err
	}
	if ent.TaskTql == "" {
		err := fmt.Errorf("invalid configure - missing Task")
		ent.setStateError(FAILED, err)
		return err
	}
	if entryId, err := ent.s.crons.AddFunc(ent.Schedule, ent.doTask); err != nil {
		ent.setStateError(FAILED, err)
		return err
	} else {
		ent.entryId = entryId
		ent.setState(RUNNING)
	}
	return nil
}

func (ent *TimerEntry) Stop() error {
	ent.lifecycleMu.Lock()
	defer ent.lifecycleMu.Unlock()
	ent.generation++
	return ent.stopLocked()
}

func (ent *TimerEntry) stopLocked() error {
	prevState := ent.Status()
	ent.setState(STOPPING)
	defer func() {
		if ent.Status() != STOP {
			ent.setState(prevState)
		}
	}()
	ent.s.crons.Remove(ent.entryId)
	ent.setState(STOP)
	return nil
}

func (ent *TimerEntry) doTask() {
	tick := time.Now()
	ent.lifecycleMu.Lock()
	generation := ent.generation
	ent.lifecycleMu.Unlock()
	var taskErr error
	ent.log.Info(ent.name, ent.TaskTql, "start")
	defer func() {
		if taskErr != nil {
			ent.log.Warn(ent.name, ent.TaskTql, ent.Status().String(), taskErr.Error(), "elapsed", time.Since(tick).String())
			ent.recordTaskError(taskErr)
		} else {
			ent.log.Info(ent.name, ent.TaskTql, ent.Status().String(), "elapsed", time.Since(tick).String())
		}
	}()
	sc, err := ent.s.tqlLoader.Load(ent.TaskTql)
	if err != nil {
		taskErr = err
		ent.handleTaskFailure(generation, err)
		return
	}
	task := tql.NewTaskContext(context.TODO())
	task.SetConsole(ent.ExecUser, "", "")
	task.SetParams(nil)
	task.SetInputReader(nil)
	task.SetOutputWriterJson(io.Discard, true)
	if err := task.CompileScript(sc); err != nil {
		taskErr = err
		ent.handleTaskFailure(generation, err)
		return
	}
	if result := task.Execute(); result == nil || result.Err != nil {
		if result == nil {
			taskErr = fmt.Errorf("TQL execution returned no result")
		} else {
			taskErr = result.Err
		}
		ent.handleTaskFailure(generation, taskErr)
	}
}

func (ent *TimerEntry) handleTaskFailure(generation uint64, err error) {
	if ent.continueOnError {
		return
	}
	ent.lifecycleMu.Lock()
	defer ent.lifecycleMu.Unlock()
	if ent.generation != generation || ent.Status() != RUNNING {
		return
	}
	ent.setStateError(FAILED, err)
	ent.generation++
	if stopErr := ent.stopLocked(); stopErr != nil {
		ent.log.Warn("failed to stop timer", ent.name, stopErr)
	}
}

func (ent *TimerEntry) recordTaskError(err error) {
	if ent.s.models == nil {
		return
	}
	ent.errorMu.Lock()
	defer ent.errorMu.Unlock()
	registry.RLock()
	defer registry.RUnlock()
	if registry.entries[ent.id] != ent {
		return
	}
	message := err.Error()
	if len(message) > 1024 {
		message = message[:1024]
		for !utf8.ValidString(message) {
			message = message[:len(message)-1]
		}
	}
	if err := ent.s.models.SetTimerRuntimeError(context.Background(), ent.id, message); err != nil {
		ent.log.Warn("failed to record timer error", ent.name, err)
	}
}
