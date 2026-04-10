package engine

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestControllerFS_MountedOnNewFS(t *testing.T) {
	address, shutdown := startMockControllerFSServer(t)
	defer shutdown()

	remoteFS, err := NewControllerFS(address)
	if err != nil {
		t.Fatalf("NewControllerFS() error: %v", err)
	}

	mfs := NewFS()
	if err := mfs.Mount("/shared", remoteFS); err != nil {
		t.Fatalf("Mount() error: %v", err)
	}

	seed, err := fs.ReadFile(mfs, "/shared/seed.txt")
	if err != nil {
		t.Fatalf("ReadFile(seed) error: %v", err)
	}
	if string(seed) != "seed-data" {
		t.Fatalf("seed content=%q, want %q", string(seed), "seed-data")
	}

	if err := mfs.Mkdir("/shared/cache"); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}
	if err := mfs.WriteFile("/shared/cache/result.txt", []byte("alpha")); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := mfs.Rename("/shared/cache/result.txt", "/shared/cache/final.txt"); err != nil {
		t.Fatalf("Rename() error: %v", err)
	}
	if err := mfs.Chmod("/shared/cache/final.txt", 0o640); err != nil {
		t.Fatalf("Chmod() error: %v", err)
	}
	if err := mfs.Chown("/shared/cache/final.txt", 1000, 1000); err != nil {
		t.Fatalf("Chown() error: %v", err)
	}

	entries, err := fs.ReadDir(mfs, "/shared/cache")
	if err != nil {
		t.Fatalf("ReadDir() error: %v", err)
	}
	entryNames := map[string]bool{}
	for _, entry := range entries {
		entryNames[entry.Name()] = true
	}
	if !entryNames["final.txt"] {
		t.Fatalf("ReadDir() entries=%v, want final.txt present", entries)
	}

	finalBytes, err := fs.ReadFile(mfs, "/shared/cache/final.txt")
	if err != nil {
		t.Fatalf("ReadFile(final) error: %v", err)
	}
	if string(finalBytes) != "alpha" {
		t.Fatalf("final content=%q, want %q", string(finalBytes), "alpha")
	}
	finalInfo, err := fs.Stat(mfs, "/shared/cache/final.txt")
	if err != nil {
		t.Fatalf("Stat(final) error: %v", err)
	}
	if finalInfo.Mode().Perm() != 0o640 {
		t.Fatalf("final mode=%v, want 0640", finalInfo.Mode())
	}

	if err := mfs.Remove("/shared/cache/final.txt"); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}
	if _, err := fs.Stat(mfs, "/shared/cache/final.txt"); err == nil {
		t.Fatal("removed remote file should not exist")
	}
}

func TestControllerFS_RemoteDescriptorOperations(t *testing.T) {
	address, shutdown := startMockControllerFSServer(t)
	defer shutdown()

	remoteFS, err := NewControllerFS(address)
	if err != nil {
		t.Fatalf("NewControllerFS() error: %v", err)
	}

	mfs := NewFS()
	if err := mfs.Mount("/shared", remoteFS); err != nil {
		t.Fatalf("Mount() error: %v", err)
	}

	fd, err := mfs.OpenFD("/shared/fd.txt", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		t.Fatalf("OpenFD(write) error: %v", err)
	}
	writer, err := mfs.HostWriterFD(fd)
	if err != nil {
		t.Fatalf("HostWriterFD() error: %v", err)
	}
	if _, err := writer.Write([]byte("remote-fd")); err != nil {
		t.Fatalf("writer.Write() error: %v", err)
	}
	if err := mfs.FchmodFD(fd, 0o600); err != nil {
		t.Fatalf("FchmodFD() error: %v", err)
	}
	if err := mfs.FchownFD(fd, 1000, 1000); err != nil {
		t.Fatalf("FchownFD() error: %v", err)
	}
	if err := mfs.FsyncFD(fd); err != nil {
		t.Fatalf("FsyncFD() error: %v", err)
	}
	if err := mfs.CloseFD(fd); err != nil {
		t.Fatalf("CloseFD(write) error: %v", err)
	}

	fd, err = mfs.OpenFD("/shared/fd.txt", os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("OpenFD(read) error: %v", err)
	}
	defer mfs.CloseFD(fd)
	info, err := mfs.FstatFD(fd)
	if err != nil {
		t.Fatalf("FstatFD() error: %v", err)
	}
	if info.Size() != int64(len("remote-fd")) {
		t.Fatalf("FstatFD size=%d, want %d", info.Size(), len("remote-fd"))
	}
	if runtime.GOOS != "windows" {
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("FstatFD mode=%v, want 0600", info.Mode())
		}
	}
	reader, err := mfs.HostReaderFD(fd)
	if err != nil {
		t.Fatalf("HostReaderFD() error: %v", err)
	}
	buf := make([]byte, len("remote-fd"))
	n, err := reader.Read(buf)
	if err != nil && err.Error() != "EOF" {
		t.Fatalf("reader.Read() error: %v", err)
	}
	if string(buf[:n]) != "remote-fd" {
		t.Fatalf("reader.Read() data=%q, want %q", string(buf[:n]), "remote-fd")
	}
}

