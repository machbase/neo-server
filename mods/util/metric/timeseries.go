package metric

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type TimeBin struct {
	Time   time.Time `json:"ts"`
	Value  Value     `json:"value,omitempty"`
	IsNull bool      `json:"isNull,omitempty"`
}

func (tv TimeBin) String() string {
	if ((any)(tv.Value)) == nil {
		return fmt.Sprintf(`{"ts":"%s",isNull:%t}`, tv.Time.In(timeZone).Format(time.DateTime), tv.IsNull)
	}
	return fmt.Sprintf(`{"ts":"%s","value":%s}`, tv.Time.In(timeZone).Format(time.DateTime), tv.Value.String())
}

func (tv TimeBin) MarshalJSON() ([]byte, error) {
	if ((any)(tv.Value)) == nil {
		return []byte(fmt.Sprintf(`{"ts":%d,"isNull":%t}`, tv.Time.UnixNano(), tv.IsNull)), nil
	} else {
		typ := fmt.Sprintf("%T", tv.Value)
		return []byte(fmt.Sprintf(`{"ts":%d,"type":%q,"value":%s}`, tv.Time.UnixNano(), typ, tv.Value.String())), nil
	}
}

func (tv *TimeBin) UnmarshalJSON(data []byte) error {
	var obj struct {
		Time   int64          `json:"ts"`
		Type   string         `json:"type,omitempty"`
		Value  map[string]any `json:"value,omitempty"`
		IsNull bool           `json:"isNull,omitempty"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	tv.Time = time.Unix(0, obj.Time).In(timeZone)
	tv.IsNull = obj.IsNull
	if tv.IsNull {
		return nil
	}
	switch obj.Type {
	case "*metric.CounterValue":
		tv.Value = &CounterValue{}
	case "*metric.GaugeValue":
		tv.Value = &GaugeValue{}
	case "*metric.HistogramValue":
		tv.Value = &HistogramValue{}
	case "*metric.MeterValue":
		tv.Value = &MeterValue{}
	case "*metric.TimerValue":
		tv.Value = &TimerValue{}
	default:
		return fmt.Errorf("unknown value type %s", obj.Type)
	}
	b, err := json.Marshal(obj.Value)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, tv.Value); err != nil {
		return err
	}
	return nil
}

type TimeSeries struct {
	sync.Mutex
	producer Producer
	lastTime time.Time // The last time the producer was updated
	data     []TimeBin
	interval time.Duration
	maxCount int
	meta     any // Optional metadata for the time series
	lsnr     func(TimeBin, any)
}

// If aggregator is nil, it will replace the last point with the new one.
// Otherwise, it will aggregate the new point with the last one when it falls within the same interval.
func NewTimeSeries(interval time.Duration, maxCount int, prod Producer) *TimeSeries {
	return &TimeSeries{
		producer: prod,
		data:     make([]TimeBin, 0, maxCount),
		interval: interval,
		maxCount: maxCount,
	}
}

func (ts *TimeSeries) roundTime(t time.Time) time.Time {
	return t.Add(ts.interval / 2).Round(ts.interval)
}

func (ts *TimeSeries) SetListener(listener func(TimeBin, any)) {
	ts.lsnr = listener
}

func (ts *TimeSeries) SetMeta(meta any) {
	ts.meta = meta
}

func (ts *TimeSeries) Meta() any {
	return ts.meta
}

func (ts *TimeSeries) String() string {
	ts.Lock()
	defer ts.Unlock()
	result := "["
	for i, d := range ts.data {
		if i > 0 {
			result += ","
		}
		result += d.String()
	}
	if len(ts.data) > 0 {
		result += ","
	}
	result += fmt.Sprintf(`{"ts":"%s","value":%v}`,
		ts.roundTime(ts.lastTime).In(timeZone).Format(time.DateTime),
		ts.producer.Produce(false))
	result += "]"
	return result
}

type Snapshot struct {
	Times    []time.Time
	Values   []Value
	Interval time.Duration
	MaxCount int
	Meta     any // Optional metadata that inherits from the TimeSeries meta
}

func (ss Snapshot) Field() (FieldInfo, bool) {
	if ss.Meta == nil {
		return FieldInfo{}, false
	}
	field, ok := ss.Meta.(FieldInfo)
	return field, ok
}

// Snapshot creates a snapshot of the current time series data.
// If snapshot is nil, it will create a new one.
// The snapshot will contain rounded times and values.
func (ts *TimeSeries) Snapshot() *Snapshot {
	ts.Lock()
	defer ts.Unlock()
	size := len(ts.data) + 1
	snapshot := &Snapshot{
		Times:    make([]time.Time, size),
		Values:   make([]Value, size),
		Interval: ts.interval,
		MaxCount: ts.maxCount,
		Meta:     ts.meta,
	}
	for i, d := range ts.data {
		snapshot.Times[i] = d.Time
		snapshot.Values[i] = d.Value
	}
	// snapshot for the current funnel state
	lt := ts.roundTime(ts.lastTime)
	lv := ts.producer.Produce(false)
	snapshot.Times[size-1], snapshot.Values[size-1] = lt, lv
	return snapshot
}

func (ts *TimeSeries) Last() (time.Time, Value) {
	times, values := ts.LastN(1)
	if len(times) == 0 {
		return time.Time{}, *new(Value)
	}
	return times[0], values[0]
}

func (ts *TimeSeries) LastN(n int) ([]time.Time, []Value) {
	ts.Lock()
	defer ts.Unlock()
	if len(ts.data) == 0 || n <= 0 {
		return nil, nil
	}

	lt := ts.roundTime(ts.lastTime)
	lv := ts.producer.Produce(false)
	if n == 1 {
		return []time.Time{lt}, []Value{lv}
	}

	offset := len(ts.data) - n - 1
	if offset < 0 {
		offset = 0
	}

	sub := ts.data[offset:]
	times := make([]time.Time, len(sub)+1)
	values := make([]Value, len(sub)+1)
	for i := range sub {
		times[i], values[i] = sub[i].Time, sub[i].Value
	}
	times[len(times)-1], values[len(values)-1] = lt, lv
	return times, values
}

func (ts *TimeSeries) After(t time.Time) ([]time.Time, []Value) {
	ts.Lock()
	defer ts.Unlock()
	idx := -1
	tick := t.UnixNano() - (int64(ts.interval) / 2)
	for i, d := range ts.data {
		if d.Time.UnixNano() >= tick {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil, nil
	}
	sub := ts.data[idx:]
	times := make([]time.Time, len(sub)+1)
	values := make([]Value, len(sub)+1)
	for i := range sub {
		times[i], values[i] = sub[i].Time, sub[i].Value
	}
	lt := ts.roundTime(ts.lastTime)
	lv := ts.producer.Produce(false)
	times[len(times)-1], values[len(values)-1] = lt, lv
	return times, values
}

func (ts *TimeSeries) Add(v float64) {
	ts.Lock()
	defer ts.Unlock()
	ts.add(nowFunc(), v)
}

func (ts *TimeSeries) AddTime(t time.Time, v float64) {
	ts.Lock()
	defer ts.Unlock()
	ts.add(t, v)
}

func (ts *TimeSeries) add(tm time.Time, val float64) {
	roll := ts.IntervalBetween(ts.lastTime, tm)

	if roll <= 0 || ts.lastTime.IsZero() {
		ts.lastTime = tm
		ts.producer.Add(val)
		return
	}

	p := ts.producer.Produce(true)
	tb := TimeBin{Time: ts.roundTime(ts.lastTime), Value: p, IsNull: p == nil}
	if ts.lsnr != nil {
		ts.lsnr(tb, ts.meta)
	}
	ts.data = append(ts.data, tb)
	ts.lastTime = tm
	ts.producer.Add(val)
	roll--

	// Reset if the gap is too large
	if roll >= ts.maxCount-1 {
		ts.data = ts.data[:0]
		return
	}
	// Remove the oldest data if we exceed maxCount
	if len(ts.data) > ts.maxCount-1 {
		ts.data = ts.data[len(ts.data)-(ts.maxCount-1):]
	}

	last := ts.data[len(ts.data)-1]
	for i := range roll {
		// Fill in the gaps with empty data points
		emptyPoint := TimeBin{
			Time:   last.Time.Add(time.Duration(i+1) * ts.interval),
			IsNull: true,
		}
		ts.data = append(ts.data, emptyPoint)
		// Remove the oldest data if we exceed maxCount
		if len(ts.data) > ts.maxCount-1 {
			ts.data = ts.data[1:]
		}
	}
}

// IntervalBetween returns the number of intervals between two times.
// (later - prev) / ts.interval
func (ts *TimeSeries) IntervalBetween(prev, later time.Time) int {
	return int(ts.timeRound(later).Sub(ts.timeRound(prev)) / ts.interval)
}

func (ts *TimeSeries) timeRound(t time.Time) time.Time {
	return t.Truncate(ts.interval)
}

func (ts *TimeSeries) MarshalJSON() ([]byte, error) {
	ts.Lock()
	defer ts.Unlock()
	buf := &bytes.Buffer{}
	buf.WriteString(`{"data":[`)
	for i, d := range ts.data {
		if i > 0 {
			buf.WriteString(",")
		}
		dd, err := json.Marshal(d)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal time bin %d: %w", i, err)
		}
		buf.Write(dd)
	}
	buf.WriteString("]")
	buf.WriteString(fmt.Sprintf(`,"interval":%d`, ts.interval))
	buf.WriteString(fmt.Sprintf(`,"maxCount":%d`, ts.maxCount))
	buf.WriteString(fmt.Sprintf(`,"lastTime":%d`, ts.lastTime.UnixNano()))
	buf.WriteString(fmt.Sprintf(`,"type":"%T"`, ts.producer))
	buf.WriteString(`,"producer":`)
	pb, err := json.Marshal(ts.producer)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal producer: %w", err)
	}
	buf.Write(pb)
	buf.WriteString("}")
	return buf.Bytes(), nil
}

func (ts *TimeSeries) UnmarshalJSON(data []byte) error {
	ts.Lock()
	defer ts.Unlock()
	obj := struct {
		Data     []TimeBin      `json:"data"`
		LastTime int64          `json:"lastTime"`
		Interval int64          `json:"interval"`
		MaxCount int            `json:"maxCount"`
		Type     string         `json:"type"`
		Producer map[string]any `json:"producer,omitempty"`
	}{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	ts.data = obj.Data
	if obj.Interval > 0 {
		ts.interval = time.Duration(obj.Interval)
	}
	if obj.MaxCount > 0 {
		ts.maxCount = obj.MaxCount
	}
	ts.lastTime = time.Unix(0, obj.LastTime).In(timeZone)
	var producer Producer
	switch obj.Type {
	case "*metric.Meter":
		producer = &Meter{}
	case "*metric.Counter":
		producer = &Counter{}
	case "*metric.Gauge":
		producer = &Gauge{}
	case "*metric.Histogram":
		producer = &Histogram{}
	default:
		return fmt.Errorf("unknown producer type %s", obj.Type)
	}
	b, err := json.Marshal(obj.Producer)
	if err != nil {
		return fmt.Errorf("failed to marshal producer data: %w", err)
	}
	if err := producer.UnmarshalJSON(b); err != nil {
		return fmt.Errorf("failed to unmarshal producer: %w", err)
	}
	ts.producer = producer
	return nil
}

type MultiTimeSeries []*TimeSeries

func (mts MultiTimeSeries) Add(v float64) {
	for _, ts := range mts {
		ts.Add(v)
	}
}

func (mts MultiTimeSeries) AddTime(t time.Time, v float64) {
	for _, ts := range mts {
		ts.AddTime(t, v)
	}
}

func (mts MultiTimeSeries) String() string {
	if len(mts) == 0 {
		return "[]"
	}
	result := "["
	for i, ts := range mts {
		if i > 0 {
			result += ","
		}
		result += ts.String()
	}
	result += "]"
	return result
}
