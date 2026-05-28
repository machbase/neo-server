package scheduler

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/robfig/cron/v3"
)

type TimerEntry struct {
	BaseEntry
	TaskTql  string
	Schedule string
	entryId  cron.EntryID
	s        *Service
	log      logging.Log
}

var _ Entry = (*TimerEntry)(nil)

func NewTimerEntry(s *Service, def *model.ScheduleDefinition) (*TimerEntry, error) {
	ret := &TimerEntry{
		BaseEntry: NewBaseEntry(def.Name, STOP, def.AutoStart),
		TaskTql:   def.Task,
		Schedule:  def.Schedule,
		log:       logging.GetLog(fmt.Sprintf("timer-%s", strings.ToLower(def.Name))),
		s:         s,
	}

	return ret, nil
}

func (ent *TimerEntry) Start() error {
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
	ent.log.Info(ent.name, ent.TaskTql, "start")
	defer func() {
		state, err := ent.statusError()
		if err != nil {
			ent.log.Warn(ent.name, ent.TaskTql, state.String(), err.Error(), "elapsed", time.Since(tick).String())
		} else {
			ent.log.Info(ent.name, ent.TaskTql, state.String(), "elapsed", time.Since(tick).String())
		}
	}()
	sc, err := ent.s.tqlLoader.Load(ent.TaskTql)
	if err != nil {
		ent.setStateError(FAILED, err)
		ent.Stop()
		return
	}
	task := tql.NewTaskContext(context.TODO())
	task.SetParams(nil)
	task.SetInputReader(nil)
	task.SetOutputWriterJson(io.Discard, true)
	if err := task.CompileScript(sc); err != nil {
		ent.setStateError(FAILED, err)
		ent.Stop()
		return
	}
	if result := task.Execute(); result == nil || result.Err != nil {
		if result != nil {
			err = result.Err
		}
		ent.setStateError(FAILED, err)
		ent.Stop()
	}
}
