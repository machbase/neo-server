package metric

import (
	"encoding/json"
	"sync"
)

func NewCounter() *Counter {
	return &Counter{}
}

var _ Producer = (*Counter)(nil)

type Counter struct {
	sync.Mutex
	samples int64
	value   float64
}

func (fs *Counter) MarshalJSON() ([]byte, error) {
	p := fs.Produce(false)
	return json.Marshal(p)
}

func (fs *Counter) UnmarshalJSON(data []byte) error {
	p := &CounterValue{}
	if err := json.Unmarshal(data, p); err != nil {
		return err
	}
	fs.samples = p.Samples
	fs.value = p.Value
	return nil
}

func (fs *Counter) Add(v float64) {
	fs.Lock()
	defer fs.Unlock()
	fs.value += v
	fs.samples++
}

func (fs *Counter) Produce(reset bool) Value {
	fs.Lock()
	defer fs.Unlock()
	ret := &CounterValue{
		Samples: int64(fs.samples),
		Value:   float64(fs.value),
	}
	if reset {
		fs.samples = 0
		fs.value = 0
	}
	return ret
}

func (fs *Counter) String() string {
	return fs.Produce(false).String()
}

type CounterValue struct {
	Samples int64   `json:"samples"`
	Value   float64 `json:"value"`
}

func (cp *CounterValue) String() string {
	b, _ := json.Marshal(cp)
	return string(b)
}
