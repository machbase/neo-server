package scheduler

import (
	"fmt"
	"strings"
	"time"

	schedrpc "github.com/machbase/neo-grpc/schedule"
	logging "github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/model"
	"github.com/machbase/neo-server/mods/tql"
	spi "github.com/machbase/neo-spi"
	"github.com/robfig/cron/v3"
)

func NewService(opts ...Option) Service {
	ret := &svr{
		log: logging.GetLog("scheduler"),
	}
	for _, o := range opts {
		o(ret)
	}
	ret.crons = cron.New(
		cron.WithLocation(time.Local),
		cron.WithSeconds(),
		cron.WithLogger(ret),
	)
	return ret
}

type Service interface {
	schedrpc.ManagementServer
	Start() error
	Stop()

	AddEntry(Entry) error
}

type svr struct {
	Service
	log       logging.Log
	crons     *cron.Cron
	tqlLoader tql.Loader
	db        spi.Database
	verbose   bool

	models model.ScheduleProvider
}

type Option func(*svr)

func WithProvider(provider model.ScheduleProvider) Option {
	return func(s *svr) {
		s.models = provider
	}
}

func WithTqlLoader(ldr tql.Loader) Option {
	return func(s *svr) {
		s.tqlLoader = ldr
	}
}

func WithDatabase(db spi.Database) Option {
	return func(s *svr) {
		s.db = db
	}
}

func WithVerbose(flag bool) Option {
	return func(s *svr) {
		s.verbose = flag
	}
}

func (s *svr) Start() error {
	lst, err := s.models.LoadAllSchedules()
	if err != nil {
		return err
	}
	for _, define := range lst {
		if err := Register(s, define); err == nil {
			s.log.Infof("add schedule %s type=%s", define.Name, define.Type)
		} else {
			s.log.Errorf("fail to add schedule %s type=%s, %s", define.Name, define.Type, err.Error())
		}
	}
	go s.crons.Run()
	s.log.Info("started.")
	return nil
}

func (s *svr) Stop() {
	UnregisterAll()

	ctx := s.crons.Stop()
	<-ctx.Done()
	s.log.Info("closed.")
}

func (s *svr) AddEntry(entry Entry) error {
	if entry.AutoStart() {
		if err := entry.Start(); err != nil {
			return err
		}
	}
	return nil
}

// implements cron.Log
func (s *svr) Info(msg string, keysAndValues ...any) {
	if !s.verbose {
		return
	}
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
		s.log.Debug(msg)
	} else {
		s.log.Debugf("%s entry[%d] next=%s %s", msg, entryId, next, strings.Join(extra, ","))
	}
}

func (s *svr) Error(err error, msg string, keysAndValues ...any) {
	s.log.Error(append([]any{err.Error(), msg}, keysAndValues...)...)
}
