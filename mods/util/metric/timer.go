package metric

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type Timer struct {
	sync.Mutex
	samples     int64
	sumDuration time.Duration
	minDuration time.Duration
	maxDuration time.Duration
}

var _ Producer = (*Timer)(nil)

func NewTimer() *Timer {
	return &Timer{}
}

func (t *Timer) MarshalJSON() ([]byte, error) {
	t.Lock()
	defer t.Unlock()
	return []byte(fmt.Sprintf(`{"samples":%d,"sum":%d,"min":%d,"max":%d}`,
		t.samples, t.sumDuration.Nanoseconds(), t.minDuration.Nanoseconds(), t.maxDuration.Nanoseconds())), nil
}

func (t *Timer) UnmarshalJSON(data []byte) error {
	var obj struct {
		Samples int64         `json:"samples"`
		Sum     time.Duration `json:"sum"`
		Min     time.Duration `json:"min"`
		Max     time.Duration `json:"max"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	t.samples = obj.Samples
	t.sumDuration = obj.Sum
	t.minDuration = obj.Min
	t.maxDuration = obj.Max
	return nil
}

func (t *Timer) String() string {
	return t.Produce(false).String()
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
	ret := TimerValue{
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
	t.samples++
	t.sumDuration += d
	if t.minDuration == 0 || d < t.minDuration {
		t.minDuration = d
	}
	if d > t.maxDuration {
		t.maxDuration = d
	}
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
