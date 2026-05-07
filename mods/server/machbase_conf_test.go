package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMachbasePresetString(t *testing.T) {
	require.Equal(t, "none", PresetNone.String())
	require.Equal(t, "fog", PresetFog.String())
	require.Equal(t, "edge", PresetEdge.String())
	require.Equal(t, "none", MachbasePreset(999).String())
}

func TestDefaultMachbaseConfigPresets(t *testing.T) {
	base := DefaultMachbaseConfig(PresetNone)
	require.Equal(t, 5656, base.PORT_NO)
	require.Equal(t, "127.0.0.1", base.BIND_IP_ADDRESS)
	require.Equal(t, 0, base.TAG_PARTITION_COUNT)
	require.Equal(t, 4, base.STREAM_THREAD_COUNT)
	require.EqualValues(t, 8192, base.HANDLE_LIMIT)

	fog := DefaultMachbaseConfig(PresetFog)
	require.EqualValues(t, 4, fog.TAG_PARTITION_COUNT)
	require.EqualValues(t, 16*1024*1024, fog.TAG_DATA_PART_SIZE)
	require.EqualValues(t, 64*1024*1024*1024, fog.PROCESS_MAX_SIZE)
	require.EqualValues(t, 31, fog.TAG_CACHE_ENABLE)

	edge := DefaultMachbaseConfig(PresetEdge)
	require.EqualValues(t, 1, edge.TAG_PARTITION_COUNT)
	require.EqualValues(t, 1024*1024, edge.TAG_DATA_PART_SIZE)
	require.EqualValues(t, 32*1024*1024, edge.RS_CACHE_MAX_MEMORY_SIZE)
	require.EqualValues(t, 0, edge.STREAM_THREAD_COUNT)
	require.EqualValues(t, 4096, edge.HANDLE_LIMIT)
}

func TestApplyMachbaseConfig(t *testing.T) {
	conf := DefaultMachbaseConfig(PresetEdge)
	conf.PORT_NO = 7878
	conf.BIND_IP_ADDRESS = "0.0.0.0"
	conf.DBS_PATH = "/tmp/machbase/dbs"

	t.Run("write_config_file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "machbase.conf")
		err := applyMachbaseConfig(path, conf)
		require.NoError(t, err)

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		text := string(content)
		require.Contains(t, text, "PORT_NO = 7878")
		require.Contains(t, text, "BIND_IP_ADDRESS = 0.0.0.0")
		require.Contains(t, text, "DBS_PATH=/tmp/machbase/dbs")
	})

	t.Run("open_file_error", func(t *testing.T) {
		err := applyMachbaseConfig(t.TempDir(), conf)
		require.Error(t, err)
		require.ErrorContains(t, err, "config file open")
	})
}
