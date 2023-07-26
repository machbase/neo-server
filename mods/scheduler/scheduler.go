package scheduler

import (
	"fmt"
	"strings"
	"time"

	logging "github.com/machbase/neo-server/mods/logging"
	"github.com/robfig/cron/v3"
)

type Service interface {
	Start() error
	Stop()

	AddEntry(*Entry) error
}

func NewService() Service {
	ret := &svc{
		log: logging.GetLog("scheduler"),
	}
	return ret
}

type Entry struct {
	Schedule string
	Work     func()

	entryId cron.EntryID
}

type svc struct {
	log   logging.Log
	crons *cron.Cron
}

func (s *svc) Start() error {
	s.crons = cron.New(
		cron.WithLocation(time.Local),
		cron.WithSeconds(),
		cron.WithLogger(s),
	)
	go s.crons.Run()
	s.log.Info("started.")
	return nil
}

func (s *svc) Stop() {
	s.log.Info("stopping...")
	ctx := s.crons.Stop()
	<-ctx.Done()
	s.log.Info("stop.")
}

func (s *svc) AddEntry(def *Entry) error {
	if len(def.Schedule) == 0 {
		return fmt.Errorf("invalid configure - missing Schedule")
	}
	if def.Work == nil {
		return fmt.Errorf("invalid configure - missing Work")
	}
	entryId, err := s.crons.AddFunc(def.Schedule, def.Work)
	if err != nil {
		return err
	}
	def.entryId = entryId
	return nil
}

// implements cron.Log
func (s *svc) Info(msg string, keysAndValues ...any) {
	var next time.Time
	var entryId int = -1
	var extra []string
	for i := 0; i < len(keysAndValues)-1; i += 2 {
		switch keysAndValues[i] {
		case "now":
			continue
		case "next":
			next = keysAndValues[i+1].(time.Time)
		case "entry":
			if eid, ok := keysAndValues[i+1].(cron.EntryID); ok {
				entryId = int(eid)
			}
		default:
			extra = append(extra, fmt.Sprintf("%s=%v", keysAndValues[i], keysAndValues[i+1]))
		}
	}
	if entryId == -1 {
		s.log.Trace(msg)
	} else {
		s.log.Tracef("%s entry[%d] next=%s %s", msg, entryId, next, strings.Join(extra, ","))
	}
}

func (s *svc) Error(err error, msg string, keysAndValues ...any) {
	s.log.Error(append([]any{err.Error(), msg}, keysAndValues...)...)
}