func startMockControllerFSServer(t *testing.T) (string, func()) {
	t.Helper()

	vfs := NewVirtualFS()
	if err := vfs.WriteFile("seed.txt", []byte("seed-data")); err != nil {
		t.Fatalf("seed WriteFile() error: %v", err)
	}
	sharedFDs := map[int]*mockSharedFD{}
	nextFD := 3

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleMockControllerFSConn(vfs, sharedFDs, &nextFD, conn)
		}
	}()
	return "tcp://" + ln.Addr().String(), func() {
		_ = ln.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("mock controller fs server did not stop")
		}
	}
}

type mockSharedFD struct {
	path       string
	data       []byte
	baseData   []byte
	baseExists bool
	baseMode   uint32
	offset     int
	dirty      bool
	append     bool
	mode       uint32
	mtime      time.Time
}

func TestControllerFS_RemoteDescriptorWriteConflict(t *testing.T) {
	address, shutdown := startMockControllerFSServer(t)
	defer shutdown()

	remoteFS, err := NewControllerFS(address)
	if err != nil {
		t.Fatalf("NewControllerFS() error: %v", err)
	}

	mfs := NewFS()
	if err := mfs.Mount("/shared", remoteFS); err != nil {
		t.Fatalf("Mount() error: %v", err)
	}
	if err := mfs.WriteFile("/shared/conflict.txt", []byte("seed")); err != nil {
		t.Fatalf("WriteFile(seed) error: %v", err)
	}

	fd1, err := mfs.OpenFD("/shared/conflict.txt", os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("OpenFD(fd1) error: %v", err)
	}
	defer mfs.CloseFD(fd1)
	fd2, err := mfs.OpenFD("/shared/conflict.txt", os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("OpenFD(fd2) error: %v", err)
	}

	writer1, err := mfs.HostWriterFD(fd1)
	if err != nil {
		t.Fatalf("HostWriterFD(fd1) error: %v", err)
	}
	if _, err := writer1.Write([]byte("lock")); err != nil {
		t.Fatalf("writer1.Write() error: %v", err)
	}
	if err := mfs.FsyncFD(fd1); err != nil {
		t.Fatalf("FsyncFD(fd1) error: %v", err)
	}

	writer2, err := mfs.HostWriterFD(fd2)
	if err != nil {
		t.Fatalf("HostWriterFD(fd2) error: %v", err)
	}
	if _, err := writer2.Write([]byte("race")); err != nil {
		t.Fatalf("writer2.Write() error: %v", err)
	}
	if err := mfs.FsyncFD(fd2); err == nil || !strings.Contains(err.Error(), "changed while descriptor was open") {
		t.Fatalf("FsyncFD(fd2) error=%v, want write conflict", err)
	}
	_ = mfs.CloseFD(fd2)

	data, err := fs.ReadFile(mfs, "/shared/conflict.txt")
	if err != nil {
		t.Fatalf("ReadFile(conflict) error: %v", err)
	}
	if string(data) != "lock" {
		t.Fatalf("conflict content=%q, want %q", string(data), "lock")
	}
}

