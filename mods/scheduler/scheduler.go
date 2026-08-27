package scheduler

import (
	"context"
	"fmt"
	"strings"
	"time"

	logging "github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/robfig/cron/v3"
)

// ScheduleType distinguishes the two kinds of schedule entries. It exists
// only at the scheduler package level: the persistence layer stores timer
// and subscriber definitions in separate tables/structs
// (model.TimerDefinition / model.SubscriberDefinition).
type ScheduleType string

const (
	SCHEDULE_UNDEFINED  ScheduleType = ""
	SCHEDULE_TIMER      ScheduleType = "timer"
	SCHEDULE_SUBSCRIBER ScheduleType = "subscriber"
)

func (typ ScheduleType) String() string {
	switch typ {
	default:
		return "UNDEFINED"
	case SCHEDULE_TIMER:
		return "TIMER"
	case SCHEDULE_SUBSCRIBER:
		return "SUBSCRIBER"
	}
}

func ParseScheduleType(typ string) ScheduleType {
	switch strings.ToUpper(typ) {
	default:
		return SCHEDULE_UNDEFINED
	case "TIMER":
		return SCHEDULE_TIMER
	case "SUBSCRIBER":
		return SCHEDULE_SUBSCRIBER
	}
}

func NewService(opts ...Option) *Service {
	ret := &Service{
		log: logging.GetLog("scheduler"),
	}
	for _, o := range opts {
		o(ret)
	}
	defaultCron := util.DefaultCron()
	if defaultCron == nil {
		defaultCron = cron.New(
			cron.WithLocation(time.Local),
			cron.WithSeconds(),
			cron.WithLogger(ret),
		)
		util.SetDefaultCron(defaultCron)
	}
	ret.crons = defaultCron
	return ret
}

type Service struct {
	log       logging.Log
	crons     *cron.Cron
	tqlLoader tql.Loader
	verbose   bool

	models ScheduleProvider
}

type Option func(*Service)

// TimerProvider is the persistence-layer dependency for timer definitions.
// It is satisfied by *model.Provider.
type TimerProvider interface {
	LoadAllTimers(ctx context.Context) ([]*model.TimerDefinition, error)
	LoadTimer(ctx context.Context, name string) (*model.TimerDefinition, error)
	LoadTimerByID(ctx context.Context, id int64) (*model.TimerDefinition, error)
	SaveTimer(ctx context.Context, def *model.TimerDefinition) error
	RemoveTimer(ctx context.Context, name string) error
	RemoveTimerByID(ctx context.Context, id int64) error
}

// SubscriberProvider is the persistence-layer dependency for subscriber
// definitions. It is satisfied by *model.Provider.
type SubscriberProvider interface {
	LoadAllSubscribers(ctx context.Context) ([]*model.SubscriberDefinition, error)
	LoadSubscriber(ctx context.Context, name string) (*model.SubscriberDefinition, error)
	LoadSubscriberByID(ctx context.Context, id int64) (*model.SubscriberDefinition, error)
	SaveSubscriber(ctx context.Context, def *model.SubscriberDefinition) error
	RemoveSubscriber(ctx context.Context, name string) error
	RemoveSubscriberByID(ctx context.Context, id int64) error
}

type ScheduleProvider interface {
	TimerProvider
	SubscriberProvider
}

func WithProvider(provider ScheduleProvider) Option {
	return func(s *Service) {
		s.models = provider
	}
}

func WithTqlLoader(ldr tql.Loader) Option {
	return func(s *Service) {
		s.tqlLoader = ldr
	}
}

func WithVerbose(flag bool) Option {
	return func(s *Service) {
		s.verbose = flag
	}
}

func (s *Service) Start() error {
	ctx := context.Background()
	timers, err := s.models.LoadAllTimers(ctx)
	if err != nil {
		return err
	}
	for _, define := range timers {
		if err := RegisterTimer(s, define); err == nil {
			s.log.Infof("add schedule %s type=TIMER", define.Name)
		} else {
			s.log.Errorf("fail to add schedule %s type=TIMER, %s", define.Name, err.Error())
		}
	}
	subs, err := s.models.LoadAllSubscribers(ctx)
	if err != nil {
		return err
	}
	for _, define := range subs {
		if err := RegisterSubscriber(s, define); err == nil {
			s.log.Infof("add schedule %s type=SUBSCRIBER", define.Name)
		} else {
			s.log.Errorf("fail to add schedule %s type=SUBSCRIBER, %s", define.Name, err.Error())
		}
	}
	go s.crons.Run()
	s.log.Info("started.")
	return nil
}

func (s *Service) Stop() {
	UnregisterAll()

	ctx := s.crons.Stop()
	<-ctx.Done()
	s.log.Info("closed.")
}

func (s *Service) AddEntry(entry Entry) error {
	if entry.AutoStart() {
		if err := entry.Start(); err != nil {
			return err
		}
	}
	return nil
}

// implements cron.Log
func (s *Service) Info(msg string, keysAndValues ...any) {
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

func (s *Service) Error(err error, msg string, keysAndValues ...any) {
	s.log.Error(append([]any{err.Error(), msg}, keysAndValues...)...)
}
