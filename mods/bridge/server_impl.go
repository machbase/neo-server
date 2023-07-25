package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	bridgerpc "github.com/machbase/neo-grpc/bridge"
	"github.com/machbase/neo-server/mods/logging"
	spi "github.com/machbase/neo-spi"
	cmap "github.com/orcaman/concurrent-map"
)

func NewService(defDir string) Service {
	s := &svr{
		log:              logging.GetLog("bridge"),
		connectorDefsDir: defDir,
		ctxMap:           cmap.New(),
	}
	s.IterateConnectorDefs(func(define *Define) bool {
		if err := Register(define); err == nil {
			s.log.Infof("add connector %s (%s)", define.Name, define.Type)
		} else {
			s.log.Errorf("fail to add connector %s (%s) failed %s", define.Name, define.Type, err.Error())
		}
		return true
	})
	return s
}

type Service interface {
	bridgerpc.ManagementServer
	bridgerpc.RuntimeServer

	Stop()
}

type rowsWrap struct {
	id      string
	rows    spi.Rows
	release func()

	enlistInfo string
	enlistTime time.Time
}

var contextIdSerial int64

type svr struct {
	Service

	log              logging.Log
	connectorDefsDir string
	ctxMap           cmap.ConcurrentMap
}

func (s *svr) Stop() {
	UnregisterAll()
	s.log.Info("closed.")
}