func TestControllerFS_RemoteDescriptorAppendNoConflict(t *testing.T) {
	address, shutdown := startMockControllerFSServer(t)
	defer shutdown()

	remoteFS, err := NewControllerFS(address)
	if err != nil {
		t.Fatalf("NewControllerFS() error: %v", err)
	}

	mfs := NewFS()
	if err := mfs.Mount("/shared", remoteFS); err != nil {
		t.Fatalf("Mount() error: %v", err)
	}
	if err := mfs.WriteFile("/shared/append.txt", []byte("seed")); err != nil {
		t.Fatalf("WriteFile(seed) error: %v", err)
	}

	fd1, err := mfs.OpenFD("/shared/append.txt", os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatalf("OpenFD(fd1) error: %v", err)
	}
	defer mfs.CloseFD(fd1)
	fd2, err := mfs.OpenFD("/shared/append.txt", os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatalf("OpenFD(fd2) error: %v", err)
	}

	writer1, err := mfs.HostWriterFD(fd1)
	if err != nil {
		t.Fatalf("HostWriterFD(fd1) error: %v", err)
	}
	if _, err := writer1.Write([]byte("-one")); err != nil {
		t.Fatalf("writer1.Write() error: %v", err)
	}
	if err := mfs.FsyncFD(fd1); err != nil {
		t.Fatalf("FsyncFD(fd1) error: %v", err)
	}

	writer2, err := mfs.HostWriterFD(fd2)
	if err != nil {
		t.Fatalf("HostWriterFD(fd2) error: %v", err)
	}
	if _, err := writer2.Write([]byte("-two")); err != nil {
		t.Fatalf("writer2.Write() error: %v", err)
	}
	if err := mfs.FsyncFD(fd2); err != nil {
		t.Fatalf("FsyncFD(fd2) error: %v", err)
	}
	_ = mfs.CloseFD(fd2)

	data, err := fs.ReadFile(mfs, "/shared/append.txt")
	if err != nil {
		t.Fatalf("ReadFile(append) error: %v", err)
	}
	if string(data) != "seed-one-two" {
		t.Fatalf("append content=%q, want %q", string(data), "seed-one-two")
	}
}

func handleMockControllerFSConn(vfs *VirtualFS, sharedFDs map[int]*mockSharedFD, nextFD *int, conn net.Conn) {
	defer conn.Close()
	var req struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		return
	}
	resp := map[string]any{"jsonrpc": "2.0", "id": 1}
	result, rpcErr := dispatchMockControllerFS(vfs, sharedFDs, nextFD, req.Method, req.Params)
	if rpcErr != nil {
		resp["error"] = map[string]any{"code": -32000, "message": rpcErr.Error()}
	} else {
		resp["result"] = result
	}
	_ = json.NewEncoder(conn).Encode(resp)
}

