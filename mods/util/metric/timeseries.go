package metric

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
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
	ts := tv.Time.UnixNano()
	if ((any)(tv.Value)) == nil {
		return []byte(fmt.Sprintf(`{"ts":%d,"isNull":%t}`, ts, tv.IsNull)), nil
	} else {
		typ := fmt.Sprintf("%T", tv.Value)
		return []byte(fmt.Sprintf(`{"ts":%d,"type":%q,"value":%s}`, ts, typ, tv.Value.String())), nil
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
	case "*metric.OdometerValue":
		tv.Value = &OdometerValue{}
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

func ToProduct(prd *Product, tb TimeBin, meta any) bool {
	mInfo, ok := meta.(SeriesInfo)
	if !ok {
		return false
	}
	*prd = Product{
		Name:        mInfo.MeasureName,
		Time:        tb.Time,
		Value:       tb.Value,
		IsNull:      tb.IsNull,
		SeriesID:    mInfo.SeriesID.ID(),
		SeriesTitle: mInfo.SeriesID.Title(),
		Period:      mInfo.SeriesID.Period(),
		Type:        mInfo.MeasureType.Name(),
		Unit:        mInfo.MeasureType.Unit(),
	}
	return true
}

func FromProduct(prd []Product) []TimeBin {
	ret := make([]TimeBin, len(prd))
	for i, p := range prd {
		ret[i] = TimeBin{
			Time:   p.Time,
			Value:  p.Value,
			IsNull: p.IsNull,
		}
	}
	return ret
}

type TimeSeries struct {
	sync.Mutex
	producer Producer
	lastTime time.Time // The last time the producer was updated
	data     []TimeBin
	interval time.Duration
	maxCount int
	meta     any // Optional metadata for the time series
	lsnr     func(Product)
}

// If aggregator is nil, it will replace the last point with the new one.
// Otherwise, it will aggregate the new point with the last one when it falls within the same interval.
func NewTimeSeries(interval time.Duration, maxCount int, prod Producer, opts ...TimeSeriesOption) *TimeSeries {
	ret := &TimeSeries{
		producer: prod,
		data:     make([]TimeBin, 0, maxCount),
		interval: interval,
		maxCount: maxCount,
	}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

type TimeSeriesOption func(*TimeSeries)

func WithListener(lsnr func(Product)) TimeSeriesOption {
	return func(ts *TimeSeries) {
		ts.lsnr = lsnr
	}
}

func WithMeta(meta any) TimeSeriesOption {
	return func(ts *TimeSeries) {
		ts.meta = meta
	}
}

func (ts *TimeSeries) roundTime(t time.Time) time.Time {
	return t.Add(ts.interval / 2).Round(ts.interval)
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

func (ts *TimeSeries) Interval() time.Duration {
	return ts.interval
}

func (ts *TimeSeries) MaxCount() int {
	return ts.maxCount
}

func (ts *TimeSeries) runDerivers(currentValue Value, preliminary bool) {
	derivers := ts.producer.Derivers()
	if len(derivers) == 0 {
		return
	}
	driving, ok := currentValue.(DerivingValue)
	if !ok {
		return
	}
	// Derive additional values
	for _, d := range derivers {
		var values []Value
		if ws := d.WindowSize(); ws > 0 {
			_, values = ts.lastN(d.WindowSize() + 1)
			if preliminary {
				values = values[1:]
			} else {
				values = values[0 : len(values)-1] // Exclude the last point which is the last one which is empty.
			}
		} else {
			_, values = ts.lastN(1)
		}
		dv := d.Derive(values)
		driving.SetDerivedValue(d.ID(), dv)
	}
}

func (ts *TimeSeries) LastBin() (TimeBin, any) {
	tm, val := ts.Last()
	tb := TimeBin{Time: tm, Value: val, IsNull: val == nil}
	return tb, ts.meta
}

func (ts *TimeSeries) Last() (time.Time, Value) {
	times, values := ts.LastN(1)
	if len(times) == 0 {
		return time.Time{}, nil
	}
	return times[0], values[0]
}

func (ts *TimeSeries) All() ([]time.Time, []Value) {
	return ts.LastN(0)
}

func (ts *TimeSeries) LastN(n int) ([]time.Time, []Value) {
	ts.Lock()
	defer ts.Unlock()
	times, values := ts.lastN(n)
	ts.runDerivers(values[len(values)-1], true)
	return times, values
}

func (ts *TimeSeries) lastN(n int) ([]time.Time, []Value) {
	lt := ts.roundTime(ts.lastTime)
	lv := ts.producer.Produce(false)
	if n == 1 {
		return []time.Time{lt}, []Value{lv}
	} else if n <= 0 || n > ts.maxCount {
		n = ts.maxCount
	}
	times := make([]time.Time, n)
	values := make([]Value, n)
	for i := range times {
		times[i] = lt.Add(-time.Duration(len(times)-i-1) * ts.interval)
		values[i] = nil
	}
	var offset int = 0
	if n > 0 {
		offset := len(ts.data) - n - 1 // -1 for the last point
		if offset < 0 {
			offset = 0 // keep at least one point before the last point
		}
	}
	tmIdx := 0
	for _, tb := range ts.data[offset:] {
		if tmIdx >= len(times)-1 {
			break
		}
		if tb.Time.Before(times[tmIdx]) {
			continue
		}
		for tb.Time.After(times[tmIdx]) {
			tmIdx++
			continue
		}
		values[tmIdx] = tb.Value
	}
	if times[len(times)-1].Equal(lt) {
		values[len(values)-1] = lv
	}
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
		if val == val { // not NaN
			ts.producer.Add(val)
		}
		return
	}

	p := ts.producer.Produce(true)
	tb := TimeBin{Time: ts.roundTime(ts.lastTime), Value: p, IsNull: p == nil}

	// Notify listener
	if ts.lsnr != nil {
		prd := Product{}
		if ok := ToProduct(&prd, tb, ts.meta); ok {
			ts.lsnr(prd)
		}
	}

	ts.data = append(ts.data, tb)
	ts.lastTime = tm
	if val == val { // not NaN
		ts.producer.Add(val)
	}
	roll--

	// Derive additional values
	ts.runDerivers(tb.Value, false)

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
	case "*metric.Odometer":
		producer = &Odometer{}
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

func (ts *TimeSeries) Restore(storage Storage, metricName string, series SeriesID) error {
	if data, err := storage.Load(series, metricName); err != nil {
		slog.Error("Failed to load time series", "metric", metricName, "series", series.ID(), "error", err)
	} else if len(data) > 0 {
		// if file is not exists, data will be nil
		ts.data = FromProduct(data)
		//
		// TODO: if the last data point is the same period as now,
		// restore the inflight TimeBin
		//
		ts.lastTime = data[len(data)-1].Time
	}

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
