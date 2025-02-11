package metric

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Metric represents a single meter
type Metric interface {
	Add(value float64)
	String() string
	Reset()
	Aggregate(roll int, samples []Metric)
}

func newMetric(builder func() Metric, frames ...string) Metric {
	if len(frames) == 0 {
		return builder()
	}
	if len(frames) == 1 {
		return NewTimeseries(builder, frames[0])
	}
	mm := MultiTimeseries{}
	for _, frame := range frames {
		mm = append(mm, NewTimeseries(builder, frame))
	}
	sort.Slice(mm, func(i, j int) bool {
		a, b := mm[i], mm[j]
		return a.interval.Seconds()*float64(len(a.samples)) < b.interval.Seconds()*float64(len(b.samples))
	})
	return mm
}

// Counter is a simple counter metric
type Counter struct {
	count atomic.Int64
}

var _ Metric = (*Counter)(nil)

func NewCounter(frames ...string) Metric {
	return newMetric(func() Metric { return &Counter{} }, frames...)
}

func (c *Counter) Add(value float64) {
	c.count.Add(int64(value))
}

func (c *Counter) Reset() {
	c.count.Store(0)
}

func (c *Counter) Aggregate(roll int, samples []Metric) {
	var total int64
	for _, sample := range samples {
		total += sample.(*Counter).count.Load()
	}
	c.count.Store(total)
}

func (c *Counter) String() string {
	return fmt.Sprintf("%d", c.count.Load())
}

func (c *Counter) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type  string `json:"type"`
		Count int64  `json:"count"`
	}{
		Type:  "c",
		Count: c.count.Load(),
	})
}

func (c *Counter) Value() int64 {
	return c.count.Load()
}

// Gauge is a simple gauge metric
type Gauge struct {
	sync.Mutex
	value float64
	sum   float64
	min   float64
	max   float64
	count int
}

var _ Metric = (*Gauge)(nil)

func NewGauge(frames ...string) Metric {
	return newMetric(func() Metric { return &Gauge{} }, frames...)
}

func (g *Gauge) Add(value float64) {
	g.Lock()
	defer g.Unlock()
	if value < g.min || g.count == 0 {
		g.min = value
	}
	if value > g.max || g.count == 0 {
		g.max = value
	}
	g.value = value
	g.sum += value
	g.count++
}

func (g *Gauge) Reset() {
	g.Lock()
	defer g.Unlock()
	g.value = 0
	g.sum = 0
	g.min = 0
	g.max = 0
	g.count = 0
}

func (g *Gauge) Aggregate(roll int, samples []Metric) {
	g.Reset()
	g.Lock()
	defer g.Unlock()
	for i := len(samples) - 1; i >= 0; i-- {
		s := samples[i].(*Gauge)
		s.Lock()
		if s.count == 0 {
			s.Unlock()
			continue
		}
		if s.min < g.min || g.count == 0 {
			g.min = s.min
		}
		if s.max > g.max || g.count == 0 {
			g.max = s.max
		}
		g.count += s.count
		g.sum += s.sum
		g.value = s.value
		s.Unlock()
	}
}

func (g *Gauge) String() string {
	return strconv.FormatFloat(g.value, 'g', -1, 64)
}

func (g *Gauge) MarshalJSON() ([]byte, error) {
	g.Lock()
	defer g.Unlock()
	avg := float64(0)
	if g.count > 0 {
		avg = g.sum / float64(g.count)
	}
	return json.Marshal(struct {
		Type  string  `json:"type"`
		Value float64 `json:"value"`
		Avg   float64 `json:"avg"`
		Min   float64 `json:"min"`
		Max   float64 `json:"max"`
	}{
		Type:  "g",
		Value: g.value,
		Avg:   avg,
		Min:   g.min,
		Max:   g.max,
	})
}

func (g *Gauge) Value() map[string]float64 {
	g.Lock()
	defer g.Unlock()
	avg := float64(0)
	if g.count > 0 {
		avg = g.sum / float64(g.count)
	}
	return map[string]float64{
		"avg": avg,
		"min": g.min,
		"max": g.max,
	}
}

