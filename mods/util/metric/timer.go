package metric

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type Timer struct {
	sync.Mutex
	samples       int64
	totalDuration time.Duration
	minDuration   time.Duration
	maxDuration   time.Duration
}

func NewTimer() *Timer {
	return &Timer{}
}

func (t *Timer) MarshalJSON() ([]byte, error) {
	t.Lock()
	defer t.Unlock()
	return []byte(fmt.Sprintf(`{"samples":%d,"total":%d,"min":%d,"max":%d}`,
		t.samples, t.totalDuration.Nanoseconds(), t.minDuration.Nanoseconds(), t.maxDuration.Nanoseconds())), nil
}

func (t *Timer) UnmarshalJSON(data []byte) error {
	var obj struct {
		Samples int64         `json:"samples"`
		Total   time.Duration `json:"total"`
		Min     time.Duration `json:"min"`
		Max     time.Duration `json:"max"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	t.samples = obj.Samples
	t.totalDuration = obj.Total
	t.minDuration = obj.Min
	t.maxDuration = obj.Max
	return nil
}

func (t *Timer) String() string {
	return time.Duration(t.Value()).String()
}

func (t *Timer) Value() time.Duration {
	t.Lock()
	defer t.Unlock()
	if t.samples == 0 {
		return 0
	}
	return time.Duration(int64(t.totalDuration) / t.samples)
}

type TimerSnapshot struct {
	Samples       int64         `json:"samples"`
	TotalDuration time.Duration `json:"total"`
	MinDuration   time.Duration `json:"min"`
	MaxDuration   time.Duration `json:"max"`
}

func (t *Timer) Snapshot() TimerSnapshot {
	t.Lock()
	defer t.Unlock()
	return TimerSnapshot{
		Samples:       t.samples,
		TotalDuration: t.totalDuration,
		MinDuration:   t.minDuration,
		MaxDuration:   t.maxDuration,
	}
}

func (t *Timer) Mark(d time.Duration) {
	t.Lock()
	defer t.Unlock()
	t.samples++
	t.totalDuration += d
	if t.minDuration == 0 || d < t.minDuration {
		t.minDuration = d
	}
	if d > t.maxDuration {
		t.maxDuration = d
	}
}

func (t *Timer) Reset() {
	t.Lock()
	defer t.Unlock()
	t.samples = 0
	t.totalDuration = 0
	t.minDuration = 0
	t.maxDuration = 0
}
