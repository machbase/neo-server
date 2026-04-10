package engine

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"strings"
	"time"
)

const (
	ControllerAddressEnv         = "SERVICE_CONTROLLER"
	ControllerSharedMountEnv     = "SERVICE_SHARED_MOUNT"
	ControllerClientIDEnv        = "SERVICE_CONTROLLER_CLIENT_ID"
	DefaultControllerSharedMount = "/proc"
	controllerFSRPCVersion       = "2.0"
	controllerFSRPCTimeout       = 5 * time.Second
)

type ControllerFS struct {
	controller string
	clientID   string
	endpoint   controllerFSEndpoint
}

type controllerFSEndpoint struct {
	network string
	address string
}

type controllerFSRequest struct {
	Version string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type controllerFSResponse struct {
	Version string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type controllerFSInfoSnapshot struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	Mode    uint32    `json:"mode"`
	ModTime time.Time `json:"mod_time"`
}

type controllerFSReadFileResult struct {
	Path     string `json:"path"`
	Data     string `json:"data"`
	Encoding string `json:"encoding"`
}

type controllerFSReadFDResult struct {
	Data      string `json:"data"`
	BytesRead int    `json:"bytes_read"`
}

type controllerFSRemoteFD struct {
	fs *ControllerFS
	id int
}

type controllerFSFileInfo struct {
	snapshot controllerFSInfoSnapshot
}

type controllerFSDirEntry struct {
	info controllerFSFileInfo
}

var _ fs.FS = (*ControllerFS)(nil)
var _ fs.ReadFileFS = (*ControllerFS)(nil)
var _ fs.ReadDirFS = (*ControllerFS)(nil)
var _ fs.StatFS = (*ControllerFS)(nil)
var _ writeFileFS = (*ControllerFS)(nil)
var _ mkdirFS = (*ControllerFS)(nil)
var _ removeFS = (*ControllerFS)(nil)
var _ renameFS = (*ControllerFS)(nil)
var _ chmodFS = (*ControllerFS)(nil)
var _ chownFS = (*ControllerFS)(nil)
var _ openFDFileSystem = (*ControllerFS)(nil)
var _ fdHandle = (*controllerFSRemoteFD)(nil)

func NewControllerFS(controller string) (*ControllerFS, error) {
	return NewControllerFSWithClientID(controller, "")
}

func NewControllerFSWithClientID(controller string, clientID string) (*ControllerFS, error) {
	endpoint, err := parseControllerFSEndpoint(controller)
	if err != nil {
		return nil, err
	}
	return &ControllerFS{controller: controller, clientID: clientID, endpoint: endpoint}, nil
}

func (cfs *ControllerFS) Open(name string) (fs.File, error) {
	info, err := cfs.statSnapshot(name)
	if err != nil {
		return nil, err
	}
	if info.IsDir {
		entries, err := cfs.ReadDir(name)
		if err != nil {
			return nil, err
		}
		return &virtualOpenDir{info: controllerSnapshotToFileInfo(info), entries: entries}, nil
	}
	data, err := cfs.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return &virtualOpenFile{reader: bytes.NewReader(data), info: controllerSnapshotToFileInfo(info)}, nil
}

func (cfs *ControllerFS) ReadFile(name string) ([]byte, error) {
	path, err := normalizeControllerFSPath("readfile", name, false)
	if err != nil {
		return nil, err
	}
	var result controllerFSReadFileResult
	if err := cfs.call("fs.readFile", map[string]any{"path": path}, &result); err != nil {
		return nil, err
	}
	data, err := base64.StdEncoding.DecodeString(result.Data)
	if err != nil {
		return nil, &fs.PathError{Op: "readfile", Path: path, Err: err}
	}
	return data, nil
}

func (cfs *ControllerFS) ReadDir(name string) ([]fs.DirEntry, error) {
	path, err := normalizeControllerFSPath("readdir", name, true)
	if err != nil {
		return nil, err
	}
	var snapshots []controllerFSInfoSnapshot
	if err := cfs.call("fs.readDir", map[string]any{"path": path}, &snapshots); err != nil {
		return nil, err
	}
	entries := make([]fs.DirEntry, 0, len(snapshots))
	for _, snapshot := range snapshots {
		entries = append(entries, controllerFSDirEntry{info: controllerFSFileInfo{snapshot: snapshot}})
	}
	return entries, nil
}

func (cfs *ControllerFS) Stat(name string) (fs.FileInfo, error) {
	snapshot, err := cfs.statSnapshot(name)
	if err != nil {
		return nil, err
	}
	return controllerSnapshotToFileInfo(snapshot), nil
}

func (cfs *ControllerFS) WriteFile(name string, data []byte) error {
	path, err := normalizeControllerFSPath("writefile", name, false)
	if err != nil {
		return err
	}
	return cfs.call("fs.writeFile", map[string]any{
		"path": path,
		"data": base64.StdEncoding.EncodeToString(data),
	}, nil)
}

func (cfs *ControllerFS) Mkdir(name string) error {
	path, err := normalizeControllerFSPath("mkdir", name, false)
	if err != nil {
		return err
	}
	return cfs.call("fs.mkdir", map[string]any{"path": path}, nil)
}

func (cfs *ControllerFS) Remove(name string) error {
	path, err := normalizeControllerFSPath("remove", name, true)
	if err != nil {
		return err
	}
	return cfs.call("fs.remove", map[string]any{"path": path}, nil)
}

func (cfs *ControllerFS) Rename(oldName string, newName string) error {
	oldPath, err := normalizeControllerFSPath("rename", oldName, false)
	if err != nil {
		return err
	}
	newPath, err := normalizeControllerFSPath("rename", newName, false)
	if err != nil {
		return err
	}
	return cfs.call("fs.rename", map[string]any{"old_path": oldPath, "new_path": newPath}, nil)
}

func (cfs *ControllerFS) Chmod(name string, mode uint32) error {
	path, err := normalizeControllerFSPath("chmod", name, true)
	if err != nil {
		return err
	}
	return cfs.call("fs.chmod", map[string]any{"path": path, "mode": mode}, nil)
}

func (cfs *ControllerFS) Chown(name string, uid, gid int) error {
	path, err := normalizeControllerFSPath("chown", name, true)
	if err != nil {
		return err
	}
	return cfs.call("fs.chown", map[string]any{"path": path, "uid": uid, "gid": gid}, nil)
}

func (cfs *ControllerFS) OpenFD(name string, flags int, mode uint32) (fdHandle, error) {
	path, err := normalizeControllerFSPath("open", name, false)
	if err != nil {
		return nil, err
	}
	var result struct {
		FD int `json:"fd"`
	}
	if err := cfs.call("fs.open", map[string]any{"path": path, "flags": flags, "mode": mode, "owner": cfs.clientID}, &result); err != nil {
		return nil, err
	}
	return &controllerFSRemoteFD{fs: cfs, id: result.FD}, nil
}

func (cfs *ControllerFS) statSnapshot(name string) (controllerFSInfoSnapshot, error) {
	path, err := normalizeControllerFSPath("stat", name, true)
	if err != nil {
		return controllerFSInfoSnapshot{}, err
	}
	var snapshot controllerFSInfoSnapshot
	if err := cfs.call("fs.stat", map[string]any{"path": path}, &snapshot); err != nil {
		return controllerFSInfoSnapshot{}, err
	}
	return snapshot, nil
}

// call attempts an RPC call with exponential backoff retry for transient connection errors.
// RPC-level errors (method not found, invalid params, etc.) are not retried.
func (cfs *ControllerFS) call(method string, params any, out any) error {
	// Keep retry window short so service shutdown is not delayed.
	// Backoff schedule: 50ms -> 100ms -> 200ms (max)
	const (
		maxAttempts    = 3
		initialBackoff = 50 * time.Millisecond
		maxBackoff     = 200 * time.Millisecond
	)

	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt < maxAttempts; attempt++ {
		conn, err := net.DialTimeout(cfs.endpoint.network, cfs.endpoint.address, controllerFSRPCTimeout)
		if err != nil {
			// Retry only transport-level errors. Path/RPC errors are not retried.
			if !isRetryableControllerFSError(err) {
				return err
			}
			lastErr = err
			if attempt < maxAttempts-1 {
				time.Sleep(backoff)
				if backoff < maxBackoff {
					backoff *= 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
				}
			}
			continue
		}

		// Connection successful - attempt RPC
		err = cfs.callOnConnection(conn, method, params, out)
		conn.Close()

		// Non-retryable RPC errors (invalid RPC response, marshal error, etc.)
		// should fail immediately without retry
		if err == nil {
			return nil
		}

		// RPC/path errors are not retried; only transport-level failures are retried.
		if !isRetryableControllerFSError(err) {
			return err
		}

		// Connection or decode error - potentially transient
		lastErr = err
		if attempt < maxAttempts-1 {
			time.Sleep(backoff)
			if backoff < maxBackoff {
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
		}
	}

	return fmt.Errorf("controller RPC failed after %d attempts: %w", maxAttempts, lastErr)
}

// callOnConnection performs a single RPC call on an established connection.
// Does not retry; connection errors propagate directly.
func (cfs *ControllerFS) callOnConnection(conn net.Conn, method string, params any, out any) error {
	_ = conn.SetDeadline(time.Now().Add(controllerFSRPCTimeout))

	req := controllerFSRequest{
		Version: controllerFSRPCVersion,
		ID:      1,
		Method:  method,
		Params:  params,
	}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return err
	}
	var resp controllerFSResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return err
	}
	if resp.Error != nil {
		return controllerFSPathError(method, params, resp.Error.Code, resp.Error.Message)
	}
	if out == nil {
		return nil
	}
	if len(resp.Result) == 0 || string(resp.Result) == "null" {
		return nil
	}
	return json.Unmarshal(resp.Result, out)
}

