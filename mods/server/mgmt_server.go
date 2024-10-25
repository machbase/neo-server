package server

import (
	"context"
	"net"
	"runtime"
	"strings"
	"time"

	"github.com/machbase/neo-server/api/machsvr"
	"github.com/machbase/neo-server/api/mgmt"
	"github.com/machbase/neo-server/booter"
	"google.golang.org/grpc/peer"
)

// // mgmt server implements
func (s *svr) Shutdown(ctx context.Context, req *mgmt.ShutdownRequest) (*mgmt.ShutdownResponse, error) {
	tick := time.Now()
	rsp := &mgmt.ShutdownResponse{}
	if runtime.GOOS != "windows" {
		p, ok := peer.FromContext(ctx)
		if !ok {
			rsp.Success, rsp.Reason = false, "failed to get peer address from ctx"
			rsp.Elapse = time.Since(tick).String()
			return rsp, nil
		}
		if p.Addr == net.Addr(nil) {
			rsp.Success, rsp.Reason = false, "failed to get peer address"
			rsp.Elapse = time.Since(tick).String()
			return rsp, nil
		}
		isUnixAddr := false
		isTcpLocal := false
		if addr, ok := p.Addr.(*net.TCPAddr); ok {
			if strings.HasPrefix(addr.String(), "127.0.0.1:") {
				isTcpLocal = true
			} else if strings.HasPrefix(addr.String(), "0:0:0:0:0:0:0:1") {
				isTcpLocal = true
			}
		} else if _, ok := p.Addr.(*net.UnixAddr); ok {
			isUnixAddr = true
		}
		s.log.Infof("shutdown request from %v", p.Addr)
		if !isUnixAddr && !isTcpLocal {
			rsp.Success, rsp.Reason = false, "remote shutdown not allowed"
			rsp.Elapse = time.Since(tick).String()
			return rsp, nil
		}
	}

	booter.NotifySignal()
	rsp.Success, rsp.Reason = true, "success"
	rsp.Elapse = time.Since(tick).String()
	return rsp, nil
}

func (s *svr) ServicePorts(ctx context.Context, req *mgmt.ServicePortsRequest) (*mgmt.ServicePortsResponse, error) {
	tick := time.Now()
	rsp := &mgmt.ServicePortsResponse{}

	ret := []*mgmt.Port{}
	ports, err := s.getServicePorts(req.Service)
	if err != nil {
		return nil, err
	}
	for _, p := range ports {
		ret = append(ret, &mgmt.Port{
			Service: p.Service,
			Address: p.Address,
		})
	}

	rsp.Ports = ret
	rsp.Elapse = time.Since(tick).String()
	return rsp, nil
}

func (s *svr) ServerInfo(ctx context.Context, req *mgmt.ServerInfoRequest) (*mgmt.ServerInfoResponse, error) {
	tick := time.Now()
	rsp := &mgmt.ServerInfoResponse{}
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("GetServerInfo panic recover", panic)
		}
		if rsp != nil {
			rsp.Elapse = time.Since(tick).String()
		}
	}()
	if r, err := s.getServerInfo(); err != nil {
		return nil, err
	} else {
		rsp = r
	}

	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil
}

func (s *svr) Sessions(ctx context.Context, req *mgmt.SessionsRequest) (*mgmt.SessionsResponse, error) {
	rsp := &mgmt.SessionsResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("Sessions panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	if req.Statz {
		if statz := machsvr.StatzSnapshot(); statz != nil {
			rsp.Statz = &mgmt.Statz{
				Conns:          statz.Conns,
				ConnsInUse:     statz.ConnsInUse,
				Stmts:          statz.Stmts,
				StmtsInUse:     statz.StmtsInUse,
				Appenders:      statz.Appenders,
				AppendersInUse: statz.AppendersInUse,
				RawConns:       statz.RawConns,
			}
		}
	}
	if req.Sessions {
		sessions := []*mgmt.Session{}
		s.db.ListWatcher(func(st *machsvr.ConnState) bool {
			sessions = append(sessions, &mgmt.Session{
				Id:            st.Id,
				CreTime:       st.CreatedTime.UnixNano(),
				LatestSqlTime: st.LatestTime.UnixNano(),
				LatestSql:     st.LatestSql,
			})
			return true
		})
	}
	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil
}

func (s *svr) KillSession(ctx context.Context, req *mgmt.KillSessionRequest) (*mgmt.KillSessionResponse, error) {
	rsp := &mgmt.KillSessionResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("Sessions kill panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	if err := s.db.KillConnection(req.Id, req.Force); err != nil {
		rsp.Reason = err.Error()
	} else {
		rsp.Success = true
		rsp.Reason = "success"
	}
	return rsp, nil
}
