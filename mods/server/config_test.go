package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/stretchr/testify/require"
)

func TestServerGetConfig(t *testing.T) {
	s := &Server{}
	require.Equal(t, string(DefaultFallbackConfig), s.GetConfig())
}

func TestServerCheckRewriteMachbaseConf(t *testing.T) {
	t.Run("match", func(t *testing.T) {
		s := &Server{
			log: logging.GetLog("test"),
			Config: Config{
				Machbase: MachbaseConfig{PORT_NO: 5656, BIND_IP_ADDRESS: "127.0.0.1"},
			},
		}
		confPath := writeMachbaseConfFile(t, "# comment\nPORT_NO = 5656\nBIND_IP_ADDRESS = 127.0.0.1\nOTHER = keep\n")

		rewrite, err := s.checkRewriteMachbaseConf(confPath)
		require.NoError(t, err)
		require.False(t, rewrite)
	})

	t.Run("port mismatch", func(t *testing.T) {
		s := &Server{
			log: logging.GetLog("test"),
			Config: Config{
				Machbase: MachbaseConfig{PORT_NO: 7777, BIND_IP_ADDRESS: "127.0.0.1"},
			},
		}
		confPath := writeMachbaseConfFile(t, "PORT_NO = 5656\nBIND_IP_ADDRESS = 127.0.0.1\n")

		rewrite, err := s.checkRewriteMachbaseConf(confPath)
		require.NoError(t, err)
		require.True(t, rewrite)
	})

	t.Run("bind mismatch", func(t *testing.T) {
		s := &Server{
			log: logging.GetLog("test"),
			Config: Config{
				Machbase: MachbaseConfig{PORT_NO: 5656, BIND_IP_ADDRESS: "0.0.0.0"},
			},
		}
		confPath := writeMachbaseConfFile(t, "PORT_NO = 5656\nBIND_IP_ADDRESS = 127.0.0.1\n")

		rewrite, err := s.checkRewriteMachbaseConf(confPath)
		require.NoError(t, err)
		require.True(t, rewrite)
	})

	t.Run("missing file", func(t *testing.T) {
		s := &Server{}
		_, err := s.checkRewriteMachbaseConf(filepath.Join(t.TempDir(), "missing.conf"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "machbase.conf not available")
	})
}

func TestServerRewriteMachbaseConf(t *testing.T) {
	s := &Server{
		Config: Config{
			Machbase: MachbaseConfig{PORT_NO: 7777, BIND_IP_ADDRESS: "0.0.0.0"},
		},
	}
	confPath := writeMachbaseConfFile(t, "# preserved\nPORT_NO = 5656\nBIND_IP_ADDRESS = 127.0.0.1\nTRACE_LOG = 2\n")

	err := s.rewriteMachbaseConf(confPath)
	require.NoError(t, err)

	body, err := os.ReadFile(confPath)
	require.NoError(t, err)
	require.Equal(t, "# preserved\nPORT_NO = 7777\nBIND_IP_ADDRESS = 0.0.0.0\nTRACE_LOG = 2", string(body))
}

func writeMachbaseConfFile(t *testing.T, content string) string {
	t.Helper()
	confPath := filepath.Join(t.TempDir(), "machbase.conf")
	require.NoError(t, os.WriteFile(confPath, []byte(content), 0o644))
	return confPath
}

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
	require.EqualValues(t, 8192, base.HANDLE_LIMIT)

	fog := DefaultMachbaseConfig(PresetFog)
	require.EqualValues(t, 4, fog.TAG_PARTITION_COUNT)
	require.EqualValues(t, 16*1024*1024, fog.TAG_DATA_PART_SIZE)
	require.EqualValues(t, 64*1024*1024*1024, fog.PROCESS_MAX_SIZE)
	require.EqualValues(t, 31, fog.TAG_CACHE_ENABLE)

	edge := DefaultMachbaseConfig(PresetEdge)
	require.EqualValues(t, 1, edge.TAG_PARTITION_COUNT)
	require.EqualValues(t, 1024*1024, edge.TAG_DATA_PART_SIZE)
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
