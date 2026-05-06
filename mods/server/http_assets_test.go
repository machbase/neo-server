package server

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStaticFSWrapOpenAppliesPrefixAndFixedModTime(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "assets", "hello.txt"), []byte("hello"), 0o644))

	fixedTime := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	fsw := &StaticFSWrap{
		TrimPrefix:      "/web/",
		PrependRealPath: "/assets/",
		Base:            http.Dir(tempDir),
		FixedModTime:    fixedTime,
	}

	file, err := fsw.Open("hello.txt")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, file.Close()) })

	body, err := io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, "hello", string(body))
	require.Equal(t, fixedTime, file.(*staticFile).ModTime())

	stat, err := file.Stat()
	require.NoError(t, err)
	require.Equal(t, fixedTime, stat.ModTime())
}

func TestWrapAssetsFallbackToIndexHTML(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "index.html"), []byte("index page"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "app.js"), []byte("console.log('ok')"), 0o644))

	fs := WrapAssets(tempDir)

	assetFile, err := fs.Open("app.js")
	require.NoError(t, err)
	assetBody, err := io.ReadAll(assetFile)
	require.NoError(t, err)
	require.NoError(t, assetFile.Close())
	require.Equal(t, "console.log('ok')", string(assetBody))

	fallbackFile, err := fs.Open("missing/route")
	require.NoError(t, err)
	fallbackBody, err := io.ReadAll(fallbackFile)
	require.NoError(t, err)
	require.NoError(t, fallbackFile.Close())
	require.Equal(t, "index page", string(fallbackBody))
}

func TestGetAssetsServesKnownFileAndSpaFallback(t *testing.T) {
	fs := GetAssets("ui")

	assetFile, err := fs.Open("/vite.svg?cache=1")
	require.NoError(t, err)
	assetBody, err := io.ReadAll(assetFile)
	require.NoError(t, err)
	require.NoError(t, assetFile.Close())
	require.Contains(t, string(assetBody), "<svg")

	fallbackFile, err := fs.Open("dashboard/metrics")
	require.NoError(t, err)
	fallbackBody, err := io.ReadAll(fallbackFile)
	require.NoError(t, err)
	require.NoError(t, fallbackFile.Close())
	require.Contains(t, string(fallbackBody), "<!DOCTYPE html>")
}

func TestIsWellKnownFileType(t *testing.T) {
	require.True(t, isWellKnownFileType("app.js"))
	require.True(t, isWellKnownFileType("image.WEBP"))
	require.True(t, isWellKnownFileType("font.woff2"))
	require.False(t, isWellKnownFileType("README"))
	require.False(t, isWellKnownFileType("archive.tar.gz"))
	require.False(t, isWellKnownFileType("custom.bin"))
}
