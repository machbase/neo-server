package server

import (
	"context"
	"fmt"
	"time"

	"github.com/machbase/neo-grpc/mgmt"
	"github.com/machbase/neo-server/mods/connector"
)

func (s *svr) ListConnector(context.Context, *mgmt.ListConnectorRequest) (*mgmt.ListConnectorResponse, error) {
	tick := time.Now()
	rsp := &mgmt.ListConnectorResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	err := s.IterateConnectorDefs(func(define *connector.Define) bool {
		rsp.Connectors = append(rsp.Connectors, &mgmt.Connector{
			Name: define.Name,
			Type: string(define.Type),
			Path: define.Path,
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

func (s *svr) AddConnector(ctx context.Context, req *mgmt.AddConnectorRequest) (*mgmt.AddConnectorResponse, error) {
	tick := time.Now()
	rsp := &mgmt.AddConnectorResponse{Reason: "not specified"}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	def := &connector.Define{}

	if len(req.Name) > 40 {
		rsp.Reason = fmt.Sprintf("name is too long, should be shorter than 40 characters")
		return rsp, nil
	} else {
		def.Name = req.Name
	}

	if t, err := connector.ParseType(req.Type); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	} else {
		def.Type = t
	}

	if len(req.Path) == 0 {
		rsp.Reason = fmt.Sprintf("path is too long, should be shorter than 40 characters")
		return rsp, nil
	} else {
		def.Path = req.Path
	}

	if err := connector.Register(def); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	if err := s.SetConnectorDef(def); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}

func (s *svr) DelConnector(ctx context.Context, req *mgmt.DelConnectorRequest) (*mgmt.DelConnectorResponse, error) {
	tick := time.Now()
	rsp := &mgmt.DelConnectorResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	if err := s.RemoveConnectorDef(req.Name); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	connector.Unregister(req.Name)

	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}

func (s *svr) TestConnector(ctx context.Context, req *mgmt.TestConnectorRequest) (*mgmt.TestConnectorResponse, error) {
	tick := time.Now()
	rsp := &mgmt.TestConnectorResponse{Reason: "unspecified"}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	cn, err := connector.GetConnector(req.Name)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	switch con := cn.(type) {
	case connector.SqlConnector:
		conn, err := con.Connect(ctx)
		if err != nil {
			rsp.Reason = err.Error()
			return rsp, nil
		}
		conn.Close()
	default:
	}
	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil
}
