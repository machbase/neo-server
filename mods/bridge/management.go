package bridge

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	bridgerpc "github.com/machbase/neo-server/v8/api/bridge"
	"github.com/machbase/neo-server/v8/api/schedule"
	"github.com/machbase/neo-server/v8/mods/model"
)

func (s *svr) ListBridge(context.Context, *bridgerpc.ListBridgeRequest) (*bridgerpc.ListBridgeResponse, error) {
	tick := time.Now()
	rsp := &bridgerpc.ListBridgeResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	if lst, err := s.models.LoadAllBridges(); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	} else {
		for _, define := range lst {
			rsp.Bridges = append(rsp.Bridges, &bridgerpc.Bridge{
				Name: define.Name,
				Type: string(define.Type),
				Path: define.Path,
			})
		}
		rsp.Success, rsp.Reason = true, "success"
		return rsp, nil
	}
}

func (s *svr) GetBridge(ctx context.Context, req *bridgerpc.GetBridgeRequest) (*bridgerpc.GetBridgeResponse, error) {
	tick := time.Now()
	rsp := &bridgerpc.GetBridgeResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	if define, err := s.models.LoadBridge(req.Name); err != nil {
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

	def := &model.BridgeDefinition{}

	if len(req.Name) > 40 {
		rsp.Reason = "name is too long, should be shorter than 40 characters"
		return rsp, nil
	} else {
		def.Name = req.Name
	}

	if t, err := model.ParseBridgeType(req.Type); err != nil {
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

	if err := s.models.SaveBridge(def); err != nil {
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

	listRsp, err := s.schedMgmtImpl.ListSchedule(ctx, &schedule.ListScheduleRequest{})
	if err != nil {
		return nil, err
	}

	subscribers := make([]string, 0)
	for _, schedule := range listRsp.Schedules {
		if strings.EqualFold(schedule.Bridge, req.Name) {
			subscribers = append(subscribers, schedule.Name)
		}
	}

	if len(subscribers) == 1 {
		return rsp, fmt.Errorf("bridge %q has a subscriber, %s", req.Name, subscribers[0])
	} else if len(subscribers) > 1 {
		return rsp, fmt.Errorf("bridge %q has subscribers, %s", req.Name, strings.Join(subscribers, ","))
	}

	if err := s.models.RemoveBridge(req.Name); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	Unregister(req.Name)

	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil

}

func (s *svr) TestBridge(ctx context.Context, req *bridgerpc.TestBridgeRequest) (*bridgerpc.TestBridgeResponse, error) {
	defer func() {
		if o := recover(); o != nil {
			fmt.Printf("panic %s\n%s", o, debug.Stack())
		}
	}()
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
		rsp.Success, rsp.Reason = true, "success"
		return rsp, nil
	case PythonBridge:
		ver, err := con.Version(ctx)
		if err != nil {
			rsp.Reason = err.Error()
			return rsp, nil
		}
		rsp.Success, rsp.Reason = true, fmt.Sprintf("%s success", ver)
		return rsp, nil
	case *MqttBridge:
		rsp.Success, rsp.Reason = con.TestConnection()
		return rsp, nil
	case *NatsBridge:
		rsp.Success, rsp.Reason = con.TestConnection()
		return rsp, nil
	default:
		rsp.Reason = fmt.Sprintf("bridge '%s' does not support testing", br.Name())
		return rsp, nil
	}
}

func (s *svr) StatsBridge(ctx context.Context, req *bridgerpc.StatsBridgeRequest) (*bridgerpc.StatsBridgeResponse, error) {
	tick := time.Now()
	rsp := &bridgerpc.StatsBridgeResponse{Reason: "unspecified"}

	defer func() {
		if o := recover(); o != nil {
			fmt.Printf("panic %s\n%s", o, debug.Stack())
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	br, err := GetBridge(req.Name)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	switch con := br.(type) {
	case *MqttBridge:
		s := con.Stats()
		rsp.InMsgs = s.InMsgs
		rsp.InBytes = s.InBytes
		rsp.OutMsgs = s.OutMsgs
		rsp.OutBytes = s.OutBytes
		rsp.Appended = s.Appended
		rsp.Inserted = s.Inserted
		rsp.Success, rsp.Reason = true, "success"
		return rsp, nil
	case *NatsBridge:
		s := con.Stats()
		rsp.InMsgs = s.InMsgs
		rsp.InBytes = s.InBytes
		rsp.OutMsgs = s.OutMsgs
		rsp.OutBytes = s.OutBytes
		rsp.Appended = s.Appended
		rsp.Inserted = s.Inserted
		rsp.Success, rsp.Reason = true, "success"
		return rsp, nil
	default:
		rsp.Reason = fmt.Sprintf("bridge '%s' does not support stats", br.Name())
		return rsp, nil
	}
}