const maxBins = 100

type Bin struct {
	value float64
	count float64
}

// Histogram is a histogram metric
type Histogram struct {
	sync.Mutex
	bins  []Bin
	total float64
}

var _ Metric = (*Histogram)(nil)

func NewHistogram(frames ...string) Metric {
	return newMetric(func() Metric { return &Histogram{} }, frames...)
}

func (h *Histogram) Reset() {
	h.Lock()
	defer h.Unlock()
	h.bins = nil
	h.total = 0
}

func (h *Histogram) Add(value float64) {
	h.Lock()
	defer func() {
		h.trim()
		h.Unlock()
	}()

	h.total += 1
	newBin := Bin{value: value, count: 1}
	for i := range h.bins {
		if h.bins[i].value > value {
			h.bins = append(h.bins[:i], append([]Bin{newBin}, h.bins[i:]...)...)
			return
		}
	}
	h.bins = append(h.bins, newBin)
}

func (h *Histogram) trim() {
	for len(h.bins) > maxBins {
		d := float64(0)
		i := 0
		for j := 1; j < len(h.bins); j++ {
			if dv := h.bins[j].value - h.bins[j-1].value; dv < d || j == 1 {
				d = dv
				i = j
			}
		}
		count := h.bins[i].count + h.bins[i-1].count
		merged := Bin{
			value: (h.bins[i].value*h.bins[i].count + h.bins[i-1].value*h.bins[i-1].count) / count,
			count: count,
		}
		h.bins = append(h.bins[:i-1], h.bins[i:]...)
		h.bins[i-1] = merged
	}
}

func (h *Histogram) bin(q float64) Bin {
	count := q * float64(h.total)
	for i := range h.bins {
		count -= h.bins[i].count
		if count <= 0 {
			return h.bins[i]
		}
	}
	return Bin{}
}

func (h *Histogram) Quantile(q float64) float64 {
	h.Lock()
	defer h.Unlock()
	return h.bin(q).value
}

func (h *Histogram) quantile(q float64) float64 {
	return h.bin(q).value
}

func (h *Histogram) Aggregate(roll int, samples []Metric) {
	h.Lock()
	defer h.Unlock()
	alpha := 2.0 / float64(len(samples)+1)
	h.total = 0
	for i := range h.bins {
		h.bins[i].count = float64(h.bins[i].count) * math.Pow(1-alpha, float64(roll))
		h.total += h.bins[i].count
	}
}

func (h *Histogram) String() string {
	return fmt.Sprintf(`{"p50": %g, "p90": %g, "p99": %g}`, h.quantile(0.5), h.quantile(0.9), h.quantile(0.99))
}

func (h *Histogram) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type string  `json:"type"`
		P50  float64 `json:"p50"`
		P90  float64 `json:"p90"`
		P99  float64 `json:"p99"`
	}{
		Type: "h",
		P50:  h.quantile(0.5),
		P90:  h.quantile(0.9),
		P99:  h.quantile(0.99),
	})
}

func (h *Histogram) Value() map[string]float64 {
	return map[string]float64{
		"p50": h.quantile(0.5),
		"p90": h.quantile(0.9),
		"p99": h.quantile(0.99),
	}
}

// Timeseries is a timeseries metric
type Timeseries struct {
	sync.Mutex
	now      time.Time
	interval time.Duration
	total    Metric
	samples  []Metric
}

var _ Metric = (*Timeseries)(nil)

