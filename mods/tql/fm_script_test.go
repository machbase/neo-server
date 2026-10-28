package tql

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/machbase/neo-client/v2/api"
	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/test_engine"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/stretchr/testify/require"
)

// The process working directory differs from the configured file root, as it
// does for a Windows service started in C:\Windows\System32 (machbase/neo#1569).
func TestScriptWorkMountFollowsFileRoot(t *testing.T) {
	fileRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(fileRoot, "marker-1569.txt"), []byte("ok"), 0644))

	serverFs, err := test_engine.NewWorkspace(engine.FSTab{MountPoint: "/work", Source: fileRoot})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	output := &bytes.Buffer{}
	task := NewTaskContext(ctx)
	task.SetFileSystem(serverFs)
	task.SetOutputWriter(output)
	require.NoError(t, task.CompileString(`
SCRIPT({
	const fs = require("fs");
	$.yield(fs.readFileSync("/work/marker-1569.txt", "utf8"));
})
CSV()
`))
	result := task.Execute()
	require.NoError(t, result.Err)
	require.Equal(t, "ok\n\n", output.String())
}

func TestFilePathOpensThroughVFS(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "payload.txt"), []byte("virtual content"), 0644))
	serverFs, err := test_engine.NewWorkspace(engine.FSTab{MountPoint: "/work", Source: dir})
	require.NoError(t, err)

	task := NewTask()
	task.SetFileSystem(serverFs)
	filePath, err := NewNode(task).fmFile("/payload.txt")
	require.NoError(t, err)
	require.Empty(t, filePath.AbsPath)
	require.False(t, filePath.IsDir)
	require.NotNil(t, filePath.OpenFile)

	file, err := filePath.OpenFile()
	require.NoError(t, err)
	content, err := io.ReadAll(file)
	require.NoError(t, err)
	require.NoError(t, file.Close())
	require.Equal(t, "virtual content", string(content))
}

func TestScriptWorkDir(t *testing.T) {
	fileRoot := t.TempDir()
	tests := []struct {
		name   string
		dirs   engine.FSTabs
		expect string
	}{
		{name: "no_server_fs", dirs: nil, expect: ""},
		{name: "no_root_mount", dirs: engine.FSTabs{{MountPoint: "/work/data", Source: fileRoot}}, expect: ""},
		{name: "root_mount", dirs: engine.FSTabs{{MountPoint: "/work", Source: fileRoot}}, expect: fileRoot},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := NewTask()
			if tc.dirs != nil {
				serverFs, err := test_engine.NewWorkspace(tc.dirs...)
				require.NoError(t, err)
				task.SetFileSystem(serverFs)
			}
			require.Equal(t, tc.expect, scriptWorkDir(NewNode(task)))
		})
	}
}

func TestScriptRootlessWorkMountIsEmpty(t *testing.T) {
	submount := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(submount, "marker.txt"), []byte("hidden"), 0644))
	tests := []struct {
		name string
		dirs engine.FSTabs
	}{
		{name: "no_server_fs"},
		{name: "child_mount_only", dirs: engine.FSTabs{{MountPoint: "/work/data", Source: submount}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			output := &bytes.Buffer{}
			task := NewTaskContext(ctx)
			if tc.dirs != nil {
				serverFs, err := test_engine.NewWorkspace(tc.dirs...)
				require.NoError(t, err)
				task.SetFileSystem(serverFs)
			}
			task.SetOutputWriter(output)
			require.NoError(t, task.CompileString(`
SCRIPT({
	const fs = require("fs");
	$.yield(fs.readdirSync("/work").length === 2);
})
CSV()
`))
			result := task.Execute()
			require.NoError(t, result.Err)
			require.Equal(t, "true\n\n", output.String())
		})
	}
}

func TestJSLog(t *testing.T) {
	t.Run("plain writer", func(t *testing.T) {
		buffer := &bytes.Buffer{}
		logger := &JSLog{w: buffer}

		logger.Print("print")
		logger.Println(" line")
		logger.Printf("value=%d", 42)
		logger.Log(slog.LevelWarn, "warning")

		require.Contains(t, buffer.String(), "print line\nvalue=42")
		require.Contains(t, buffer.String(), "[WARN]")
		require.Contains(t, buffer.String(), "warning")
	})

	t.Run("logging writer", func(t *testing.T) {
		buffer := &bytes.Buffer{}
		writer := logging.NewLog("script-test", buffer)
		writer.SetLevel(logging.LevelInfo)

		(&JSLog{w: writer}).Log(slog.LevelInfo, "message")

		require.Contains(t, buffer.String(), "message")
	})

	t.Run("task writer", func(t *testing.T) {
		buffer := &bytes.Buffer{}
		task := NewTask()
		task.SetLogWriter(buffer)
		task.SetLogLevel(INFO)

		(&JSLog{w: task}).Println("message")

		require.Contains(t, buffer.String(), "message")
	})
}

func TestScriptDataTypes(t *testing.T) {
	tests := map[string]api.DataType{
		"int16":    api.DataTypeInt16,
		"int32":    api.DataTypeInt32,
		"int64":    api.DataTypeInt64,
		"datetime": api.DataTypeDatetime,
		"float":    api.DataTypeFloat32,
		"double":   api.DataTypeFloat64,
		"ipv4":     api.DataTypeIPv4,
		"ipv6":     api.DataTypeIPv6,
		"varchar":  api.DataTypeString,
		"binary":   api.DataTypeBinary,
		"numeric":  api.DataTypeDecimal,
		"bool":     api.DataTypeBoolean,
		"int8":     api.DataTypeByte,
		"unknown":  api.DataType("Unsupported DataType: unknown"),
	}

	for input, expect := range tests {
		t.Run(input, func(t *testing.T) {
			require.Equal(t, expect, toDataType(input))
		})
	}
}