func (s *svr) IterateConnectorDefs(cb func(define *Define) bool) error {
	if cb == nil {
		return nil
	}
	entries, err := os.ReadDir(s.connectorDefsDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") || entry.IsDir() {
			continue
		}

		content, err := os.ReadFile(filepath.Join(s.connectorDefsDir, entry.Name()))
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

func (s *svr) GetConnectorDef(name string) (*Define, error) {
	path := filepath.Join(s.connectorDefsDir, fmt.Sprintf("%s.json", name))
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

func (s *svr) SetConnectorDef(def *Define) error {
	buf, err := json.Marshal(def)
	if err != nil {
		s.log.Warnf("connection def file", err.Error())
		return err
	}

	path := filepath.Join(s.connectorDefsDir, fmt.Sprintf("%s.json", def.Name))
	return os.WriteFile(path, buf, 00600)
}

func (s *svr) RemoveConnectorDef(name string) error {
	path := filepath.Join(s.connectorDefsDir, fmt.Sprintf("%s.json", name))
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
	err := s.IterateConnectorDefs(func(define *Define) bool {
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
		rsp.Reason = "path is too long, should be shorter than 40 characters"
		return rsp, nil
	} else {
		def.Path = req.Path
	}

	if err := Register(def); err != nil {
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

func (s *svr) GetBridge(context.Context, *bridgerpc.GetBridgeRequest) (*bridgerpc.GetBridgeResponse, error) {
	return nil, nil
}
func (s *svr) DelBridge(ctx context.Context, req *bridgerpc.DelBridgeRequest) (*bridgerpc.DelBridgeResponse, error) {
	tick := time.Now()
	rsp := &bridgerpc.DelBridgeResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	if err := s.RemoveConnectorDef(req.Name); err != nil {
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

	cn, err := GetConnector(req.Name)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	switch con := cn.(type) {
	case SqlConnector:
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

// ////////////////////////////
// runtime service
func (s *svr) Exec(ctx context.Context, req *bridgerpc.ExecRequest) (*bridgerpc.ExecResponse, error) {
	rsp := &bridgerpc.ExecResponse{}
	tick := time.Now()
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	conn, err := GetConnector(req.Name)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	switch c := conn.(type) {
	case SqlConnector:
		db, err := WrapDatabase(c)
		if err != nil {
			rsp.Reason = err.Error()
			return rsp, nil
		}
		rows, err := db.QueryContext(ctx, req.Command)
		if err != nil {
			rsp.Reason = err.Error()
			return rsp, nil
		}

		cols, err := rows.Columns()
		if err != nil {
			rsp.Reason = err.Error()
			return rsp, nil
		}

		rsp.Result = &bridgerpc.Result{}
		for _, c := range cols {
			rsp.Result.Fields = append(rsp.Result.Fields, &bridgerpc.ResultField{
				Name:   c.Name,
				Type:   c.Type,
				Size:   int32(c.Size),
				Length: int32(c.Length),
			})
		}

		if len(cols) > 0 { // Fetchable
			handle := strconv.FormatInt(atomic.AddInt64(&contextIdSerial, 1), 10)
			rsp.Result.Handle = handle
			// TODO leak detector
			s.ctxMap.Set(handle, &rowsWrap{
				id:         handle,
				rows:       rows,
				enlistInfo: fmt.Sprintf("%s: %s", req.Name, req.Command),
				enlistTime: time.Now(),
				release: func() {
					s.ctxMap.RemoveCb(handle, func(key string, v interface{}, exists bool) bool {
						rows.Close()
						return true
					})
				},
			})
		} else {
			rows.Close()
		}
		rsp.Success, rsp.Reason = true, "success"
		return rsp, nil
	case Connector:
		rsp.Reason = fmt.Sprintf("connector '%s' (%s) does not support exec", conn.Name(), conn.Type())
		return rsp, nil
	default:
		rsp.Reason = fmt.Sprintf("connector '%s' (%s) is unknown", conn.Name(), conn.Type())
		return rsp, nil
	}
}
func (s *svr) ResultFetch(ctx context.Context, cr *bridgerpc.Result) (*bridgerpc.ResultFetchResponse, error) {
	rsp := &bridgerpc.ResultFetchResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("ConnectorResultFetch panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()

	rowsWrapVal, exists := s.ctxMap.Get(cr.Handle)
	if !exists {
		rsp.Reason = fmt.Sprintf("handle '%s' not found", cr.Handle)
		return rsp, nil
	}
	rowsWrap, ok := rowsWrapVal.(*rowsWrap)
	if !ok {
		rsp.Reason = fmt.Sprintf("handle '%s' is not valid", cr.Handle)
		return rsp, nil
	}

	if !rowsWrap.rows.Next() {
		rsp.Success = true
		rsp.Reason = "success"
		rsp.HasNoRows = true
		return rsp, nil
	}

	columns, err := rowsWrap.rows.Columns()
	if err != nil {
		rsp.Success = false
		rsp.Reason = err.Error()
		return rsp, nil
	}

	values := columns.MakeBuffer()
	err = rowsWrap.rows.Scan(values...)
	if err != nil {
		rsp.Success = false
		rsp.Reason = err.Error()
		return rsp, nil
	}
	rsp.Values, err = bridgerpc.ConvertToDatum(values...)
	if err != nil {
		rsp.Success = false
		rsp.Reason = err.Error()
		return rsp, nil
	}
	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil
}
func (s *svr) ResultClose(ctx context.Context, cr *bridgerpc.Result) (*bridgerpc.ResultCloseResponse, error) {
	rsp := &bridgerpc.ResultCloseResponse{}
	tick := time.Now()
	defer func() {
		if panic := recover(); panic != nil {
			s.log.Error("ConnectorResultClose panic recover", panic)
		}
		rsp.Elapse = time.Since(tick).String()
	}()
	rowsWrapVal, exists := s.ctxMap.Get(cr.Handle)
	if !exists {
		rsp.Reason = fmt.Sprintf("handle '%s' not found", cr.Handle)
		return rsp, nil
	}
	rowsWrap, ok := rowsWrapVal.(*rowsWrap)
	if !ok {
		rsp.Reason = fmt.Sprintf("handle '%s' is not valid", cr.Handle)
		return rsp, nil
	}
	rowsWrap.release()
	rsp.Success = true
	rsp.Reason = "success"
	return rsp, nil
}
