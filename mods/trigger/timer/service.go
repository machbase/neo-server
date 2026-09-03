package timer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/robfig/cron/v3"
)

// Provider is the timer persistence contract. USER_NAME scopes management;
// EXEC_USER remains the identity used when a timer task executes.
type Provider interface {
	LoadTimers(context.Context, model.UserScope) ([]*model.TimerDefinition, error)
	LoadAllTimers(context.Context) ([]*model.TimerDefinition, error)
	LoadTimerForUser(context.Context, model.UserScope, int64) (*model.TimerDefinition, error)
	SaveTimerForUser(context.Context, model.UserScope, *model.TimerDefinition) error
	RemoveTimerForUser(context.Context, model.UserScope, int64) error
	SetTimerRuntimeError(context.Context, int64, string) error
}

type Service struct {
	log       logging.Log
	crons     *cron.Cron
	tqlLoader tql.Loader
	verbose   bool
	models    Provider
}

type Option func(*Service)

func WithProvider(provider Provider) Option {
	return func(service *Service) { service.models = provider }
}
func WithTqlLoader(loader tql.Loader) Option {
	return func(service *Service) { service.tqlLoader = loader }
}
func WithVerbose(verbose bool) Option { return func(service *Service) { service.verbose = verbose } }

func NewService(options ...Option) *Service {
	service := &Service{
		log:   logging.GetLog("trigger-timer"),
		crons: cron.New(cron.WithLocation(time.Local), cron.WithSeconds()),
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (service *Service) Start() error {
	definitions, err := service.models.LoadAllTimers(context.Background())
	if err != nil {
		return err
	}
	for _, definition := range definitions {
		if err := service.Register(definition); err != nil {
			service.log.Errorf("fail to register timer %d, %s", definition.Id, err.Error())
		}
	}
	go service.crons.Run()
	return nil
}

func (service *Service) Stop() {
	unregisterAll()
	ctx := service.crons.Stop()
	<-ctx.Done()
}

func (service *Service) Info(msg string, keysAndValues ...any) {
	if !service.verbose {
		return
	}
	var next time.Time
	entryID := -1
	var extra []string
	for index := 0; index < len(keysAndValues)-1; index += 2 {
		switch keysAndValues[index] {
		case "now":
			continue
		case "next":
			next, _ = keysAndValues[index+1].(time.Time)
		case "entry":
			if id, ok := keysAndValues[index+1].(cron.EntryID); ok {
				entryID = int(id)
			}
		default:
			extra = append(extra, fmt.Sprintf("%s=%v", keysAndValues[index], keysAndValues[index+1]))
		}
	}
	if entryID == -1 {
		service.log.Debug(msg)
		return
	}
	service.log.Debugf("%s entry[%d] next=%s %s", msg, entryID, next, strings.Join(extra, ","))
}

func (service *Service) Error(err error, msg string, keysAndValues ...any) {
	service.log.Error(append([]any{err.Error(), msg}, keysAndValues...)...)
}