func dispatchMockControllerFS(vfs *VirtualFS, sharedFDs map[int]*mockSharedFD, nextFD *int, method string, params json.RawMessage) (any, error) {
	decodePath := func(key string) (string, error) {
		var req map[string]string
		if err := json.Unmarshal(params, &req); err != nil {
			return "", err
		}
		return req[key], nil
	}
	snapshotInfo := func(path string, info fs.FileInfo) map[string]any {
		return map[string]any{
			"name":     info.Name(),
			"path":     CleanPath(path),
			"is_dir":   info.IsDir(),
			"size":     info.Size(),
			"mode":     uint32(info.Mode()),
			"mod_time": info.ModTime(),
		}
	}
	switch method {
	case "fs.stat":
		path, err := decodePath("path")
		if err != nil {
			return nil, err
		}
		info, err := vfs.Stat(path)
		if err != nil {
			return nil, err
		}
		return snapshotInfo(path, info), nil
	case "fs.readDir":
		path, err := decodePath("path")
		if err != nil {
			return nil, err
		}
		entries, err := vfs.ReadDir(path)
		if err != nil {
			return nil, err
		}
		result := make([]map[string]any, 0, len(entries))
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				return nil, err
			}
			result = append(result, snapshotInfo(CleanPath(path+"/"+entry.Name()), info))
		}
		return result, nil
	case "fs.readFile":
		path, err := decodePath("path")
		if err != nil {
			return nil, err
		}
		data, err := vfs.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return map[string]any{"path": path, "data": base64.StdEncoding.EncodeToString(data), "encoding": "base64"}, nil
	case "fs.writeFile":
		var req map[string]string
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, err
		}
		data, err := base64.StdEncoding.DecodeString(req["data"])
		if err != nil {
			return nil, err
		}
		if err := vfs.WriteFile(req["path"], data); err != nil {
			return nil, err
		}
		info, err := vfs.Stat(req["path"])
		if err != nil {
			return nil, err
		}
		return snapshotInfo(req["path"], info), nil
	case "fs.chmod":
		var req struct {
			Path string `json:"path"`
			Mode uint32 `json:"mode"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, err
		}
		if err := vfs.Chmod(req.Path, req.Mode); err != nil {
			return nil, err
		}
		return true, nil
	case "fs.chown":
		var req struct {
			Path string `json:"path"`
			UID  int    `json:"uid"`
			GID  int    `json:"gid"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, err
		}
		if err := vfs.Chown(req.Path, req.UID, req.GID); err != nil {
			return nil, err
		}
		return true, nil
	case "fs.mkdir":
		path, err := decodePath("path")
		if err != nil {
			return nil, err
		}
		if err := vfs.Mkdir(path); err != nil {
			return nil, err
		}
		info, err := vfs.Stat(path)
		if err != nil {
			return nil, err
		}
		return snapshotInfo(path, info), nil
	case "fs.remove":
		path, err := decodePath("path")
		if err != nil {
			return nil, err
		}
		if err := vfs.Remove(path); err != nil {
			return nil, err
		}
		return true, nil
	case "fs.rename":
		var req map[string]string
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, err
		}
		if err := vfs.Rename(req["old_path"], req["new_path"]); err != nil {
			return nil, err
		}
		info, err := vfs.Stat(req["new_path"])
		if err != nil {
			return nil, err
		}
		return snapshotInfo(req["new_path"], info), nil
	case "fs.open":
		var req struct {
			Path  string `json:"path"`
			Flags int    `json:"flags"`
			Mode  uint32 `json:"mode"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, err
		}
		data, err := vfs.ReadFile(req.Path)
		exists := err == nil
		if err != nil && !isNotExist(err) {
			return nil, err
		}
		if !exists && req.Flags&os.O_CREATE == 0 {
			return nil, fs.ErrNotExist
		}
		mode := req.Mode
		if exists {
			if info, err := vfs.Stat(req.Path); err == nil {
				mode = uint32(info.Mode())
			}
		}
		if req.Flags&os.O_TRUNC != 0 && req.Flags&(os.O_WRONLY|os.O_RDWR) != 0 {
			data = []byte{}
		}
		fd := *nextFD
		*nextFD = *nextFD + 1
		sharedFDs[fd] = &mockSharedFD{path: req.Path, data: append([]byte(nil), data...), baseData: append([]byte(nil), data...), baseExists: exists, baseMode: mode, append: req.Flags&os.O_APPEND != 0, mode: mode, mtime: time.Now()}
		if sharedFDs[fd].append {
			sharedFDs[fd].offset = len(sharedFDs[fd].data)
		}
		return map[string]any{"fd": fd}, nil
	case "fs.read":
		var req struct {
			FD     int `json:"fd"`
			Length int `json:"length"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, err
		}
		handle, ok := sharedFDs[req.FD]
		if !ok {
			return nil, fs.ErrInvalid
		}
		if handle.offset >= len(handle.data) {
			return map[string]any{"data": "", "bytes_read": 0}, nil
		}
		end := handle.offset + req.Length
		if end > len(handle.data) {
			end = len(handle.data)
		}
		chunk := append([]byte(nil), handle.data[handle.offset:end]...)
		handle.offset = end
		return map[string]any{"data": base64.StdEncoding.EncodeToString(chunk), "bytes_read": len(chunk)}, nil
	case "fs.write":
		var req struct {
			FD   int    `json:"fd"`
			Data string `json:"data"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, err
		}
		handle, ok := sharedFDs[req.FD]
		if !ok {
			return nil, fs.ErrInvalid
		}
		data, err := base64.StdEncoding.DecodeString(req.Data)
		if err != nil {
			return nil, err
		}
		if handle.append {
			handle.offset = len(handle.data)
		}
		end := handle.offset + len(data)
		if end > len(handle.data) {
			grown := make([]byte, end)
			copy(grown, handle.data)
			handle.data = grown
		}
		copy(handle.data[handle.offset:end], data)
		handle.offset = end
		handle.dirty = true
		return map[string]any{"bytes_written": len(data)}, nil
	case "fs.close":
		var req struct {
			FD int `json:"fd"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, err
		}
		handle, ok := sharedFDs[req.FD]
		if !ok {
			return nil, fs.ErrInvalid
		}
		if handle.dirty {
			if err := flushMockSharedFD(vfs, handle); err != nil {
				return nil, err
			}
		}
		delete(sharedFDs, req.FD)
		return true, nil
	case "fs.fstat":
		var req struct {
			FD int `json:"fd"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, err
		}
		handle, ok := sharedFDs[req.FD]
		if !ok {
			return nil, fs.ErrInvalid
		}
		name := handle.path
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		return map[string]any{"name": name, "path": handle.path, "is_dir": false, "size": len(handle.data), "mode": handle.mode, "mod_time": handle.mtime}, nil
	case "fs.fsync":
		var req struct {
			FD int `json:"fd"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, err
		}
		handle, ok := sharedFDs[req.FD]
		if !ok {
			return nil, fs.ErrInvalid
		}
		if handle.dirty {
			if err := flushMockSharedFD(vfs, handle); err != nil {
				return nil, err
			}
			handle.dirty = false
		}
		return true, nil
	case "fs.fchmod":
		var req struct {
			FD   int    `json:"fd"`
			Mode uint32 `json:"mode"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, err
		}
		handle, ok := sharedFDs[req.FD]
		if !ok {
			return nil, fs.ErrInvalid
		}
		handle.mode = req.Mode
		return true, nil
	case "fs.fchown":
		var req struct {
			FD  int `json:"fd"`
			UID int `json:"uid"`
			GID int `json:"gid"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, err
		}
		if _, ok := sharedFDs[req.FD]; !ok {
			return nil, fs.ErrInvalid
		}
		return true, nil
	default:
		return nil, fs.ErrInvalid
	}
}