// isRetryableControllerFSError returns true only for transient transport errors.
// RPC/path errors (e.g. ECONFLICT) must be returned immediately.
func isRetryableControllerFSError(err error) bool {
	if err == nil {
		return false
	}
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

func parseControllerFSEndpoint(raw string) (controllerFSEndpoint, error) {
	if raw == "" {
		return controllerFSEndpoint{}, fmt.Errorf("controller address is empty")
	}
	if head, tail, found := strings.Cut(raw, "://"); found {
		switch strings.ToLower(head) {
		case "tcp", "unix":
			if tail == "" {
				return controllerFSEndpoint{}, fmt.Errorf("controller address is empty")
			}
			return controllerFSEndpoint{network: strings.ToLower(head), address: tail}, nil
		default:
			return controllerFSEndpoint{}, fmt.Errorf("unsupported controller address scheme %q", head)
		}
	}
	return controllerFSEndpoint{network: "tcp", address: raw}, nil
}

func normalizeControllerFSPath(op string, name string, allowRoot bool) (string, error) {
	if name == "" {
		name = "."
	}
	trimmed := strings.TrimPrefix(name, "/")
	if trimmed == "" || trimmed == "." {
		if allowRoot {
			return "/", nil
		}
		return "", &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	if !fs.ValidPath(trimmed) {
		return "", &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	return CleanPath(trimmed), nil
}

func controllerSnapshotToFileInfo(snapshot controllerFSInfoSnapshot) fs.FileInfo {
	return controllerFSFileInfo{snapshot: snapshot}
}

func controllerFSPathError(method string, params any, code int, message string) error {
	path := "/"
	if typed, ok := params.(map[string]any); ok {
		if value, ok := typed["path"].(string); ok && value != "" {
			path = value
		} else if value, ok := typed["old_path"].(string); ok && value != "" {
			path = value
		}
	}
	err := fs.ErrInvalid
	msg := strings.ToLower(message)
	if strings.Contains(msg, "not found") || strings.Contains(msg, "no such") {
		err = fs.ErrNotExist
	}
	if code == -32009 {
		return &fs.PathError{Op: method, Path: path, Err: fmt.Errorf("ECONFLICT: %s", message)}
	}
	return &fs.PathError{Op: method, Path: path, Err: fmt.Errorf("%w: %s", err, message)}
}

func (fi controllerFSFileInfo) Name() string {
	return fi.snapshot.Name
}

func (fi controllerFSFileInfo) Size() int64 {
	return fi.snapshot.Size
}

func (fi controllerFSFileInfo) Mode() fs.FileMode {
	return fs.FileMode(fi.snapshot.Mode)
}

func (fi controllerFSFileInfo) ModTime() time.Time {
	return fi.snapshot.ModTime
}

func (fi controllerFSFileInfo) IsDir() bool {
	return fi.snapshot.IsDir
}

func (fi controllerFSFileInfo) Sys() any {
	return fi.snapshot
}

func (de controllerFSDirEntry) Name() string {
	return de.info.Name()
}

func (de controllerFSDirEntry) IsDir() bool {
	return de.info.IsDir()
}

func (de controllerFSDirEntry) Type() fs.FileMode {
	return de.info.Mode().Type()
}

func (de controllerFSDirEntry) Info() (fs.FileInfo, error) {
	return de.info, nil
}

func (fd *controllerFSRemoteFD) Read(p []byte) (int, error) {
	var result controllerFSReadFDResult
	if err := fd.fs.call("fs.read", map[string]any{"fd": fd.id, "length": len(p)}, &result); err != nil {
		return 0, err
	}
	data, err := base64.StdEncoding.DecodeString(result.Data)
	if err != nil {
		return 0, err
	}
	copy(p, data)
	if result.BytesRead == 0 {
		return 0, io.EOF
	}
	if result.BytesRead < len(p) {
		return result.BytesRead, io.EOF
	}
	return result.BytesRead, nil
}

func (fd *controllerFSRemoteFD) Write(p []byte) (int, error) {
	var result struct {
		BytesWritten int `json:"bytes_written"`
	}
	if err := fd.fs.call("fs.write", map[string]any{"fd": fd.id, "data": base64.StdEncoding.EncodeToString(p)}, &result); err != nil {
		return 0, err
	}
	return result.BytesWritten, nil
}

func (fd *controllerFSRemoteFD) Close() error {
	return fd.fs.call("fs.close", map[string]any{"fd": fd.id}, nil)
}

func (fd *controllerFSRemoteFD) Stat() (fs.FileInfo, error) {
	var snapshot controllerFSInfoSnapshot
	if err := fd.fs.call("fs.fstat", map[string]any{"fd": fd.id}, &snapshot); err != nil {
		return nil, err
	}
	return controllerSnapshotToFileInfo(snapshot), nil
}

func (fd *controllerFSRemoteFD) Sync() error {
	return fd.fs.call("fs.fsync", map[string]any{"fd": fd.id}, nil)
}

func (fd *controllerFSRemoteFD) Chmod(mode fs.FileMode) error {
	return fd.fs.call("fs.fchmod", map[string]any{"fd": fd.id, "mode": uint32(mode)}, nil)
}

func (fd *controllerFSRemoteFD) Chown(uid, gid int) error {
	return fd.fs.call("fs.fchown", map[string]any{"fd": fd.id, "uid": uid, "gid": gid}, nil)
}
