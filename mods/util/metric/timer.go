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
		sumDuration: v.SumDuration,
		minDuration: v.MinDuration,
		maxDuration: v.MaxDuration,
	}
}

type Timer struct {
	sync.Mutex
	samples     int64
	sumDuration time.Duration
	minDuration time.Duration
	maxDuration time.Duration
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
	t.sumDuration = tv.SumDuration
	t.minDuration = tv.MinDuration
	t.maxDuration = tv.MaxDuration
	return nil
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
		Samples:     t.samples,
		SumDuration: t.sumDuration,
		MinDuration: t.minDuration,
		MaxDuration: t.maxDuration,
	}
	if reset {
		t.samples = 0
		t.sumDuration = 0
		t.minDuration = 0
		t.maxDuration = 0
	}
	return ret
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
	Samples     int64         `json:"samples"`
	SumDuration time.Duration `json:"sum"`
	MinDuration time.Duration `json:"min"`
	MaxDuration time.Duration `json:"max"`
}

func (tp TimerValue) String() string {
	b, _ := json.Marshal(tp)
	return string(b)
}
