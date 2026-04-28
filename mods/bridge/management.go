package bridge

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/machbase/neo-server/v8/mods/model"
)

type ListBridgeResponse struct {
	Success bool          `json:"success"`
	Reason  string        `json:"reason"`
	Elapse  string        `json:"elapse"`
	Bridges []*BridgeInfo `json:"bridges"`
}

type BridgeInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
}

func (s *Service) ListBridge(context.Context) (*ListBridgeResponse, error) {
	tick := time.Now()
	rsp := &ListBridgeResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	if lst, err := s.models.LoadAllBridges(); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	} else {
		for _, define := range lst {
			rsp.Bridges = append(rsp.Bridges, &BridgeInfo{
				Name: define.Name,
				Type: string(define.Type),
				Path: define.Path,
			})
		}
		rsp.Success, rsp.Reason = true, "success"
		return rsp, nil
	}
}

type GetBridgeRequest struct {
	Name string `json:"name"`
}

type GetBridgeResponse struct {
	Success bool        `json:"success"`
	Reason  string      `json:"reason"`
	Elapse  string      `json:"elapse"`
	Bridge  *BridgeInfo `json:"bridge"`
}

func (s *Service) GetBridge(ctx context.Context, req *GetBridgeRequest) (*GetBridgeResponse, error) {
	tick := time.Now()
	rsp := &GetBridgeResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	if define, err := s.models.LoadBridge(req.Name); err != nil {
		rsp.Reason = err.Error()
	} else {
		rsp.Bridge = &BridgeInfo{
			Name: define.Name,
			Type: string(define.Type),
			Path: define.Path,
		}
		rsp.Success, rsp.Reason = true, "success"
	}
	return rsp, nil
}

type AddBridgeRequest struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
}

type AddBridgeResponse struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
}

func (s *Service) AddBridge(ctx context.Context, req *AddBridgeRequest) (*AddBridgeResponse, error) {
	tick := time.Now()
	rsp := &AddBridgeResponse{Reason: "not specified"}
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

type DelBridgeRequest struct {
	Name string `json:"name"`
}

type DelBridgeResponse struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
}

func (s *Service) DelBridge(ctx context.Context, req *DelBridgeRequest) (*DelBridgeResponse, error) {
	tick := time.Now()
	rsp := &DelBridgeResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	if err := s.models.RemoveBridge(req.Name); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	Unregister(req.Name)

	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil

}

type TestBridgeRequest struct {
	Name string `json:"name"`
}

type TestBridgeResponse struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
}

func (s *Service) TestBridge(ctx context.Context, req *TestBridgeRequest) (*TestBridgeResponse, error) {
	defer func() {
		if o := recover(); o != nil {
			fmt.Printf("panic %s\n%s", o, debug.Stack())
		}
	}()
	tick := time.Now()
	rsp := &TestBridgeResponse{Reason: "unspecified"}
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

type StatsBridgeRequest struct {
	Name string `json:"name"`
}

type StatsBridgeResponse struct {
	Success  bool   `json:"success"`
	Reason   string `json:"reason"`
	Elapse   string `json:"elapse"`
	InMsgs   uint64 `json:"inMsgs"`
	InBytes  uint64 `json:"inBytes"`
	OutMsgs  uint64 `json:"outMsgs"`
	OutBytes uint64 `json:"outBytes"`
	Inserted uint64 `json:"inserted"`
	Appended uint64 `json:"appended"`
}

func (s *Service) StatsBridge(ctx context.Context, req *StatsBridgeRequest) (*StatsBridgeResponse, error) {
	tick := time.Now()
	rsp := &StatsBridgeResponse{Reason: "unspecified"}

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