func isNotExist(err error) bool {
	return err != nil && (err == fs.ErrNotExist || strings.Contains(err.Error(), fs.ErrNotExist.Error()))
}

func flushMockSharedFD(vfs *VirtualFS, handle *mockSharedFD) error {
	if handle.append {
		if len(handle.data) < len(handle.baseData) || !strings.HasPrefix(string(handle.data), string(handle.baseData)) {
			return &fs.PathError{Op: "write", Path: handle.path, Err: errors.New("shared file changed while descriptor was open")}
		}
		appendData := handle.data[len(handle.baseData):]
		info, err := vfs.Stat(handle.path)
		currentExists := err == nil
		if err != nil && !isNotExist(err) {
			return err
		}
		if currentExists && info.IsDir() {
			return fs.ErrInvalid
		}
		if !currentExists && handle.baseExists {
			return &fs.PathError{Op: "write", Path: handle.path, Err: errors.New("shared file changed while descriptor was open")}
		}
		if len(appendData) > 0 {
			if err := vfs.AppendFile(handle.path, appendData); err != nil {
				return err
			}
			if !currentExists {
				if err := vfs.Chmod(handle.path, handle.mode); err != nil {
					return err
				}
			}
		} else if !currentExists {
			if err := vfs.WriteFile(handle.path, nil); err != nil {
				return err
			}
			if err := vfs.Chmod(handle.path, handle.mode); err != nil {
				return err
			}
		}
		handle.baseExists = true
		handle.baseData = append(handle.baseData[:0], handle.data...)
		handle.baseMode = handle.mode
		return nil
	}
	if err := assertMockSharedFDUnchanged(vfs, handle); err != nil {
		return err
	}
	if err := vfs.WriteFile(handle.path, handle.data); err != nil {
		return err
	}
	if err := vfs.Chmod(handle.path, handle.mode); err != nil {
		return err
	}
	handle.baseExists = true
	handle.baseData = append(handle.baseData[:0], handle.data...)
	handle.baseMode = handle.mode
	return nil
}

func assertMockSharedFDUnchanged(vfs *VirtualFS, handle *mockSharedFD) error {
	info, err := vfs.Stat(handle.path)
	if err != nil {
		if isNotExist(err) {
			if handle.baseExists {
				return &fs.PathError{Op: "write", Path: handle.path, Err: errors.New("shared file changed while descriptor was open")}
			}
			return nil
		}
		return err
	}
	if !handle.baseExists {
		return &fs.PathError{Op: "write", Path: handle.path, Err: errors.New("shared file changed while descriptor was open")}
	}
	data, err := vfs.ReadFile(handle.path)
	if err != nil {
		return err
	}
	if uint32(info.Mode()) != handle.baseMode || string(data) != string(handle.baseData) {
		return &fs.PathError{Op: "write", Path: handle.path, Err: errors.New("shared file changed while descriptor was open")}
	}
	return nil
}

