package bridge

import (
	bridgerpc "github.com/machbase/neo-server/v8/api/bridge"
	"github.com/machbase/neo-server/v8/api/schedule"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/model"
	cmap "github.com/orcaman/concurrent-map/v2"
)

func NewService(opts ...Option) Service {
	s := &svr{
		log:    logging.GetLog("bridge"),
		ctxMap: cmap.New[*rowsWrap](),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

type Service interface {
	bridgerpc.ManagementServer
	bridgerpc.RuntimeServer

	Start() error
	Stop()
}

type Option func(*svr)

func WithProvider(provider model.BridgeProvider) Option {
	return func(s *svr) {
		s.models = provider
	}
}

func WithScheduleServer(handler schedule.ManagementServer) Option {
	return func(s *svr) {
		s.schedMgmtImpl = handler
	}
}

type svr struct {
	Service

	log    logging.Log
	ctxMap cmap.ConcurrentMap[string, *rowsWrap]

	schedMgmtImpl schedule.ManagementServer
	models        model.BridgeProvider
}

func (s *svr) Start() error {
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

func (s *svr) Stop() {
	UnregisterAll()
	s.log.Info("closed.")
}