func NewTimeseries(builder func() Metric, frame string) *Timeseries {
	var totalNum int
	var intervalNum int
	var totalUnit rune
	var intervalUnit rune

	units := map[rune]time.Duration{
		's': time.Second,
		'm': time.Minute,
		'h': time.Hour,
		'd': 24 * time.Hour,
		'w': 7 * 24 * time.Hour,
		'M': 30 * 24 * time.Hour,
		'y': 365 * 24 * time.Hour,
	}
	fmt.Sscanf(frame, "%d%c%d%c", &totalNum, &totalUnit, &intervalNum, &intervalUnit)
	interval := units[intervalUnit] * time.Duration(intervalNum)
	if interval == 0 {
		interval = time.Minute
	}
	totalDuration := units[totalUnit] * time.Duration(totalNum)
	if totalDuration == 0 {
		totalDuration = interval * 15
	}
	n := int(totalDuration / interval)
	samples := make([]Metric, n)
	for i := range samples {
		samples[i] = builder()
	}
	totalMetric := builder()

	return &Timeseries{
		interval: interval,
		total:    totalMetric,
		samples:  samples,
	}
}

func (ts *Timeseries) Reset() {
	ts.Lock()
	defer ts.Unlock()
	ts.reset()
}

func (ts *Timeseries) Interval() time.Duration {
	return ts.interval
}

func (ts *Timeseries) Samples() []Metric {
	return ts.samples
}

func (ts *Timeseries) reset() {
	ts.total.Reset()
	for _, s := range ts.samples {
		s.Reset()
	}
}

func (ts *Timeseries) Aggregate(roll int, samples []Metric) {} // noop

// It is a variable so that it can be replaced in tests
var nowFunc func() time.Time = time.Now

func (ts *Timeseries) roll() {
	t := nowFunc()
	roll := int(t.Round(ts.interval).Sub(ts.now.Round(ts.interval)) / ts.interval)
	ts.now = t
	n := len(ts.samples)
	if roll <= 0 {
		return
	}
	if roll >= len(ts.samples) {
		ts.reset()
	} else {
		for i := 0; i < roll; i++ {
			tmp := ts.samples[n-1]
			for j := n - 1; j > 0; j-- {
				ts.samples[j] = ts.samples[j-1]
			}
			ts.samples[0] = tmp
			ts.samples[0].Reset()
		}
		ts.total.Aggregate(roll, ts.samples)
	}
}

func (ts *Timeseries) Add(value float64) {
	ts.Lock()
	defer ts.Unlock()
	ts.roll()
	ts.total.Add(value)
	ts.samples[0].Add(value)
}

func (ts *Timeseries) String() string {
	ts.Lock()
	defer ts.Unlock()
	ts.roll()
	return ts.total.String()
}

func (ts *Timeseries) MarshalJSON() ([]byte, error) {
	ts.Lock()
	defer ts.Unlock()
	ts.roll()
	return json.Marshal(struct {
		Interval float64  `json:"interval"`
		Total    Metric   `json:"total"`
		Samples  []Metric `json:"samples"`
	}{
		Interval: ts.interval.Seconds(),
		Total:    ts.total,
		Samples:  ts.samples,
	})
}

func (ts *Timeseries) Value() any {
	ts.Lock()
	defer ts.Unlock()
	ts.roll()
	switch m := ts.total.(type) {
	case *Counter:
		return m.Value()
	case *Gauge:
		return m.Value()
	case *Histogram:
		return m.Value()
	}
	return nil
}

// MultiTimeseries is a metric that aggregates multiple metrics
type MultiTimeseries []*Timeseries

var _ Metric = (*MultiTimeseries)(nil)

func (mm MultiTimeseries) Add(value float64) {
	for _, m := range mm {
		m.Add(value)
	}
}

func (mm MultiTimeseries) Reset() {} // noop

func (mm MultiTimeseries) Aggregate(roll int, samples []Metric) {} // noop

func (mm MultiTimeseries) String() string {
	return mm[len(mm)-1].String()
}

func (mm MultiTimeseries) MarshalJSON() ([]byte, error) {
	ret := []any{}
	for _, m := range mm {
		b, _ := json.Marshal(m)
		var v any
		json.Unmarshal(b, &v)
		ret = append(ret, v)
	}

	return json.Marshal(map[string]any{
		"metrics": ret,
	})
}

func (mm MultiTimeseries) Value() any {
	if len(mm) == 0 {
		return nil
	}
	return mm[0].Value()
}
