package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/machbase/neo-client/v2/api"
	"github.com/stretchr/testify/require"
)

func TestExternalEmbedding(t *testing.T) {
	t.Setenv("NEO_TEST_EMBED_KEY", "test-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		var request struct {
			Model string `json:"model"`
			Input string `json:"input"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		require.Equal(t, "demo-model", request.Model)
		require.Equal(t, "펌프 점검", request.Input)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[0.25,-1,2]}]}`))
	}))
	defer server.Close()

	service := NewService(Config{
		ExternalURL: server.URL, ExternalModel: "demo-model",
		ExternalSpace: "demo-space", ExternalDimension: 3, ExternalAPIKeyEnv: "NEO_TEST_EMBED_KEY",
	})
	result, err := service.Embed(context.Background(), "펌프 점검", "external")
	require.NoError(t, err)
	require.Equal(t, api.Vector{0.25, -1, 2}, result.Vector)
	require.Equal(t, 3, result.Dimension)
	require.Equal(t, "demo-space", result.Space)
}

func TestExternalEmbeddingRejectsWrongDimension(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[1]}]}`))
	}))
	defer server.Close()
	service := NewService(Config{
		ExternalURL: server.URL, ExternalModel: "demo-model",
		ExternalSpace: "demo-space", ExternalDimension: 3,
	})
	_, err := service.Embed(context.Background(), "question", "external")
	require.ErrorContains(t, err, "dimension")
}
