package subscriber

import (
	"context"

	logging "github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/tql"
)

type Provider interface {
	LoadSubscribers(context.Context, model.UserScope) ([]*model.SubscriberDefinition, error)
	LoadAllSubscribers(context.Context) ([]*model.SubscriberDefinition, error)
	LoadSubscriberForUser(context.Context, model.UserScope, int64) (*model.SubscriberDefinition, error)
	SaveSubscriberForUser(context.Context, model.UserScope, *model.SubscriberDefinition) error
	RemoveSubscriberForUser(context.Context, model.UserScope, int64) error
	SetSubscriberRuntimeError(context.Context, int64, string) error
}

type Service struct {
	log       logging.Log
	models    Provider
	tqlLoader tql.Loader
}

type Option func(*Service)

func WithProvider(provider Provider) Option {
	return func(service *Service) { service.models = provider }
}
func WithTqlLoader(loader tql.Loader) Option {
	return func(service *Service) { service.tqlLoader = loader }
}
func NewService(options ...Option) *Service {
	service := &Service{log: logging.GetLog("trigger-subscriber")}
	for _, option := range options {
		option(service)
	}
	return service
}
func (service *Service) Start() error {
	definitions, err := service.models.LoadAllSubscribers(context.Background())
	if err != nil {
		return err
	}
	for _, definition := range definitions {
		_ = service.Register(definition)
	}
	return nil
}
func (service *Service) Stop() { unregisterAll() }
