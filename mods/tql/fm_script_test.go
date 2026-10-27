package tql

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/machbase/neo-client/v2/api"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
	"github.com/stretchr/testify/require"
)

// The process working directory differs from the configured file root, as it
// does for a Windows service started in C:\Windows\System32 (machbase/neo#1569).
func TestScriptWorkMountFollowsFileRoot(t *testing.T) {
	fileRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(fileRoot, "marker-1569.txt"), []byte("ok"), 0644))

	prevFs := ssfs.Default()
	serverFs, err := ssfs.NewServerSideFileSystem([]string{"/=" + fileRoot})
	require.NoError(t, err)
	ssfs.SetDefault(serverFs)
	t.Cleanup(func() { ssfs.SetDefault(prevFs) })

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	output := &bytes.Buffer{}
	task := NewTaskContext(ctx)
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

func TestScriptWorkDir(t *testing.T) {
	prevFs := ssfs.Default()
	t.Cleanup(func() { ssfs.SetDefault(prevFs) })

	fileRoot := t.TempDir()
	tests := []struct {
		name   string
		dirs   []string
		expect string
	}{
		{name: "no_server_fs", dirs: nil, expect: "."},
		{name: "no_root_mount", dirs: []string{"/data=" + fileRoot}, expect: "."},
		{name: "root_mount", dirs: []string{"/=" + fileRoot}, expect: fileRoot},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.dirs == nil {
				ssfs.SetDefault(nil)
			} else {
				serverFs, err := ssfs.NewServerSideFileSystem(tc.dirs)
				require.NoError(t, err)
				ssfs.SetDefault(serverFs)
			}
			require.Equal(t, tc.expect, scriptWorkDir())
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
