package bridge

import (
	"context"
	"time"

	bridgerpc "github.com/machbase/neo-grpc/bridge"
)

func (s *svr) ListBridge(context.Context, *bridgerpc.ListBridgeRequest) (*bridgerpc.ListBridgeResponse, error) {
	tick := time.Now()
	rsp := &bridgerpc.ListBridgeResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	err := s.iterateConfigs(func(define *Define) bool {
		rsp.Bridges = append(rsp.Bridges, &bridgerpc.Bridge{
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

func (s *svr) GetBridge(ctx context.Context, req *bridgerpc.GetBridgeRequest) (*bridgerpc.GetBridgeResponse, error) {
	tick := time.Now()
	rsp := &bridgerpc.GetBridgeResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	if define, err := s.loadConfig(req.Name); err != nil {
		rsp.Reason = err.Error()
	} else {
		rsp.Bridge = &bridgerpc.Bridge{
			Name: define.Name,
			Type: string(define.Type),
			Path: define.Path,
		}
		rsp.Success, rsp.Reason = true, "success"
	}
	return rsp, nil
}

func (s *svr) AddBridge(ctx context.Context, req *bridgerpc.AddBridgeRequest) (*bridgerpc.AddBridgeResponse, error) {
	tick := time.Now()
	rsp := &bridgerpc.AddBridgeResponse{Reason: "not specified"}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	def := &Define{}

	if len(req.Name) > 40 {
		rsp.Reason = "name is too long, should be shorter than 40 characters"
		return rsp, nil
	} else {
		def.Name = req.Name
	}

	if t, err := ParseType(req.Type); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	} else {
		def.Type = t
	}

	if len(req.Path) == 0 {
		rsp.Reason = "path is empty, it should be specified"
		return rsp, nil
	} else {
		def.Path = req.Path
	}

	if err := Register(def); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	if err := s.saveConfig(def); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
}

func (s *svr) DelBridge(ctx context.Context, req *bridgerpc.DelBridgeRequest) (*bridgerpc.DelBridgeResponse, error) {
	tick := time.Now()
	rsp := &bridgerpc.DelBridgeResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	if err := s.removeConfig(req.Name); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	Unregister(req.Name)

	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil

}

func (s *svr) TestBridge(ctx context.Context, req *bridgerpc.TestBridgeRequest) (*bridgerpc.TestBridgeResponse, error) {
	tick := time.Now()
	rsp := &bridgerpc.TestBridgeResponse{Reason: "unspecified"}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	br, err := GetBridge(req.Name)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	switch con := br.(type) {
	case SqlBridge:
		conn, err := con.Connect(ctx)
		if err != nil {
			rsp.Reason = err.Error()
			return rsp, nil
		}
		defer conn.Close()
		err = conn.PingContext(ctx)
		if err != nil {
			rsp.Reason = err.Error()
			return rsp, nil
		}
	default:
	}
	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil

}
