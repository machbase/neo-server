package server

import (
	"context"
	"fmt"
	"time"

	"github.com/machbase/neo-grpc/mgmt"
	"github.com/machbase/neo-server/mods/service/sshd"
)

func (s *svr) ListShell(context.Context, *mgmt.ListShellRequest) (*mgmt.ListShellResponse, error) {
	tick := time.Now()
	rsp := &mgmt.ListShellResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	err := s.IterateShellDefs(func(define *sshd.ShellDefinition) bool {
		rsp.Shells = append(rsp.Shells, &mgmt.ShellDefinition{
			Name: define.Name,
			Args: define.Args,
		})
		return true
	})
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}

func (s *svr) AddShell(ctx context.Context, req *mgmt.AddShellRequest) (*mgmt.AddShellResponse, error) {
	tick := time.Now()
	rsp := &mgmt.AddShellResponse{Reason: "not specified"}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	def := &sshd.ShellDefinition{}

	if len(req.Name) > 40 {
		rsp.Reason = fmt.Sprintf("name is too long, should be shorter than 40 characters")
		return rsp, nil
	} else {
		def.Name = req.Name
	}

	if len(req.Args) == 0 {
		rsp.Reason = fmt.Sprintf("path is too long, should be shorter than 40 characters")
		return rsp, nil
	} else {
		def.Args = req.Args
	}

	if err := s.SetShellDef(def); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}

func (s *svr) DelShell(ctx context.Context, req *mgmt.DelShellRequest) (*mgmt.DelShellResponse, error) {
	tick := time.Now()
	rsp := &mgmt.DelShellResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	if err := s.RemoveShellDef(req.Name); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}
