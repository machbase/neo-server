package server

import (
	"context"
	"time"

	"github.com/machbase/booter"
	"github.com/machbase/neo-grpc/mgmt"
)

// // mgmt server implements
func (s *svr) Shutdown(context.Context, *mgmt.ShutdownRequest) (*mgmt.ShutdownResponse, error) {
	tick := time.Now()
	rsp := &mgmt.ShutdownResponse{}
	booter.NotifySignal()
	rsp.Success, rsp.Reason = true, "success"
	rsp.Elapse = time.Since(tick).String()
	return rsp, nil
}
