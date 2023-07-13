package server

import (
	"context"
	"net"
	"runtime"
	"strings"
	"time"

	"github.com/machbase/neo-grpc/mgmt"
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
