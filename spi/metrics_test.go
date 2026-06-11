package spi

import (
	"encoding/json"
	"expvar"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

func TestHandleStatz(t *testing.T) {
	metricKey := "custom:" + uuid.Must(uuid.NewV4()).String()
	metricValue := expvar.NewInt(metricKey)
	metricValue.Set(42)

	t.Run("json with invalid interval fallback", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/debug/statz?interval=not-a-duration&keys="+url.QueryEscape(metricKey), nil)
		writer := httptest.NewRecorder()

		HandleStatz(writer, req)

		require.Equal(t, http.StatusOK, writer.Code)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(writer.Body.Bytes(), &payload))
		require.EqualValues(t, 42, payload[metricKey])
	})

	t.Run("html renders included metric", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/debug/statz?format=html&keys="+url.QueryEscape(metricKey), nil)
		writer := httptest.NewRecorder()

		HandleStatz(writer, req)

		require.Equal(t, http.StatusOK, writer.Code)
		require.Contains(t, writer.Body.String(), "<table>")
		require.Contains(t, writer.Body.String(), metricKey)
		require.Contains(t, writer.Body.String(), "42")
	})
}
