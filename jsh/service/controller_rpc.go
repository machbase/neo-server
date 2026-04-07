package service

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/jsh/engine"
)

const (
	jsonRPCVersion      = "2.0"
	jsonRPCInvalidReq   = -32600
	jsonRPCMethodMiss   = -32601
	jsonRPCInvalidParam = -32602
	jsonRPCInternal     = -32603
	jsonRPCNotFound     = -32004
	jsonRPCConflict     = -32009
)

type ConfigSnapshot struct {
	Name        string            `json:"name"`
	Enable      bool              `json:"enable"`
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

type ServiceRuntimeSnapshot struct {
	Output  []string       `json:"output"`
	Details map[string]any `json:"details"`
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

type SharedFileInfoSnapshot struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	Mode    uint32    `json:"mode"`
	ModTime time.Time `json:"mod_time"`
}

type SharedReadFileResult struct {
	Path     string `json:"path"`
	Data     string `json:"data"`
	Encoding string `json:"encoding"`
}

type SharedOpenFDResult struct {
	FD int `json:"fd"`
}

type SharedReadFDResult struct {
	Data      string `json:"data"`
	BytesRead int    `json:"bytes_read"`
}

type SharedWriteFDResult struct {
	BytesWritten int `json:"bytes_written"`
}

type sharedFileHandle struct {
	Path       string
	Owner      string
	Flags      int
	Mode       uint32
	BaseMode   uint32
	Data       []byte
	BaseData   []byte
	BaseExists bool
	Offset     int
	Dirty      bool
	ModeDirty  bool
	Readable   bool
	Writable   bool
	Append     bool
	Closed     bool
	ModTime    time.Time
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

type serviceRuntimeDetailRequest struct {
	Name  string `json:"name"`
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type sharedPathRequest struct {
	Path string `json:"path"`
}

type sharedWriteFileRequest struct {
	Path string `json:"path"`
	Data string `json:"data"`
}

type sharedChmodRequest struct {
	Path string `json:"path"`
	Mode uint32 `json:"mode"`
}

type sharedChownRequest struct {
	Path string `json:"path"`
	UID  int    `json:"uid"`
	GID  int    `json:"gid"`
}

type sharedRenameRequest struct {
	OldPath string `json:"old_path"`
	NewPath string `json:"new_path"`
}

type sharedOpenFDRequest struct {
	Path  string `json:"path"`
	Flags int    `json:"flags"`
	Mode  uint32 `json:"mode"`
	Owner string `json:"owner,omitempty"`
}

type sharedFDRequest struct {
	FD int `json:"fd"`
}

type sharedReadFDRequest struct {
	FD     int `json:"fd"`
	Length int `json:"length"`
}

type sharedWriteFDRequest struct {
	FD   int    `json:"fd"`
	Data string `json:"data"`
}

type sharedFchmodFDRequest struct {
	FD   int    `json:"fd"`
	Mode uint32 `json:"mode"`
}

type sharedFchownFDRequest struct {
	FD  int `json:"fd"`
	UID int `json:"uid"`
	GID int `json:"gid"`
}

var errSharedWriteConflict = errors.New("shared file changed while descriptor was open")

func (ctl *Controller) Address() string {
	ctl.mu.RLock()
	defer ctl.mu.RUnlock()
	return ctl.rpcListenAddr
}

func parseRPCAddress(raw string) (string, string, error) {
	scheme := "tcp"
	address := raw
	if head, tail, found := strings.Cut(raw, "://"); found {
		scheme = strings.ToLower(head)
		address = tail
	}
	if address == "" {
		return "", "", fmt.Errorf("rpc address is empty")
	}
	switch scheme {
	case "tcp", "unix":
		return scheme, address, nil
	default:
		return "", "", fmt.Errorf("unsupported rpc address scheme %q", scheme)
	}
}

func formatRPCAddress(network string, addr net.Addr) string {
	switch typed := addr.(type) {
	case *net.TCPAddr:
		return fmt.Sprintf("tcp://%s", typed.String())
	case *net.UnixAddr:
		return fmt.Sprintf("unix://%s", typed.Name)
	default:
		if addr == nil {
			return ""
		}
		return fmt.Sprintf("%s://%s", network, addr.String())
	}
}

func cleanupRPCAddress(raw string) {
	network, address, err := parseRPCAddress(raw)
	if err != nil || network != "unix" || address == "" {
		return
	}
	if err := os.Remove(address); err != nil && !errors.Is(err, os.ErrNotExist) {
		return
	}
}

func (ctl *Controller) startRPC() error {
	ctl.mu.Lock()
	defer ctl.mu.Unlock()

	if ctl.rpcLn != nil {
		return nil
	}
	network, address, err := parseRPCAddress(ctl.rpcConfigAddr)
	if err != nil {
		return fmt.Errorf("start controller rpc listener: %w", err)
	}
	ln, err := net.Listen(network, address)
	if err != nil {
		return fmt.Errorf("start controller rpc listener: %w", err)
	}
	ctl.rpcLn = ln
	ctl.rpcListenAddr = formatRPCAddress(network, ln.Addr())
	ctl.rpcWG.Add(1)
	go ctl.serveRPC(ln)
	return nil
}

func (ctl *Controller) stopRPC() {
	ctl.mu.Lock()
	ln := ctl.rpcLn
	listenAddr := ctl.rpcListenAddr
	ctl.rpcLn = nil
	ctl.rpcListenAddr = ""
	ctl.mu.Unlock()

	if ln == nil {
		return
	}
	_ = ln.Close()
	ctl.rpcWG.Wait()
	cleanupRPCAddress(listenAddr)
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
	case "service.runtime.get":
		var req serviceNameRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		svc := ctl.StatusOf(req.Name)
		if svc == nil {
			return nil, &controllerRPCError{Code: jsonRPCNotFound, Message: fmt.Sprintf("service %s not found", req.Name)}
		}
		return snapshotServiceRuntime(svc), nil
	case "service.runtime.detail.add":
		var req serviceRuntimeDetailRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		svc := ctl.StatusOf(req.Name)
		if svc == nil {
			return nil, &controllerRPCError{Code: jsonRPCNotFound, Message: fmt.Sprintf("service %s not found", req.Name)}
		}
		if err := svc.addDetail(req.Key, req.Value); err != nil {
			return nil, invalidParamsError(err)
		}
		return snapshotServiceRuntime(svc), nil
	case "service.runtime.detail.update":
		var req serviceRuntimeDetailRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		svc := ctl.StatusOf(req.Name)
		if svc == nil {
			return nil, &controllerRPCError{Code: jsonRPCNotFound, Message: fmt.Sprintf("service %s not found", req.Name)}
		}
		if err := svc.updateDetail(req.Key, req.Value); err != nil {
			return nil, &controllerRPCError{Code: jsonRPCNotFound, Message: err.Error()}
		}
		return snapshotServiceRuntime(svc), nil
	case "service.runtime.detail.set":
		var req serviceRuntimeDetailRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		svc := ctl.StatusOf(req.Name)
		if svc == nil {
			return nil, &controllerRPCError{Code: jsonRPCNotFound, Message: fmt.Sprintf("service %s not found", req.Name)}
		}
		if err := svc.setDetail(req.Key, req.Value); err != nil {
			return nil, invalidParamsError(err)
		}
		return snapshotServiceRuntime(svc), nil
	case "service.runtime.detail.delete":
		var req serviceRuntimeDetailRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		svc := ctl.StatusOf(req.Name)
		if svc == nil {
			return nil, &controllerRPCError{Code: jsonRPCNotFound, Message: fmt.Sprintf("service %s not found", req.Name)}
		}
		if err := svc.deleteDetail(req.Key); err != nil {
			return nil, &controllerRPCError{Code: jsonRPCNotFound, Message: err.Error()}
		}
		return snapshotServiceRuntime(svc), nil
	case "fs.stat":
		var req sharedPathRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		info, err := ctl.sharedStat(req.Path)
		if err != nil {
			return nil, mapSharedFSError(err)
		}
		return info, nil
	case "fs.readDir":
		var req sharedPathRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		entries, err := ctl.sharedReadDir(req.Path)
		if err != nil {
			return nil, mapSharedFSError(err)
		}
		return entries, nil
	case "fs.readFile":
		var req sharedPathRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		result, err := ctl.sharedReadFileRPC(req.Path)
		if err != nil {
			return nil, mapSharedFSError(err)
		}
		return result, nil
	case "fs.writeFile":
		var req sharedWriteFileRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		info, err := ctl.sharedWriteFileRPC(req.Path, req.Data)
		if err != nil {
			var base64Err base64.CorruptInputError
			if errors.As(err, &base64Err) {
				return nil, invalidParamsError(err)
			}
			return nil, mapSharedFSError(err)
		}
		return info, nil
	case "fs.chmod":
		var req sharedChmodRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		if err := ctl.sharedChmod(req.Path, req.Mode); err != nil {
			return nil, mapSharedFSError(err)
		}
		return true, nil
	case "fs.chown":
		var req sharedChownRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		if err := ctl.sharedChown(req.Path, req.UID, req.GID); err != nil {
			return nil, mapSharedFSError(err)
		}
		return true, nil
	case "fs.mkdir":
		var req sharedPathRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		if err := ctl.sharedMkdir(req.Path); err != nil {
			return nil, mapSharedFSError(err)
		}
		info, err := ctl.sharedStat(req.Path)
		if err != nil {
			return nil, mapSharedFSError(err)
		}
		return info, nil
	case "fs.remove":
		var req sharedPathRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		if err := ctl.sharedRemove(req.Path); err != nil {
			return nil, mapSharedFSError(err)
		}
		return true, nil
	case "fs.rename":
		var req sharedRenameRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		if err := ctl.sharedRename(req.OldPath, req.NewPath); err != nil {
			return nil, mapSharedFSError(err)
		}
		info, err := ctl.sharedStat(req.NewPath)
		if err != nil {
			return nil, mapSharedFSError(err)
		}
		return info, nil
	case "fs.open":
		var req sharedOpenFDRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		result, err := ctl.sharedOpenFD(req.Path, req.Flags, req.Mode, req.Owner)
		if err != nil {
			return nil, mapSharedFSError(err)
		}
		return result, nil
	case "fs.read":
		var req sharedReadFDRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		result, err := ctl.sharedReadFD(req.FD, req.Length)
		if err != nil {
			return nil, mapSharedFSError(err)
		}
		return result, nil
	case "fs.write":
		var req sharedWriteFDRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		data, err := base64.StdEncoding.DecodeString(req.Data)
		if err != nil {
			return nil, invalidParamsError(err)
		}
		result, err := ctl.sharedWriteFD(req.FD, data)
		if err != nil {
			return nil, mapSharedFSError(err)
		}
		return result, nil
	case "fs.close":
		var req sharedFDRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		if err := ctl.sharedCloseFD(req.FD); err != nil {
			return nil, mapSharedFSError(err)
		}
		return true, nil
	case "fs.fstat":
		var req sharedFDRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		result, err := ctl.sharedFstatFD(req.FD)
		if err != nil {
			return nil, mapSharedFSError(err)
		}
		return result, nil
	case "fs.fsync":
		var req sharedFDRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		if err := ctl.sharedFsyncFD(req.FD); err != nil {
			return nil, mapSharedFSError(err)
		}
		return true, nil
	case "fs.fchmod":
		var req sharedFchmodFDRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		if err := ctl.sharedFchmodFD(req.FD, req.Mode); err != nil {
			return nil, mapSharedFSError(err)
		}
		return true, nil
	case "fs.fchown":
		var req sharedFchownFDRequest
		if err := decodeRPCParams(params, &req); err != nil {
			return nil, invalidParamsError(err)
		}
		if err := ctl.sharedFchownFD(req.FD, req.UID, req.GID); err != nil {
			return nil, mapSharedFSError(err)
		}
		return true, nil
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
		return ctl.reloadSnapshot(), nil
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

func (ctl *Controller) reloadSnapshot() ControllerUpdateResult {
	result := ControllerUpdateResult{Actions: []ControllerAction{}}
	ctl.Reload(func(sc *Config, action string, err error) {
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
	if svc.Status != ServiceStatusStopped && svc.Status != ServiceStatusFailed {
		if svc.cmd != nil && svc.cmd.Process != nil {
			result.PID = svc.cmd.Process.Pid
		}
	}
	return result
}

func snapshotServiceRuntime(svc *Service) ServiceRuntimeSnapshot {
	return ServiceRuntimeSnapshot{
		Output:  svc.outputSnapshot(),
		Details: svc.detailsSnapshot(),
	}
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
	if errors.Is(err, errServiceMustBeStopped) {
		return &controllerRPCError{Code: jsonRPCConflict, Message: err.Error()}
	}
	return &controllerRPCError{Code: jsonRPCInternal, Message: err.Error()}
}

func mapSharedFSError(err error) *controllerRPCError {
	if errors.Is(err, errSharedWriteConflict) {
		return &controllerRPCError{Code: jsonRPCConflict, Message: err.Error()}
	}
	if errors.Is(err, fs.ErrNotExist) {
		return &controllerRPCError{Code: jsonRPCNotFound, Message: err.Error()}
	}
	return invalidParamsError(err)
}

func snapshotSharedFileInfo(path string, info fs.FileInfo) SharedFileInfoSnapshot {
	return SharedFileInfoSnapshot{
		Name:    info.Name(),
		Path:    engine.CleanPath(path),
		IsDir:   info.IsDir(),
		Size:    info.Size(),
		Mode:    uint32(info.Mode()),
		ModTime: info.ModTime(),
	}
}

func snapshotSharedDirEntries(parent string, entries []fs.DirEntry) ([]SharedFileInfoSnapshot, error) {
	result := make([]SharedFileInfoSnapshot, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		result = append(result, snapshotSharedFileInfo(engine.CleanPath(parent+"/"+entry.Name()), info))
	}
	return result, nil
}

func (ctl *Controller) sharedStat(name string) (SharedFileInfoSnapshot, error) {
	ctl.sharedMu.RLock()
	defer ctl.sharedMu.RUnlock()
	path := engine.CleanPath(name)
	info, err := ctl.sharedFS.Stat(path)
	if err != nil {
		return SharedFileInfoSnapshot{}, err
	}
	return snapshotSharedFileInfo(path, info), nil
}

func (ctl *Controller) sharedReadDir(name string) ([]SharedFileInfoSnapshot, error) {
	ctl.sharedMu.RLock()
	defer ctl.sharedMu.RUnlock()
	path := engine.CleanPath(name)
	entries, err := ctl.sharedFS.ReadDir(path)
	if err != nil {
		return nil, err
	}
	return snapshotSharedDirEntries(path, entries)
}

func (ctl *Controller) sharedReadFile(name string) ([]byte, error) {
	path := engine.CleanPath(name)
	return ctl.sharedReadFilePath(path)
}

func (ctl *Controller) sharedReadFilePath(path string) ([]byte, error) {
	ctl.sharedMu.RLock()
	defer ctl.sharedMu.RUnlock()
	data, err := ctl.sharedFS.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (ctl *Controller) sharedReadFileRPC(name string) (SharedReadFileResult, error) {
	path := engine.CleanPath(name)
	data, err := ctl.sharedReadFilePath(path)
	if err != nil {
		return SharedReadFileResult{}, err
	}
	return SharedReadFileResult{
		Path:     path,
		Data:     base64.StdEncoding.EncodeToString(data),
		Encoding: "base64",
	}, nil
}

func (ctl *Controller) mutateSharedFS(apply func(*engine.VirtualFS) error, persist func() error) error {
	ctl.sharedMu.Lock()
	defer ctl.sharedMu.Unlock()
	backup := ctl.sharedFS.Clone()
	if err := apply(ctl.sharedFS); err != nil {
		return err
	}
	if err := persist(); err != nil {
		ctl.sharedFS = backup
		return err
	}
	return nil
}

func (ctl *Controller) SharedMountPoint() string {
	return ctl.sharedMountPoint
}

func (ctl *Controller) WriteSharedFileString(name string, str string) error {
	return ctl.sharedWriteFile(name, []byte(str))
}

func (ctl *Controller) WriteSharedFileJSON(name string, v any) error {
	b := &bytes.Buffer{}
	enc := json.NewEncoder(b)
	enc.SetIndent("", "  ")
	err := enc.Encode(v)
	if err != nil {
		return err
	}
	return ctl.sharedWriteFile(name, b.Bytes())
}

func (ctl *Controller) sharedWriteFileRPC(name string, encoded string) (SharedFileInfoSnapshot, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return SharedFileInfoSnapshot{}, err
	}
	if err := ctl.sharedWriteFile(name, data); err != nil {
		return SharedFileInfoSnapshot{}, err
	}
	return ctl.sharedStat(name)
}

func (ctl *Controller) sharedWriteFile(name string, data []byte) error {
	path := engine.CleanPath(name)
	return ctl.mutateSharedFS(func(vfs *engine.VirtualFS) error {
		return vfs.WriteFile(path, data)
	}, func() error {
		return ctl.persistSharedWriteFile(path, data)
	})
}

func (ctl *Controller) sharedWriteFileMode(name string, data []byte, mode uint32) error {
	path := engine.CleanPath(name)
	return ctl.mutateSharedFS(func(vfs *engine.VirtualFS) error {
		if err := vfs.WriteFile(path, data); err != nil {
			return err
		}
		return vfs.Chmod(path, mode)
	}, func() error {
		if err := ctl.persistSharedWriteFile(path, data); err != nil {
			return err
		}
		if ctl.backendDir == "" {
			return nil
		}
		return ctl.fs.Chmod(ctl.sharedBackendPath(path), mode)
	})
}

func (ctl *Controller) sharedChmod(name string, mode uint32) error {
	path := engine.CleanPath(name)
	return ctl.mutateSharedFS(func(vfs *engine.VirtualFS) error {
		return vfs.Chmod(path, mode)
	}, func() error {
		if ctl.backendDir == "" {
			return nil
		}
		return ctl.fs.Chmod(ctl.sharedBackendPath(path), mode)
	})
}

func (ctl *Controller) sharedChown(name string, uid, gid int) error {
	ctl.sharedMu.RLock()
	defer ctl.sharedMu.RUnlock()
	return ctl.sharedFS.Chown(engine.CleanPath(name), uid, gid)
}
func (ctl *Controller) sharedMkdir(name string) error {
	path := engine.CleanPath(name)
	return ctl.mutateSharedFS(func(vfs *engine.VirtualFS) error {
		return vfs.Mkdir(path)
	}, func() error {
		return ctl.persistSharedMkdir(path)
	})
}

func (ctl *Controller) sharedRemove(name string) error {
	path := engine.CleanPath(name)
	return ctl.mutateSharedFS(func(vfs *engine.VirtualFS) error {
		return vfs.Remove(path)
	}, func() error {
		return ctl.persistSharedRemove(path)
	})
}

func (ctl *Controller) sharedRename(oldName string, newName string) error {
	oldPath := engine.CleanPath(oldName)
	newPath := engine.CleanPath(newName)
	return ctl.mutateSharedFS(func(vfs *engine.VirtualFS) error {
		return vfs.Rename(oldPath, newPath)
	}, func() error {
		return ctl.persistSharedRename(oldPath, newPath)
	})
}

func (ctl *Controller) sharedOpenFD(name string, flags int, mode uint32, owner string) (SharedOpenFDResult, error) {
	ctl.sharedMu.Lock()
	defer ctl.sharedMu.Unlock()

	path := engine.CleanPath(name)
	readable := flags == os.O_RDONLY || flags&os.O_RDWR != 0
	writable := flags&(os.O_WRONLY|os.O_RDWR) != 0
	appendMode := flags&os.O_APPEND != 0
	createMode := flags&os.O_CREATE != 0
	truncateMode := flags&os.O_TRUNC != 0
	exclusiveMode := flags&os.O_EXCL != 0

	var data []byte
	var info fs.FileInfo
	modTime := time.Now()
	info, err := ctl.sharedFS.Stat(path)
	exists := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return SharedOpenFDResult{}, err
	}
	if exists && info.IsDir() {
		return SharedOpenFDResult{}, fs.ErrInvalid
	}
	if !exists {
		if !createMode {
			return SharedOpenFDResult{}, fs.ErrNotExist
		}
		if mode == 0 {
			mode = 0644
		}
	} else {
		if createMode && exclusiveMode {
			return SharedOpenFDResult{}, fs.ErrExist
		}
		data, err = ctl.sharedFS.ReadFile(path)
		if err != nil {
			return SharedOpenFDResult{}, err
		}
		mode = uint32(info.Mode())
		modTime = info.ModTime()
	}
	if truncateMode && writable {
		data = []byte{}
	}
	handle := &sharedFileHandle{
		Path:       path,
		Owner:      owner,
		Flags:      flags,
		Mode:       mode,
		BaseMode:   mode,
		Data:       append([]byte(nil), data...),
		BaseData:   append([]byte(nil), data...),
		BaseExists: exists,
		Offset:     0,
		Dirty:      (truncateMode && writable) || (!exists && createMode),
		Readable:   readable,
		Writable:   writable,
		Append:     appendMode,
		ModTime:    modTime,
	}
	if appendMode {
		handle.Offset = len(handle.Data)
	}
	fd := ctl.sharedNextFD
	ctl.sharedNextFD++
	ctl.sharedFDs[fd] = handle
	return SharedOpenFDResult{FD: fd}, nil
}

func (ctl *Controller) sharedReadFD(fd int, length int) (SharedReadFDResult, error) {
	ctl.sharedMu.Lock()
	defer ctl.sharedMu.Unlock()
	handle, err := ctl.lookupSharedFDLocked(fd)
	if err != nil {
		return SharedReadFDResult{}, err
	}
	if !handle.Readable {
		return SharedReadFDResult{}, fs.ErrPermission
	}
	if length < 0 {
		return SharedReadFDResult{}, fs.ErrInvalid
	}
	if handle.Offset >= len(handle.Data) || length == 0 {
		return SharedReadFDResult{Data: "", BytesRead: 0}, nil
	}
	end := handle.Offset + length
	if end > len(handle.Data) {
		end = len(handle.Data)
	}
	chunk := append([]byte(nil), handle.Data[handle.Offset:end]...)
	handle.Offset = end
	return SharedReadFDResult{Data: base64.StdEncoding.EncodeToString(chunk), BytesRead: len(chunk)}, nil
}

func (ctl *Controller) sharedWriteFD(fd int, data []byte) (SharedWriteFDResult, error) {
	ctl.sharedMu.Lock()
	defer ctl.sharedMu.Unlock()
	handle, err := ctl.lookupSharedFDLocked(fd)
	if err != nil {
		return SharedWriteFDResult{}, err
	}
	if !handle.Writable {
		return SharedWriteFDResult{}, fs.ErrPermission
	}
	if handle.Append {
		handle.Offset = len(handle.Data)
	}
	end := handle.Offset + len(data)
	if end > len(handle.Data) {
		grown := make([]byte, end)
		copy(grown, handle.Data)
		handle.Data = grown
	}
	copy(handle.Data[handle.Offset:end], data)
	handle.Offset = end
	handle.Dirty = true
	handle.ModTime = time.Now()
	return SharedWriteFDResult{BytesWritten: len(data)}, nil
}

func (ctl *Controller) sharedCloseFD(fd int) error {
	ctl.sharedMu.Lock()
	defer ctl.sharedMu.Unlock()
	handle, err := ctl.lookupSharedFDLocked(fd)
	if err != nil {
		return err
	}
	if err := ctl.flushSharedFDLocked(handle); err != nil {
		return err
	}
	if handle, ok := ctl.sharedFDs[fd]; ok {
		handle.Closed = true
		delete(ctl.sharedFDs, fd)
	}
	return nil
}

func (ctl *Controller) sharedFstatFD(fd int) (SharedFileInfoSnapshot, error) {
	ctl.sharedMu.RLock()
	defer ctl.sharedMu.RUnlock()
	handle, err := ctl.lookupSharedFDRLocked(fd)
	if err != nil {
		return SharedFileInfoSnapshot{}, err
	}
	mode := fs.FileMode(handle.Mode)
	if mode == 0 {
		mode = 0644
	}
	return SharedFileInfoSnapshot{
		Name:    pathBase(handle.Path),
		Path:    handle.Path,
		IsDir:   false,
		Size:    int64(len(handle.Data)),
		Mode:    uint32(mode),
		ModTime: handle.ModTime,
	}, nil
}

func (ctl *Controller) sharedFsyncFD(fd int) error {
	ctl.sharedMu.Lock()
	defer ctl.sharedMu.Unlock()
	handle, err := ctl.lookupSharedFDLocked(fd)
	if err != nil {
		return err
	}
	return ctl.flushSharedFDLocked(handle)
}

func (ctl *Controller) sharedFchmodFD(fd int, mode uint32) error {
	ctl.sharedMu.Lock()
	defer ctl.sharedMu.Unlock()
	handle, err := ctl.lookupSharedFDLocked(fd)
	if err != nil {
		return err
	}
	handle.Mode = mode
	handle.ModeDirty = true
	handle.ModTime = time.Now()
	return nil
}

func (ctl *Controller) sharedFchownFD(fd int, uid, gid int) error {
	ctl.sharedMu.RLock()
	defer ctl.sharedMu.RUnlock()
	_, err := ctl.lookupSharedFDRLocked(fd)
	return err
}

func (ctl *Controller) flushSharedFDLocked(handle *sharedFileHandle) error {
	if !handle.Dirty && !handle.ModeDirty {
		return nil
	}
	if handle.Append && !handle.ModeDirty {
		return ctl.flushSharedAppendFDLocked(handle)
	}
	if err := ctl.assertSharedFDUnchangedLocked(handle); err != nil {
		return err
	}
	backup := ctl.sharedFS.Clone()
	if err := ctl.sharedFS.WriteFile(handle.Path, handle.Data); err != nil {
		return err
	}
	if err := ctl.sharedFS.Chmod(handle.Path, handle.Mode); err != nil {
		ctl.sharedFS = backup
		return err
	}
	if err := ctl.persistSharedWriteFile(handle.Path, handle.Data); err != nil {
		ctl.sharedFS = backup
		return err
	}
	if ctl.backendDir != "" {
		if err := ctl.fs.Chmod(ctl.sharedBackendPath(handle.Path), handle.Mode); err != nil {
			ctl.sharedFS = backup
			return err
		}
	}
	handle.BaseExists = true
	handle.BaseData = append(handle.BaseData[:0], handle.Data...)
	handle.BaseMode = handle.Mode
	handle.Dirty = false
	handle.ModeDirty = false
	return nil
}

func (ctl *Controller) flushSharedAppendFDLocked(handle *sharedFileHandle) error {
	if len(handle.Data) < len(handle.BaseData) || !bytes.Equal(handle.Data[:len(handle.BaseData)], handle.BaseData) {
		return &fs.PathError{Op: "write", Path: handle.Path, Err: errSharedWriteConflict}
	}
	appendData := append([]byte(nil), handle.Data[len(handle.BaseData):]...)
	info, err := ctl.sharedFS.Stat(handle.Path)
	currentExists := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if currentExists && info.IsDir() {
		return fs.ErrInvalid
	}
	if !currentExists && handle.BaseExists {
		return &fs.PathError{Op: "write", Path: handle.Path, Err: errSharedWriteConflict}
	}
	if len(appendData) == 0 {
		if !currentExists {
			backup := ctl.sharedFS.Clone()
			if err := ctl.sharedFS.WriteFile(handle.Path, nil); err != nil {
				return err
			}
			if err := ctl.sharedFS.Chmod(handle.Path, handle.Mode); err != nil {
				ctl.sharedFS = backup
				return err
			}
			if err := ctl.persistSharedWriteFile(handle.Path, nil); err != nil {
				ctl.sharedFS = backup
				return err
			}
		}
		handle.BaseExists = true
		handle.BaseData = append(handle.BaseData[:0], handle.Data...)
		handle.BaseMode = handle.Mode
		handle.Dirty = false
		handle.ModeDirty = false
		return nil
	}
	backup := ctl.sharedFS.Clone()
	if err := ctl.sharedFS.AppendFile(handle.Path, appendData); err != nil {
		return err
	}
	if !currentExists {
		if err := ctl.sharedFS.Chmod(handle.Path, handle.Mode); err != nil {
			ctl.sharedFS = backup
			return err
		}
	}
	if err := ctl.persistSharedAppendFile(handle.Path, appendData); err != nil {
		ctl.sharedFS = backup
		return err
	}
	if ctl.backendDir != "" && !currentExists {
		if err := ctl.fs.Chmod(ctl.sharedBackendPath(handle.Path), handle.Mode); err != nil {
			ctl.sharedFS = backup
			return err
		}
	}
	handle.BaseExists = true
	handle.BaseData = append(handle.BaseData[:0], handle.Data...)
	handle.BaseMode = handle.Mode
	handle.Dirty = false
	handle.ModeDirty = false
	return nil
}

func (ctl *Controller) assertSharedFDUnchangedLocked(handle *sharedFileHandle) error {
	info, err := ctl.sharedFS.Stat(handle.Path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			if handle.BaseExists {
				return &fs.PathError{Op: "write", Path: handle.Path, Err: errSharedWriteConflict}
			}
			return nil
		}
		return err
	}
	if info.IsDir() {
		return fs.ErrInvalid
	}
	if !handle.BaseExists {
		return &fs.PathError{Op: "write", Path: handle.Path, Err: errSharedWriteConflict}
	}
	data, err := ctl.sharedFS.ReadFile(handle.Path)
	if err != nil {
		return err
	}
	if uint32(info.Mode()) != handle.BaseMode || !bytes.Equal(data, handle.BaseData) {
		return &fs.PathError{Op: "write", Path: handle.Path, Err: errSharedWriteConflict}
	}
	return nil
}

func (ctl *Controller) lookupSharedFDLocked(fd int) (*sharedFileHandle, error) {
	handle, ok := ctl.sharedFDs[fd]
	if !ok || handle.Closed {
		return nil, fs.ErrInvalid
	}
	return handle, nil
}

func (ctl *Controller) lookupSharedFDRLocked(fd int) (*sharedFileHandle, error) {
	handle, ok := ctl.sharedFDs[fd]
	if !ok || handle.Closed {
		return nil, fs.ErrInvalid
	}
	return handle, nil
}

func (ctl *Controller) cleanupSharedFDsByOwner(owner string) {
	if owner == "" {
		return
	}
	ctl.sharedMu.Lock()
	defer ctl.sharedMu.Unlock()
	for fd, handle := range ctl.sharedFDs {
		if handle == nil || handle.Owner != owner {
			continue
		}
		handle.Closed = true
		delete(ctl.sharedFDs, fd)
	}
}

func pathBase(path string) string {
	trimmed := strings.TrimSuffix(path, "/")
	if idx := strings.LastIndex(trimmed, "/"); idx >= 0 {
		return trimmed[idx+1:]
	}
	return trimmed
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
