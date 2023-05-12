package server

import (
	"context"
	"runtime"
	"strings"
	"time"

	"github.com/machbase/booter"
	"github.com/machbase/neo-grpc/mgmt"
	"google.golang.org/grpc/peer"
)

// // mgmt server implements
func (s *svr) Shutdown(ctx context.Context, req *mgmt.ShutdownRequest) (*mgmt.ShutdownResponse, error) {
	tick := time.Now()
	rsp := &mgmt.ShutdownResponse{}
	if runtime.GOOS == "windows" {
		p, _ := peer.FromContext(ctx)
		s.log.Infof("shutdown request from %v", p.Addr)
		if !strings.HasPrefix(p.Addr.String(), "127.0.0.1:") && !strings.HasPrefix(p.Addr.String(), "0:0:0:0:0:0:0:1") {
			rsp.Success, rsp.Reason = false, "remote access not allowed"
			rsp.Elapse = time.Since(tick).String()
			return rsp, nil
		}
	}

	booter.NotifySignal()
	rsp.Success, rsp.Reason = true, "success"
	rsp.Elapse = time.Since(tick).String()
	return rsp, nil
}
