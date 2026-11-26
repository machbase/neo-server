package tql

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/util/ssfs"
	"github.com/stretchr/testify/require"
)

func TestScriptWorkMountFollowsFileRoot(t *testing.T) {
	if instance == nil {
		Init()
		t.Cleanup(Deinit)
	}

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
