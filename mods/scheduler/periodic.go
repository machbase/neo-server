package scheduler

import (
	"fmt"

	"github.com/robfig/cron/v3"
)

type PeriodicEntry struct {
	BaseEntry
	TaskTql  string
	Schedule string
	entryId  cron.EntryID
}

var _ Entry = &PeriodicEntry{}

func (ent *PeriodicEntry) Start(s *svr) error {
	ent.state = STARTING
	defer func() {
		if ent.state != RUNNING {
			ent.state = STOP
		}
	}()

	if len(ent.Schedule) == 0 {
		return fmt.Errorf("invalid configure - missing Schedule")
	}
	if ent.TaskTql == "" {
		return fmt.Errorf("invalid configure - missing Work")
	}
	if entryId, err := s.crons.AddFunc(ent.Schedule, s.doTask(ent)); err != nil {
		return err
	} else {
		ent.entryId = entryId
		ent.state = RUNNING
	}
	return nil
}

func (s *svr) doTask(ent *PeriodicEntry) func() {
	return func() {
	}
}
