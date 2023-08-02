package scheduler

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/model"
	"github.com/machbase/neo-server/mods/tql/fx"
	"github.com/robfig/cron/v3"
)

type TimerEntry struct {
	BaseEntry
	TaskTql  string
	Schedule string
	entryId  cron.EntryID
	s        *svr
	log      logging.Log
}

var _ Entry = &TimerEntry{}

func NewTimerEntry(s *svr, def *model.ScheduleDefinition) (*TimerEntry, error) {
	ret := &TimerEntry{
		BaseEntry: BaseEntry{name: def.Name, state: STOP, autoStart: def.AutoStart},
		TaskTql:   def.Task,
		Schedule:  def.Schedule,
		log:       logging.GetLog(fmt.Sprintf("scheduler-%s", strings.ToLower(def.Name))),
		s:         s,
	}

	return ret, nil
}

func (ent *TimerEntry) Start() error {
	ent.state = STARTING
	ent.err = nil

	if len(ent.Schedule) == 0 {
		ent.state = FAILED
		ent.err = fmt.Errorf("invalid configure - missing Schedule")
		return ent.err
	}
	if ent.TaskTql == "" {
		ent.state = FAILED
		ent.err = fmt.Errorf("invalid configure - missing Task")
		return ent.err
	}
	if entryId, err := ent.s.crons.AddFunc(ent.Schedule, ent.doTask); err != nil {
		ent.state = FAILED
		ent.err = err
		return err
	} else {
		ent.entryId = entryId
		ent.state = RUNNING
	}
	return nil
}

func (ent *TimerEntry) Stop() error {
	prevState := ent.state
	ent.state = STOPPING
	defer func() {
		if ent.state != STOP {
			ent.state = prevState
		}
	}()
	ent.s.crons.Remove(ent.entryId)
	ent.state = STOP
	return nil
}

func (ent *TimerEntry) doTask() {
	tick := time.Now()
	ent.log.Info(ent.name, ent.TaskTql, "start")
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
	fxTask := fx.NewTaskContext(context.TODO())
	fxTask.SetParams(nil)
	fxTask.SetDataReader(nil)
	fxTask.SetDataWriter(io.Discard)
	fxTask.SetJsonOutput(true)
	task, err := sc.Parse(fxTask)
	if err != nil {
		ent.err = err
		ent.state = FAILED
		ent.Stop()
		return
	}
	if err := task.Execute(fxTask, ent.s.db); err != nil {
		ent.err = err
		ent.state = FAILED
		ent.Stop()
	}
}
