package bridge

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/model"
	cmap "github.com/orcaman/concurrent-map/v2"
)

func NewService(opts ...Option) *Service {
	s := &Service{
		log:    logging.GetLog("bridge"),
		ctxMap: cmap.New[*rowsWrap](),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

type Option func(*Service)

// BridgeProvider is the persistence-layer dependency of the bridge Service.
// It is satisfied by *model.Provider.
type BridgeProvider interface {
	LoadAllBridgesForBootstrap(ctx context.Context) ([]*model.BridgeDefinition, error)
	LoadAllBridges(ctx context.Context, scope model.UserScope) ([]*model.BridgeDefinition, error)
	LoadBridge(ctx context.Context, scope model.UserScope, name string) (*model.BridgeDefinition, error)
	SaveBridge(ctx context.Context, scope model.UserScope, def *model.BridgeDefinition) error
	RemoveBridge(ctx context.Context, scope model.UserScope, name string) error
}

type Service struct {
	log    logging.Log
	ctxMap cmap.ConcurrentMap[string, *rowsWrap]
}

func (s *Service) Start() error {
	ctx := context.Background()
	lst, err := defProvider.LoadAllBridgesForBootstrap(ctx)
	if err != nil {
		return err
	}
	for _, define := range lst {
		if err := RegisterByID(define); err == nil {
			s.log.Infof("add bridge %s type=%s owner=%s", define.Name, define.Type, define.Owner)
		} else {
			s.log.Errorf("fail to add bridge %s type=%s, %s", define.Name, define.Type, err.Error())
		}
	}
	return nil
}

func (s *Service) Stop() {
	UnregisterAll()
	s.log.Info("closed.")
}

type ListBridgeResponse struct {
	Success bool          `json:"success"`
	Reason  string        `json:"reason"`
	Elapse  string        `json:"elapse"`
	Bridges []*BridgeInfo `json:"bridges"`
}

type BridgeInfo struct {
	Id          int64  `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Path        string `json:"path"`
	Owner       string `json:"owner"`
	IsPublic    bool   `json:"isPublic"`
	AllowedUser string `json:"allowedUser,omitempty"`
}

func (s *Service) ListBridge(ctx context.Context, scope model.UserScope) (*ListBridgeResponse, error) {
	tick := time.Now()
	rsp := &ListBridgeResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	lst, err := defProvider.LoadAllBridges(ctx, scope)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	for _, define := range lst {
		rsp.Bridges = append(rsp.Bridges, &BridgeInfo{
			Id:          define.Id,
			Name:        define.Name,
			Type:        string(define.Type),
			Path:        define.Path,
			Owner:       define.Owner,
			IsPublic:    define.IsPublic,
			AllowedUser: define.AllowedUser,
		})
	}
	rsp.Success, rsp.Reason = true, "success"
	return rsp, nil
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

func (s *Service) GetBridge(ctx context.Context, scope model.UserScope, req *GetBridgeRequest) (*GetBridgeResponse, error) {
	tick := time.Now()
	rsp := &GetBridgeResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	if define, err := defProvider.LoadBridge(ctx, scope, req.Name); err != nil {
		rsp.Reason = err.Error()
	} else {
		rsp.Bridge = &BridgeInfo{
			Name:     define.Name,
			Type:     string(define.Type),
			Path:     define.Path,
			Owner:    define.Owner,
			IsPublic: define.IsPublic,
		}
		rsp.Success, rsp.Reason = true, "success"
	}
	return rsp, nil
}

type AddBridgeRequest struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Path        string `json:"path"`
	IsPublic    bool   `json:"isPublic,omitempty"`
	AllowedUser string `json:"allowedUser,omitempty"`
}

type AddBridgeResponse struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
}

func (s *Service) AddBridge(ctx context.Context, scope model.UserScope, req *AddBridgeRequest) (*AddBridgeResponse, error) {
	tick := time.Now()
	rsp := &AddBridgeResponse{Reason: "not specified"}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	def := &model.BridgeDefinition{}

	if len(req.Name) > 40 {
		rsp.Reason = "name is too long, should be shorter than 40 characters"
		return rsp, nil
	}
	def.Name = req.Name

	t, err := model.ParseBridgeType(req.Type)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	def.Type = t

	if len(req.Path) == 0 {
		rsp.Reason = "path is empty, it should be specified"
		return rsp, nil
	}
	def.Path = req.Path
	// least-privilege default: a newly created bridge is private to its
	// owner unless the caller explicitly opts into IsPublic/AllowedUser.
	def.IsPublic = req.IsPublic
	def.AllowedUser = req.AllowedUser

	if err := defProvider.SaveBridge(ctx, scope, def); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	if err := RegisterByID(def); err != nil {
		// the persisted definition is unusable (e.g. connection failed),
		// so roll it back instead of leaving an unreachable bridge.
		_ = defProvider.RemoveBridge(ctx, scope, def.Name)
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

func (s *Service) DelBridge(ctx context.Context, scope model.UserScope, req *DelBridgeRequest) (*DelBridgeResponse, error) {
	tick := time.Now()
	rsp := &DelBridgeResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	def, err := defProvider.LoadBridge(ctx, scope, req.Name)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	if err := defProvider.RemoveBridge(ctx, scope, req.Name); err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	UnregisterByID(def.Id)

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

func (s *Service) TestBridge(ctx context.Context, scope model.UserScope, req *TestBridgeRequest) (*TestBridgeResponse, error) {
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

	br, err := GetBridge(ctx, scope, req.Name)
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
	case ConnectionTestBridge:
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

func (s *Service) StatsBridge(ctx context.Context, scope model.UserScope, req *StatsBridgeRequest) (*StatsBridgeResponse, error) {
	tick := time.Now()
	rsp := &StatsBridgeResponse{Reason: "unspecified"}

	defer func() {
		if o := recover(); o != nil {
			fmt.Printf("panic %s\n%s", o, debug.Stack())
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	br, err := GetBridge(ctx, scope, req.Name)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	switch con := br.(type) {
	case StatsBridge:
		s := con.StatsSnapshot()
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
