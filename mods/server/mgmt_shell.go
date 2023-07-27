package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/machbase/neo-grpc/mgmt"
	"github.com/machbase/neo-server/mods/model"
)

func (s *svr) ListShell(context.Context, *mgmt.ListShellRequest) (*mgmt.ListShellResponse, error) {
	tick := time.Now()
	rsp := &mgmt.ListShellResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	lst := s.models.ShellProvider().GetAllShells(false)
	for _, define := range lst {
		rsp.Shells = append(rsp.Shells, &mgmt.ShellDefinition{
			Id:      define.Id,
			Name:    define.Label,
			Command: define.Command,
		})
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

	def := &model.ShellDefinition{}
	if len(req.Name) > 16 {
		rsp.Reason = "name is too long, should be shorter than 16 characters"
		return rsp, nil
	}
	uid, err := uuid.DefaultGenerator.NewV4()
	if err != nil {
		return nil, err
	}
	def.Id = uid.String()
	def.Label = req.Name
	def.Type = model.SHELL_TERM
	def.Attributes = &model.ShellAttributes{Removable: true, Cloneable: true, Editable: true}

	if len(strings.TrimSpace(req.Command)) == 0 {
		rsp.Reason = "command not specified"
		return rsp, nil
	} else {
		def.Command = req.Command
	}

	if err := s.models.ShellProvider().SaveShell(def); err != nil {
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
	if err := s.models.ShellProvider().RemoveShell(req.Id); err != nil {
		rsp.Reason = fmt.Sprintf("fail to remove %s", req.Id)
		return rsp, nil
	}
	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}
