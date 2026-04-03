package util_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/stretchr/testify/require"
)

func TestMkDirIfNotExistsModeCreatesNestedDirectories(t *testing.T) {
	baseDir := t.TempDir()
	targetDir := filepath.Join(baseDir, "alpha", "beta", "gamma")

	err := util.MkDirIfNotExistsMode(targetDir, 0750)
	require.NoError(t, err)

	info, err := os.Stat(targetDir)
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

func TestMkDirIfNotExistsModeAcceptsExistingDirectories(t *testing.T) {
	baseDir := t.TempDir()
	targetDir := filepath.Join(baseDir, "existing", "child")
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	err := util.MkDirIfNotExists(targetDir)
	require.NoError(t, err)

	info, err := os.Stat(targetDir)
	require.NoError(t, err)
	require.True(t, info.IsDir())
}
