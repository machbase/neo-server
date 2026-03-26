package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
)

const (
	jsonRPCVersion      = "2.0"
	jsonRPCInvalidReq   = -32600
	jsonRPCMethodMiss   = -32601
	jsonRPCInvalidParam = -32602
	jsonRPCInternal     = -32603
	jsonRPCNotFound     = -32004
)

type ConfigSnapshot struct {
	Name        string            `json:"name"`
	Enable      bool              `json:"enable"`
	AutoStart   bool              `json:"auto_start"`
	WorkingDir  string            `json:"working_dir"`
	Environment map[string]string `json:"environment,omitempty"`
	Executable  string            `json:"executable"`
	Args        []string          `json:"args,omitempty"`
	ReadError   string            `json:"read_error,omitempty"`
	StartError  string            `json:"start_error,omitempty"`
	StopError   string            `json:"stop_error,omitempty"`
}

type ServiceSnapshot struct {
	Config   ConfigSnapshot `json:"config"`
	Status   ServiceStatus  `json:"status"`
	ExitCode int            `json:"exit_code"`
	Error    string         `json:"error,omitempty"`
	PID      int            `json:"pid,omitempty"`
	Output   []string       `json:"output,omitempty"`
}

type ServiceListSnapshot struct {
	Unchanged []ConfigSnapshot `json:"unchanged"`
	Added     []ConfigSnapshot `json:"added"`
	Removed   []ConfigSnapshot `json:"removed"`
	Updated   []ConfigSnapshot `json:"updated"`
	Errored   []ConfigSnapshot `json:"errored"`
}

type ControllerAction struct {
	Name   string `json:"name"`
	Action string `json:"action"`
	Error  string `json:"error,omitempty"`
}

type ControllerUpdateResult struct {
	Actions  []ControllerAction `json:"actions"`
	Services []ServiceSnapshot  `json:"services"`
}

type controllerRPCRequest struct {
	Version string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

type controllerRPCResponse struct {
	Version string              `json:"jsonrpc"`
	Result  any                 `json:"result,omitempty"`
	Error   *controllerRPCError `json:"error,omitempty"`
	ID      json.RawMessage     `json:"id,omitempty"`
}

type controllerRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type serviceNameRequest struct {
	Name string `json:"name"`
}

func (ctl *Controller) Port() int {
	ctl.mu.RLock()
	defer ctl.mu.RUnlock()
	return ctl.rpcPort
}

func (ctl *Controller) Address() string {
	ctl.mu.RLock()
	defer ctl.mu.RUnlock()
	return fmt.Sprintf("tcp://127.0.0.1:%d", ctl.rpcPort)
}

func (ctl *Controller) startRPC() error {
	ctl.mu.Lock()
	defer ctl.mu.Unlock()

	if ctl.rpcLn != nil {
		return nil
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start controller rpc listener: %w", err)
	}
	ctl.rpcLn = ln
	if addr, ok := ln.Addr().(*net.TCPAddr); ok {
		ctl.rpcPort = addr.Port
	}
	ctl.rpcWG.Add(1)
	go ctl.serveRPC(ln)
	return nil
}

func (ctl *Controller) stopRPC() {
	ctl.mu.Lock()
	ln := ctl.rpcLn
	ctl.rpcLn = nil
	ctl.rpcPort = 0
	ctl.mu.Unlock()

	if ln == nil {
		return
	}
	_ = ln.Close()
	ctl.rpcWG.Wait()
}

func (ctl *Controller) serveRPC(ln net.Listener) {
	defer ctl.rpcWG.Done()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				continue
			}
			return
		}
		go ctl.serveRPCConn(conn)
	}
}

func (ctl *Controller) serveRPCConn(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for {
		var req controllerRPCRequest
		if err := decoder.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			_ = encoder.Encode(controllerRPCResponse{
				Version: jsonRPCVersion,
				ID:      json.RawMessage("null"),
				Error:   &controllerRPCError{Code: jsonRPCInvalidReq, Message: err.Error()},
			})
			return
		}

		resp := ctl.handleRPC(req)
		if !req.hasResponse() {
			continue
		}
		if err := encoder.Encode(resp); err != nil {
			return
		}
	}
}

func (ctl *Controller) handleRPC(req controllerRPCRequest) controllerRPCResponse {
	resp := controllerRPCResponse{
		Version: jsonRPCVersion,
		ID:      req.responseID(),
	}
	if req.Version != "" && req.Version != jsonRPCVersion {
		resp.Error = &controllerRPCError{Code: jsonRPCInvalidReq, Message: "unsupported jsonrpc version"}
		return resp
	}
	if req.Method == "" {
		resp.Error = &controllerRPCError{Code: jsonRPCInvalidReq, Message: "method is required"}
		return resp
	}

	result, rpcErr := ctl.dispatchRPC(req.Method, req.Params)
	if rpcErr != nil {
		resp.Error = rpcErr
		return resp
	}
	resp.Result = result
	return resp
}

