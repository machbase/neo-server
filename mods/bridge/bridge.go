package bridge

import (
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/model"
	cmap "github.com/orcaman/concurrent-map/v2"
)

func NewService(opts ...Option) *Service {
	s := &Service{
		log:    logging.GetLog("bridge"),
		ctxMap: cmap.New[*rowsWrap](),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

type Option func(*Service)

func WithProvider(provider model.BridgeProvider) Option {
	return func(s *Service) {
		s.models = provider
	}
}

type Service struct {
	log    logging.Log
	ctxMap cmap.ConcurrentMap[string, *rowsWrap]

	models model.BridgeProvider
}

func (s *Service) Start() error {
	lst, err := s.models.LoadAllBridges()
	if err != nil {
		return err
	}
	for _, define := range lst {
		if err := Register(define); err == nil {
			s.log.Infof("add bridge %s type=%s", define.Name, define.Type)
		} else {
			s.log.Errorf("fail to add bridge %s type=%s, %s", define.Name, define.Type, err.Error())
		}
	}
	return nil
}

func (s *Service) Stop() {
	UnregisterAll()
	s.log.Info("closed.")
}
