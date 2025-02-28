package metric

import (
	"encoding/json"
	"expvar"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func mockTime(sec int) func() time.Time {
	return func() time.Time {
		return time.Date(2025, 01, 8, 9, 0, sec, 0, time.UTC)
	}
}

type H map[string]interface{}

func mapToJSON(m map[string]any) string {
	ret, _ := json.Marshal(m)
	return string(ret)
}

func ToJSON(m Metric) string {
	ret, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return string(ret)
}

func TestCounter(t *testing.T) {
	c := NewCounter()
	expect := func(counter int64) string {
		return mapToJSON(H{"type": "c", "count": counter})
	}
	require.JSONEq(t, expect(0), ToJSON(c))
	c.Add(1)
	require.JSONEq(t, expect(1), ToJSON(c))
	c.Add(10)
	require.JSONEq(t, expect(11), ToJSON(c))
	c.Reset()
	require.JSONEq(t, expect(0), ToJSON(c))
}

func TestGauge(t *testing.T) {
	g := NewGauge()
	expect := func(value, min, max, avg float64) string {
		return mapToJSON(H{"type": "g", "value": value, "min": min, "max": max, "avg": avg})
	}
	require.JSONEq(t, expect(0, 0, 0, 0), ToJSON(g))
	g.Add(1)
	require.JSONEq(t, expect(1, 1, 1, 1), ToJSON(g))
	g.Add(5)
	require.JSONEq(t, expect(5, 1, 5, 3), ToJSON(g))
	g.Add(0)
	require.JSONEq(t, expect(0, 0, 5, 2), ToJSON(g))
	g.Add(10)
	require.JSONEq(t, expect(10, 0, 10, 4), ToJSON(g))
	g.Reset()
	require.JSONEq(t, expect(0, 0, 0, 0), ToJSON(g))
}

func TestHistogram(t *testing.T) {
	h := NewHistogram()
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
	hist := NewHistogram().(*Histogram)
	for i := 0; i < 10000; i++ {
		hist.Add(rand.Float64() * 10)
	}

	if v := hist.quantile(0.5); math.Abs(v-5) > 0.5 {
		t.Fatalf("expected 5, got %f", v)
	}

	if v := hist.quantile(0.9); math.Abs(v-9) > 0.5 {
		t.Fatalf("expected 9, got %f", v)
	}

	if v := hist.quantile(0.99); math.Abs(v-10) > 0.5 {
		t.Fatalf("expected 10, got %f", v)
	}
}

func TestCounterTimeline(t *testing.T) {
	nowFunc = mockTime(0)
	c := NewCounter("3s1s")
	expect := func(total float64, samples ...float64) string {
		timeline := []any{}
		for _, s := range samples {
			timeline = append(timeline, H{"type": "c", "count": s})
		}
		return mapToJSON(H{
			"interval": 1,
			"total":    H{"type": "c", "count": total},
			"samples":  timeline,
		})
	}
	require.JSONEq(t, expect(0, 0, 0, 0), ToJSON(c))
	c.Add(1)
	require.JSONEq(t, expect(1, 1, 0, 0), ToJSON(c))
	nowFunc = mockTime(1)
	require.JSONEq(t, expect(1, 0, 1, 0), ToJSON(c))
	c.Add(5)
	require.JSONEq(t, expect(6, 5, 1, 0), ToJSON(c))
	nowFunc = mockTime(3)
	require.JSONEq(t, expect(5, 0, 0, 5), ToJSON(c))
	nowFunc = mockTime(10)
	require.JSONEq(t, expect(0, 0, 0, 0), ToJSON(c))
}

func TestGaugeTimeline(t *testing.T) {
	nowFunc = mockTime(0)
	g := NewGauge("3s1s")
	gauge := func(value, min, max, avg float64) H {
		return H{"type": "g", "value": value, "min": min, "max": max, "avg": avg}
	}
	expect := func(total H, samples ...H) H {
		return H{"interval": 1, "total": total, "samples": samples}
	}
	require.JSONEq(t, mapToJSON(expect(gauge(0, 0, 0, 0), gauge(0, 0, 0, 0), gauge(0, 0, 0, 0), gauge(0, 0, 0, 0))), ToJSON(g))
	g.Add(1)
	require.JSONEq(t, mapToJSON(expect(gauge(1, 1, 1, 1), gauge(1, 1, 1, 1), gauge(0, 0, 0, 0), gauge(0, 0, 0, 0))), ToJSON(g))
	nowFunc = mockTime(1)
	require.JSONEq(t, mapToJSON(expect(gauge(1, 1, 1, 1), gauge(0, 0, 0, 0), gauge(1, 1, 1, 1), gauge(0, 0, 0, 0))), ToJSON(g))
	g.Add(5)
	require.JSONEq(t, mapToJSON(expect(gauge(5, 1, 5, 3), gauge(5, 5, 5, 5), gauge(1, 1, 1, 1), gauge(0, 0, 0, 0))), ToJSON(g))
	nowFunc = mockTime(3)
	require.JSONEq(t, mapToJSON(expect(gauge(5, 5, 5, 5), gauge(0, 0, 0, 0), gauge(0, 0, 0, 0), gauge(5, 5, 5, 5))), ToJSON(g))
	nowFunc = mockTime(10)
	require.JSONEq(t, mapToJSON(expect(gauge(0, 0, 0, 0), gauge(0, 0, 0, 0), gauge(0, 0, 0, 0), gauge(0, 0, 0, 0))), ToJSON(g))
}

func TestHistogramTimeline(t *testing.T) {
	nowFunc = mockTime(0)
	hist := NewHistogram("3s1s")
	histogram := func(p50, p90, p99 float64) H {
		return H{"type": "h", "p50": p50, "p90": p90, "p99": p99}
	}
	expect := func(total H, samples ...H) H {
		return H{"interval": 1, "total": total, "samples": samples}
	}
	require.JSONEq(t, mapToJSON(expect(histogram(0, 0, 0), histogram(0, 0, 0), histogram(0, 0, 0), histogram(0, 0, 0))), ToJSON(hist))
	hist.Add(1)
	require.JSONEq(t, mapToJSON(expect(histogram(1, 1, 1), histogram(1, 1, 1), histogram(0, 0, 0), histogram(0, 0, 0))), ToJSON(hist))
	nowFunc = mockTime(1)
	require.JSONEq(t, mapToJSON(expect(histogram(1, 1, 1), histogram(0, 0, 0), histogram(1, 1, 1), histogram(0, 0, 0))), ToJSON(hist))
	hist.Add(3)
	hist.Add(5)
	require.JSONEq(t, mapToJSON(expect(histogram(3, 5, 5), histogram(3, 5, 5), histogram(1, 1, 1), histogram(0, 0, 0))), ToJSON(hist))
	nowFunc = mockTime(3)
	require.JSONEq(t, mapToJSON(expect(histogram(3, 5, 5), histogram(0, 0, 0), histogram(0, 0, 0), histogram(3, 5, 5))), ToJSON(hist))
	nowFunc = mockTime(10)
	require.JSONEq(t, mapToJSON(expect(histogram(0, 0, 0), histogram(0, 0, 0), histogram(0, 0, 0), histogram(0, 0, 0))), ToJSON(hist))
}

func TestMulti(t *testing.T) {
	m := NewCounter("10s1s", "30s5s")
	m.Add(5)
	if s := m.String(); s != `5` {
		t.Fatal(s)
	}
}

func TestExpVar(t *testing.T) {
	expvar.Publish("test:count", NewCounter())
	expvar.Publish("test:timeline", NewGauge("3s1s"))
	expvar.Get("test:count").(Metric).Add(1)
	expvar.Get("test:timeline").(Metric).Add(1)
	if s := expvar.Get("test:count").String(); s != `1` {
		t.Fatal(s)
	}
	if s := expvar.Get("test:timeline").String(); s != `1` {
		t.Fatal(s)
	}
}

func BenchmarkMetrics(b *testing.B) {
	b.Run("counter", func(b *testing.B) {
		c := &Counter{}
		for i := 0; i < b.N; i++ {
			c.Add(rand.Float64())
		}
	})
	b.Run("gauge", func(b *testing.B) {
		c := &Gauge{}
		for i := 0; i < b.N; i++ {
			c.Add(rand.Float64())
		}
	})
	b.Run("histogram", func(b *testing.B) {
		c := &Histogram{}
		for i := 0; i < b.N; i++ {
			c.Add(rand.Float64())
		}
	})
	b.Run("timeline/counter", func(b *testing.B) {
		c := NewCounter("10s1s")
		for i := 0; i < b.N; i++ {
			c.Add(rand.Float64())
		}
	})
	b.Run("timeline/gauge", func(b *testing.B) {
		c := NewGauge("10s1s")
		for i := 0; i < b.N; i++ {
			c.Add(rand.Float64())
		}
	})
	b.Run("timeline/histogram", func(b *testing.B) {
		c := NewHistogram("10s1s")
		for i := 0; i < b.N; i++ {
			c.Add(rand.Float64())
		}
	})
}
