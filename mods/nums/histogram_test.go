package nums_test

import (
	"encoding/json"
	"math"
	"math/rand"
	"testing"

	"github.com/machbase/neo-server/v8/mods/nums"
	"github.com/stretchr/testify/require"
)

func mapToJSON(m map[string]any) string {
	ret, _ := json.Marshal(m)
	return string(ret)
}

type H map[string]interface{}

func TestHistogram(t *testing.T) {
	h := nums.Histogram{MaxBins: 100}
	require.JSONEq(t, mapToJSON(H{"p50": 0.000000, "p90": 0.000000, "p99": 0.000000}), h.String())
	h.Add(1)
	require.JSONEq(t, mapToJSON(H{"p50": 1.000000, "p90": 1.000000, "p99": 1.000000}), h.String())
	for i := 2; i < 100; i++ {
		h.Add(float64(i))
	}
	require.JSONEq(t, mapToJSON(H{"p50": 50.000000, "p90": 90.000000, "p99": 99.000000}), h.String())
	h.Reset()
	require.JSONEq(t, mapToJSON(H{"p50": 0.000000, "p90": 0.000000, "p99": 0.000000}), h.String())
}

func TestHistogramNormalDist(t *testing.T) {
	hist := nums.Histogram{MaxBins: 100}
	for i := 0; i < 10000; i++ {
		hist.Add(rand.Float64() * 10)
	}

	if v := hist.Quantile(0.5); math.Abs(v-5) > 0.5 {
		t.Fatalf("expected 5, got %f", v)
	}

	if v := hist.Quantile(0.9); math.Abs(v-9) > 0.5 {
		t.Fatalf("expected 9, got %f", v)
	}

	if v := hist.Quantile(0.99); math.Abs(v-10) > 0.5 {
		t.Fatalf("expected 10, got %f", v)
	}
}
