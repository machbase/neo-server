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
