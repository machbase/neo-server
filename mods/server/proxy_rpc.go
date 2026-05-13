package server

import (
	"github.com/gin-gonic/gin"
)

func (s *Server) cleanupServiceProxies(serviceName string) {
	if s == nil || s.proxyMgr == nil || serviceName == "" {
		return
	}
	removed, err := s.proxyMgr.Unregister(ProxyUnregisterRequest{Service: serviceName})
	if err != nil {
		if s.log != nil {
			s.log.Warnf("service %s proxy cleanup failed: %v", serviceName, err)
		}
		return
	}
	if len(removed) > 0 && s.log != nil {
		s.log.Infof("service %s proxy cleanup removed %d entries", serviceName, len(removed))
	}
}

func (s *Server) registerProxy(req ProxyRegisterRequest) (ProxyEntrySnapshot, error) {
	if s.proxyMgr == nil {
		s.proxyMgr = NewProxyManager()
	}
	return s.proxyMgr.Register(req)
}

func (s *Server) unregisterProxy(req ProxyUnregisterRequest) ([]ProxyEntrySnapshot, error) {
	if s.proxyMgr == nil {
		s.proxyMgr = NewProxyManager()
	}
	return s.proxyMgr.Unregister(req)
}

func (s *Server) listProxies(service string) ([]ProxyEntrySnapshot, error) {
	if s.proxyMgr == nil {
		return []ProxyEntrySnapshot{}, nil
	}
	return s.proxyMgr.List(service), nil
}

func (s *Server) getProxy(req ProxyGetRequest) (ProxyEntrySnapshot, error) {
	if s.proxyMgr == nil {
		return ProxyEntrySnapshot{}, errProxyNotFound
	}
	return s.proxyMgr.Get(req)
}

func (svr *httpd) handleServiceProxy(ctx *gin.Context) {
	if svr.authServer == nil || svr.authServer.proxyMgr == nil {
		ctx.JSON(404, gin.H{"success": false, "reason": "proxy not registered"})
		return
	}
	svr.authServer.proxyMgr.Handle(ctx, ctx.Param("path"))
}