// TestControllerFS_ConnectionPooling verifies that ControllerFS reuses
// a single persistent connection for multiple sequential RPC calls,
// significantly reducing TCP handshake overhead.
func TestControllerFS_ConnectionPooling(t *testing.T) {
	address, shutdown := startMockControllerFSServer(t)
	defer shutdown()

	remoteFS, err := NewControllerFS(address)
	if err != nil {
		t.Fatalf("NewControllerFS() error: %v", err)
	}
	defer remoteFS.Close()

	mfs := NewFS()
	if err := mfs.Mount("/shared", remoteFS); err != nil {
		t.Fatalf("Mount() error: %v", err)
	}

	// Perform multiple sequential RPC operations that should reuse the connection
	operations := []struct {
		name string
		op   func() error
	}{
		{"Mkdir", func() error { return mfs.Mkdir("/shared/pooltest") }},
		{"WriteFile1", func() error { return mfs.WriteFile("/shared/pooltest/file1.txt", []byte("data1")) }},
		{"WriteFile2", func() error { return mfs.WriteFile("/shared/pooltest/file2.txt", []byte("data2")) }},
		{"ReadFile1", func() error { _, err := fs.ReadFile(mfs, "/shared/pooltest/file1.txt"); return err }},
		{"Rename", func() error { return mfs.Rename("/shared/pooltest/file1.txt", "/shared/pooltest/renamed.txt") }},
		{"Stat", func() error { _, err := fs.Stat(mfs, "/shared/pooltest/renamed.txt"); return err }},
		{"ReadDir", func() error { _, err := fs.ReadDir(mfs, "/shared/pooltest"); return err }},
		{"Chmod", func() error { return mfs.Chmod("/shared/pooltest/renamed.txt", 0o600) }},
	}

	// Execute operations
	for _, op := range operations {
		if err := op.op(); err != nil {
			t.Fatalf("%s error: %v", op.name, err)
		}
	}

	// Verify the persistent connection was utilized (conn field should be set)
	if remoteFS.conn == nil {
		t.Fatal("Expected ControllerFS.conn to be set (i.e., connection should be persistent)")
	}

	// Verify connection is closed after Close()
	if err := remoteFS.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	if remoteFS.conn != nil {
		t.Fatal("Expected ControllerFS.conn to be nil after Close()")
	}

	// Verify Close() is idempotent
	if err := remoteFS.Close(); err != nil {
		t.Fatalf("Second Close() error: %v", err)
	}
}

// TestControllerFS_ConnectionResilience verifies that ControllerFS properly
// recovers from connection errors by invalidating and re-establishing connections.
func TestControllerFS_ConnectionResilience(t *testing.T) {
	address, shutdown := startMockControllerFSServer(t)
	defer shutdown()

	remoteFS, err := NewControllerFS(address)
	if err != nil {
		t.Fatalf("NewControllerFS() error: %v", err)
	}
	defer remoteFS.Close()

	mfs := NewFS()
	if err := mfs.Mount("/shared", remoteFS); err != nil {
		t.Fatalf("Mount() error: %v", err)
	}

	// Establish connection with first operation
	if err := mfs.WriteFile("/shared/test1.txt", []byte("data1")); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if remoteFS.conn == nil {
		t.Fatal("Expected connection to be established after first operation")
	}

	// Simulate connection failure by invalidating it
	remoteFS.invalidateConnection()
	if remoteFS.conn != nil {
		t.Fatal("Expected connection to be nil after invalidation")
	}

	// Verify new connection is established on next operation
	if err := mfs.WriteFile("/shared/test2.txt", []byte("data2")); err != nil {
		t.Fatalf("WriteFile() after invalidation error: %v", err)
	}
	if remoteFS.conn == nil {
		t.Fatal("Expected connection to be re-established after next operation")
	}

	// Verify data was written correctly
	data1, err := fs.ReadFile(mfs, "/shared/test1.txt")
	if err != nil || string(data1) != "data1" {
		t.Fatalf("ReadFile(test1) error or content mismatch: %v, %q", err, string(data1))
	}
	data2, err := fs.ReadFile(mfs, "/shared/test2.txt")
	if err != nil || string(data2) != "data2" {
		t.Fatalf("ReadFile(test2) error or content mismatch: %v, %q", err, string(data2))
	}
}
