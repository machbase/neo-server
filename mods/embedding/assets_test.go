package embedding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnsureFileVerifiesAndRepairsCache(t *testing.T) {
	const payload = "verified model asset"
	digest := sha256.Sum256([]byte(payload))
	wantHash := hex.EncodeToString(digest[:])
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = w.Write([]byte(payload))
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "model", "asset")
	require.NoError(t, ensureFile(context.Background(), path, server.URL, wantHash))
	require.NoError(t, ensureFile(context.Background(), path, server.URL, wantHash))
	require.Equal(t, 1, requests)
	require.NoError(t, os.WriteFile(path, []byte("corrupt"), 0644))
	require.NoError(t, ensureFile(context.Background(), path, server.URL, wantHash))
	require.Equal(t, 2, requests)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, payload, string(content))
}

func TestEnsureFileRejectsWrongHash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("wrong"))
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "asset")
	err := ensureFile(context.Background(), path, server.URL, "0000000000000000000000000000000000000000000000000000000000000000")
	require.ErrorContains(t, err, "SHA-256 mismatch")
	_, err = os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)
}
