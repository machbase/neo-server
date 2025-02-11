package metric

import (
	"encoding/json"
	"expvar"
	"fmt"
	"time"
)

type ExpVarValueType interface {
	string | float64 | int | int64 | time.Duration
}

type ExpVarUnit string

const (
	UnitNone    ExpVarUnit = ""
	UnitBytes   ExpVarUnit = "bytes"
	UnitCount   ExpVarUnit = "count"
	UnitNanoSec ExpVarUnit = "ns"
)

type ExpVar interface {
	Key() string
	Metric() Metric
	MetricType() string
	ValueType() string
}

type ExpVarMetric[T ExpVarValueType] struct {
	key       string
	valueType string
	raw       Metric
	rawType   string
}

var _ ExpVar = (*ExpVarMetric[int])(nil)
var _ expvar.Var = (*ExpVarMetric[int])(nil)

func NewExpVarIntCounter(key string, frames ...string) *ExpVarMetric[int64] {
	metric := NewCounter(frames...)
	ret := &ExpVarMetric[int64]{key: key, valueType: "i", raw: metric, rawType: "c"}
	expvar.Publish(key, ret)
	return ret
}

func NewExpVarIntGauge(key string, frames ...string) *ExpVarMetric[int64] {
	metric := NewGauge(frames...)
	ret := &ExpVarMetric[int64]{key: key, valueType: "i", raw: metric, rawType: "g"}
	expvar.Publish(key, ret)
	return ret
}

func NewExpVarDurationGauge(key string, frames ...string) *ExpVarMetric[time.Duration] {
	metric := NewGauge(frames...)
	ret := &ExpVarMetric[time.Duration]{key: key, valueType: "dur", raw: metric, rawType: "g"}
	expvar.Publish(key, ret)
	return ret
}

func NewExpVarDurationHistogram(key string, frames ...string) *ExpVarMetric[time.Duration] {
	metric := NewHistogram(frames...)
	ret := &ExpVarMetric[time.Duration]{key: key, valueType: "dur", raw: metric, rawType: "h"}
	expvar.Publish(key, ret)
	return ret
}

func (m *ExpVarMetric[T]) Key() string                  { return m.key }
func (m *ExpVarMetric[T]) Metric() Metric               { return m.raw }
func (m *ExpVarMetric[T]) MetricType() string           { return m.rawType }
func (m *ExpVarMetric[T]) ValueType() string            { return m.valueType }
func (m *ExpVarMetric[T]) String() string               { return m.raw.String() }
func (m *ExpVarMetric[t]) MarshalJSON() ([]byte, error) { return json.Marshal(m.raw) }

func (m *ExpVarMetric[T]) Quantile(p float64) float64 {
	if m.raw != nil {
		if h, ok := m.raw.(*Histogram); ok {
			return h.Quantile(p)
		} else if m, ok := m.raw.(MultiTimeseries); ok {
			if h, ok := m[0].total.(*Histogram); ok {
				return h.Quantile(p)
			}
		}
	}
	return 0
}

func (m *ExpVarMetric[T]) Value() T {
	if m.raw != nil {
		switch raw := m.raw.(type) {
		case *Counter:
			if v, ok := any(raw.count.Load()).(T); ok {
				return v
			}
		case *Gauge:
			switch m.valueType {
			case "dur":
				if v, ok := any(time.Duration(raw.value)).(T); ok {
					return v
				}
			case "i":
				if v, ok := any(int64(raw.value)).(T); ok {
					return v
				}
			}
		case *Histogram:
			switch m.valueType {
			case "dur":
				if v, ok := any(time.Duration(raw.total)).(T); ok {
					return v
				}
			case "i":
				if v, ok := any(int64(raw.total)).(T); ok {
					return v
				}
			}
		case *Timeseries:
			return metricValue(raw.total, m.valueType).(T)
		case MultiTimeseries:
			return metricValue(raw[0].total, m.valueType).(T)
		}
	}
	panic(fmt.Sprintf("invalid metric type %s, %T", m.key, m.Metric))
}

func (m *ExpVarMetric[T]) Add(value T) {
	if m.raw != nil {
		metricAdd(m.raw, value)
	} else if val := expvar.Get(m.key); val != nil {
		switch v := val.(type) {
		case Metric:
			metricAdd(v, value)
		case *expvar.String:
			val := any(value)
			if str, ok := val.(string); ok {
				v.Set(str)
			} else {
				v.Set(fmt.Sprintf("%v", value))
			}
		}
	}
}

func metricValue(m Metric, t string) any {
	switch raw := m.(type) {
	case *Counter:
		return raw.count.Load()
	case *Gauge:
		v := raw.Value()["avg"]
		switch t {
		case "i":
			return int64(v)
		case "dur":
			return time.Duration(int64(v))
		default:
			return v
		}
	case *Histogram:
		return raw.Value()
	case MultiTimeseries:
		return metricValue(raw[0].total, t)
	}
	panic(fmt.Sprintf("invalid metric type %T", m))
}

func metricAdd[T ExpVarValueType](m Metric, value T) {
	var val any = value
	switch v := val.(type) {
	case string:
	case float64:
		m.Add(v)
	case time.Duration:
		m.Add(float64(v))
	case int:
		m.Add(float64(v))
	case int64:
		m.Add(float64(v))
	case uint64:
		m.Add(float64(v))
	default:
		panic(fmt.Sprintf("invalid metric type %T", value))
	}
}
