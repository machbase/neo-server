package metric

import (
	"encoding/json"
	"sync"
	"time"
)

func NewTimer() *Timer {
	return &Timer{}
}

func NewTimerWithValue(v *TimerValue) *Timer {
	return &Timer{
		samples:     v.Samples,
		sumDuration: v.Sum,
		minDuration: v.Min,
		maxDuration: v.Max,
	}
}

type Timer struct {
	sync.Mutex
	samples     int64
	sumDuration time.Duration
	minDuration time.Duration
	maxDuration time.Duration
	derivers    []Deriver
}

var _ Producer = (*Timer)(nil)

func (t *Timer) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Produce(false))
}

func (t *Timer) UnmarshalJSON(data []byte) error {
	tv := &TimerValue{}
	if err := json.Unmarshal(data, tv); err != nil {
		return err
	}

	t.samples = tv.Samples
	t.sumDuration = tv.Sum
	t.minDuration = tv.Min
	t.maxDuration = tv.Max
	return nil
}

func (t *Timer) WithDerivers(derivers ...Deriver) *Timer {
	t.derivers = append(t.derivers, derivers...)
	return t
}

func (t *Timer) Derivers() []Deriver {
	return t.derivers
}

func (t *Timer) String() string {
	b, _ := json.Marshal(t.Produce(false))
	return string(b)
}

func (t *Timer) Value() time.Duration {
	t.Lock()
	defer t.Unlock()
	if t.samples == 0 {
		return 0
	}
	return time.Duration(int64(t.sumDuration) / t.samples)
}

func (t *Timer) Produce(reset bool) Value {
	t.Lock()
	defer t.Unlock()
	ret := &TimerValue{
		Samples: t.samples,
		Sum:     t.sumDuration,
		Min:     t.minDuration,
		Max:     t.maxDuration,
	}
	if reset {
		t.samples = 0
		t.sumDuration = 0
		t.minDuration = 0
		t.maxDuration = 0
	}
	return ret
}

type TimerMarker struct {
	t     *Timer
	start time.Time
}

var _ Marker = (*TimerMarker)(nil)

func (w *TimerMarker) Mark() {
	w.t.Mark(time.Since(w.start))
}

func (t *Timer) New() Marker {
	return &TimerMarker{t: t, start: nowFunc()}
}

func (t *Timer) Add(v float64) {
	t.Mark(time.Duration(v))
}

func (t *Timer) Mark(d time.Duration) {
	t.Lock()
	defer t.Unlock()
	if t.samples == 0 {
		t.minDuration = d
		t.maxDuration = d
	}
	if d < t.minDuration {
		t.minDuration = d
	}
	if d > t.maxDuration {
		t.maxDuration = d
	}
	t.sumDuration += d
	t.samples++
}

type TimerValue struct {
	Samples int64         `json:"samples"`
	Sum     time.Duration `json:"sum"`
	Min     time.Duration `json:"min"`
	Max     time.Duration `json:"max"`
	// Optional derived values, such as moving averages
	DerivedValues map[string]Value `json:"derived,omitempty"`
}

func (tp TimerValue) String() string {
	b, _ := json.Marshal(tp)
	return string(b)
}

func (cp *TimerValue) SetDerivedValue(name string, value Value) {
	if cp.DerivedValues == nil {
		cp.DerivedValues = make(map[string]Value)
	}
	cp.DerivedValues[name] = value
}
