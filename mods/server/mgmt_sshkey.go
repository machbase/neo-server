package server

import (
	"context"
	"time"

	"github.com/machbase/neo-server/api/mgmt"
)

func (s *svr) ListSshKey(ctx context.Context, req *mgmt.ListSshKeyRequest) (*mgmt.ListSshKeyResponse, error) {
	tick := time.Now()
	rsp := &mgmt.ListSshKeyResponse{Reason: "not-implemented"}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	list, err := s.GetAllAuthorizedSshKeys()
	if err != nil {
		return nil, err
	}
	for _, rec := range list {
		rsp.SshKeys = append(rsp.SshKeys, &mgmt.SshKey{KeyType: rec.KeyType, Fingerprint: rec.Fingerprint, Comment: rec.Comment})
	}
	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}

func (s *svr) AddSshKey(ctx context.Context, req *mgmt.AddSshKeyRequest) (*mgmt.AddSshKeyResponse, error) {
	tick := time.Now()
	rsp := &mgmt.AddSshKeyResponse{Reason: "not-implemented"}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	err := s.AddAuthorizedSshKey(req.KeyType, req.Key, req.Comment)
	if err != nil {
		rsp.Reason = err.Error()
	} else {
		rsp.Success, rsp.Reason = true, "success"
	}
	return rsp, nil
}

func (s *svr) DelSshKey(ctx context.Context, req *mgmt.DelSshKeyRequest) (*mgmt.DelSshKeyResponse, error) {
	tick := time.Now()
	rsp := &mgmt.DelSshKeyResponse{Reason: "not-implemented"}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	err := s.RemoveAuthorizedSshKey(req.Fingerprint)
	if err != nil {
		rsp.Reason = err.Error()
	} else {
		rsp.Success, rsp.Reason = true, "success"
	}
	return rsp, nil
}