func (ctl *Controller) dispatchRPC(method string, params json.RawMessage) (any, *controllerRPCError) {
	switch method {
	case "service.list":
		return ctl.statusSnapshots(), nil
	case "service.get":
		var req serviceNameRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		svc := ctl.StatusOf(req.Name)
		if svc == nil {
			return nil, &controllerRPCError{Code: jsonRPCNotFound, Message: fmt.Sprintf("service %s not found", req.Name)}
		}
		return snapshotService(svc), nil
	case "service.read":
		if err := ctl.Read(); err != nil {
			return nil, internalRPCError(err)
		}
		return ctl.rereadSnapshot(), nil
	case "service.update":
		return ctl.updateSnapshot(), nil
	case "service.reload":
		if err := ctl.Read(); err != nil {
			return nil, internalRPCError(err)
		}
		return ctl.updateSnapshot(), nil
	case "service.install":
		var sc Config
		if err := decodeRPCParams(params, &sc); err != nil {
			return nil, invalidParamsError(err)
		}
		if err := ctl.Install(&sc); err != nil {
			return nil, internalRPCError(err)
		}
		svc := ctl.StatusOf(sc.Name)
		if svc == nil {
			return nil, &controllerRPCError{Code: jsonRPCInternal, Message: fmt.Sprintf("service %s missing after install", sc.Name)}
		}
		return snapshotService(svc), nil
	case "service.uninstall":
		var req serviceNameRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		if err := ctl.Uninstall(req.Name); err != nil {
			return nil, internalRPCError(err)
		}
		return true, nil
	case "service.start":
		var req serviceNameRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		svc, err := ctl.StartService(req.Name)
		if err != nil {
			if svc == nil {
				return nil, &controllerRPCError{Code: jsonRPCNotFound, Message: err.Error()}
			}
			return nil, internalRPCError(err)
		}
		return snapshotService(svc), nil
	case "service.stop":
		var req serviceNameRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		svc, err := ctl.StopService(req.Name)
		if err != nil {
			if svc == nil {
				return nil, &controllerRPCError{Code: jsonRPCNotFound, Message: err.Error()}
			}
			return nil, internalRPCError(err)
		}
		return snapshotService(svc), nil
	default:
		return nil, &controllerRPCError{Code: jsonRPCMethodMiss, Message: fmt.Sprintf("method %s not found", method)}
	}
}

func (ctl *Controller) rereadSnapshot() ServiceListSnapshot {
	ctl.mu.RLock()
	defer ctl.mu.RUnlock()

	if ctl.reread == nil {
		return ServiceListSnapshot{
			Unchanged: []ConfigSnapshot{},
			Added:     []ConfigSnapshot{},
			Removed:   []ConfigSnapshot{},
			Updated:   []ConfigSnapshot{},
			Errored:   []ConfigSnapshot{},
		}
	}
	return snapshotServiceList(ctl.reread)
}

func (ctl *Controller) updateSnapshot() ControllerUpdateResult {
	result := ControllerUpdateResult{Actions: []ControllerAction{}}
	ctl.Update(func(sc *Config, action string, err error) {
		item := ControllerAction{Name: sc.Name, Action: action}
		if err != nil {
			item.Error = err.Error()
		}
		result.Actions = append(result.Actions, item)
	})
	result.Services = ctl.statusSnapshots()
	return result
}

func (ctl *Controller) statusSnapshots() []ServiceSnapshot {
	services := ctl.Status(nil)
	result := make([]ServiceSnapshot, 0, len(services))
	for _, svc := range services {
		result = append(result, snapshotService(svc))
	}
	return result
}

func snapshotServiceList(list *ServiceList) ServiceListSnapshot {
	return ServiceListSnapshot{
		Unchanged: snapshotConfigs(list.Unchanged),
		Added:     snapshotConfigs(list.Added),
		Removed:   snapshotConfigs(list.Removed),
		Updated:   snapshotConfigs(list.Updated),
		Errored:   snapshotConfigs(list.Errored),
	}
}

func snapshotConfigs(configs []*Config) []ConfigSnapshot {
	result := make([]ConfigSnapshot, 0, len(configs))
	for _, sc := range configs {
		result = append(result, snapshotConfig(sc))
	}
	return result
}

func snapshotConfig(sc *Config) ConfigSnapshot {
	result := ConfigSnapshot{
		Name:       sc.Name,
		Enable:     sc.Enable,
		AutoStart:  sc.AutoStart,
		WorkingDir: sc.WorkingDir,
		Executable: sc.Executable,
	}
	if len(sc.Environment) > 0 {
		result.Environment = make(map[string]string, len(sc.Environment))
		for key, value := range sc.Environment {
			result.Environment[key] = value
		}
	}
	if len(sc.Args) > 0 {
		result.Args = append([]string{}, sc.Args...)
	}
	if sc.ReadError != nil {
		result.ReadError = sc.ReadError.Error()
	}
	if sc.StartError != nil {
		result.StartError = sc.StartError.Error()
	}
	if sc.StopError != nil {
		result.StopError = sc.StopError.Error()
	}
	return result
}

func snapshotService(svc *Service) ServiceSnapshot {
	result := ServiceSnapshot{
		Config:   snapshotConfig(&svc.Config),
		Status:   svc.Status,
		ExitCode: svc.ExitCode,
		Output:   svc.outputSnapshot(),
	}
	if svc.Error != nil {
		result.Error = svc.Error.Error()
	}
	if svc.cmd != nil && svc.cmd.Process != nil {
		result.PID = svc.cmd.Process.Pid
	}
	return result
}

func decodeRPCParams(raw json.RawMessage, out any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func invalidParamsError(err error) *controllerRPCError {
	return &controllerRPCError{Code: jsonRPCInvalidParam, Message: err.Error()}
}

func internalRPCError(err error) *controllerRPCError {
	return &controllerRPCError{Code: jsonRPCInternal, Message: err.Error()}
}

func (req controllerRPCRequest) hasResponse() bool {
	id := req.responseID()
	return len(id) > 0 && string(id) != "null"
}

func (req controllerRPCRequest) responseID() json.RawMessage {
	if len(req.ID) == 0 {
		return json.RawMessage("null")
	}
	return req.ID
}
