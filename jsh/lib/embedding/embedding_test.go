package embedding

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dop251/goja"
	service "github.com/machbase/neo-server/v8/mods/embedding"
	"github.com/stretchr/testify/require"
)

func TestJSHExternalEmbedding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"index":0,"embedding":[1,0,0]}]}`))
	}))
	defer server.Close()
	require.NoError(t, service.Configure(service.Config{
		DefaultProvider: "external", ExternalURL: server.URL,
		ExternalModel: "mock", ExternalSpace: "mock-space", ExternalDimension: 3,
	}))
	rt := goja.New()
	module := rt.NewObject()
	exports := rt.NewObject()
	require.NoError(t, module.Set("exports", exports))
	Module(context.Background(), rt, module)
	require.NoError(t, rt.Set("embedding", exports))
	value, err := rt.RunString(`embedding.embed("pump").then(result => result.vector[0])`)
	require.NoError(t, err)
	promise, ok := value.Export().(*goja.Promise)
	require.True(t, ok)
	require.Equal(t, goja.PromiseStateFulfilled, promise.State())
	require.Equal(t, int64(1), promise.Result().ToInteger())
}
