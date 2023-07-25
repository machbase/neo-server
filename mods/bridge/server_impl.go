package bridge

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	bridgerpc "github.com/machbase/neo-grpc/bridge"
	"github.com/machbase/neo-server/mods/logging"
	cmap "github.com/orcaman/concurrent-map"
)

func NewService(defDir string) Service {
	s := &svr{
		log:     logging.GetLog("bridge"),
		confDir: defDir,
		ctxMap:  cmap.New(),
	}
	return s
}

type Service interface {
	bridgerpc.ManagementServer
	bridgerpc.RuntimeServer

	Start() error
	Stop()
}

type rowsWrap struct {
	id      string
	conn    *sql.Conn
	rows    *sql.Rows
	ctx     context.Context
	release func()

	enlistInfo string
	enlistTime time.Time
}

var contextIdSerial int64

type svr struct {
	Service

	log     logging.Log
	confDir string
	ctxMap  cmap.ConcurrentMap
}

func (s *svr) Start() error {
	s.iterateConfigs(func(define *Define) bool {
		if err := Register(define); err == nil {
			s.log.Infof("add bridge %s type=%s", define.Name, define.Type)
		} else {
			s.log.Errorf("fail to add bridge %s type=%s failed %s", define.Name, define.Type, err.Error())
		}
		return true
	})
	return nil
}

func (s *svr) Stop() {
	UnregisterAll()
	s.log.Info("closed.")
}

func (s *svr) iterateConfigs(cb func(define *Define) bool) error {
	if cb == nil {
		return nil
	}
	entries, err := os.ReadDir(s.confDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") || entry.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(s.confDir, entry.Name()))
		if err != nil {
			s.log.Warnf("connection def file", err.Error())
			continue
		}
		def := &Define{}
		if err = json.Unmarshal(content, def); err != nil {
			s.log.Warnf("connection def format", err.Error())
			continue
		}
		flag := cb(def)
		if !flag {
			break
		}
	}
	return nil
}

func (s *svr) loadConfig(name string) (*Define, error) {
	path := filepath.Join(s.confDir, fmt.Sprintf("%s.json", name))
	content, err := os.ReadFile(path)
	if err != nil {
		s.log.Warnf("connection def file", err.Error())
		return nil, err
	}
	def := &Define{}
	if err := json.Unmarshal(content, def); err != nil {
		s.log.Warnf("connection def format", err.Error())
		return nil, err
	}
	return def, nil
}

func (s *svr) saveConfig(def *Define) error {
	buf, err := json.Marshal(def)
	if err != nil {
		s.log.Warnf("connection def file", err.Error())
		return err
	}

	path := filepath.Join(s.confDir, fmt.Sprintf("%s.json", def.Name))
	return os.WriteFile(path, buf, 00600)
}

func (s *svr) removeConfig(name string) error {
	path := filepath.Join(s.confDir, fmt.Sprintf("%s.json", name))
	return os.Remove(path)
}

// ///////////////////////////
// management service
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
		conn.Close()
	default:
	}
	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil

}
