package metric

import (
	"encoding/json"
	"sync"
)

func NewGauge() *Gauge {
	return &Gauge{}
}

func NewGaugeWithValue(v *GaugeValue) *Gauge {
	return &Gauge{
		samples: v.Samples,
		sum:     v.Sum,
		value:   v.Value,
	}
}

var _ Producer = (*Gauge)(nil)

type Gauge struct {
	sync.Mutex
	samples  int64
	sum      float64
	value    float64
	derivers []Deriver
}

func (fs *Gauge) MarshalJSON() ([]byte, error) {
	p := fs.Produce(false)
	return json.Marshal(p)
}

func (fs *Gauge) UnmarshalJSON(data []byte) error {
	p := &GaugeValue{}
	if err := json.Unmarshal(data, p); err != nil {
		return err
	}
	fs.samples = p.Samples
	fs.sum = p.Sum
	fs.value = p.Value
	return nil
}

func (fs *Gauge) WithDerivers(derivers ...Deriver) *Gauge {
	fs.derivers = append(fs.derivers, derivers...)
	return fs
}

func (fs *Gauge) Derivers() []Deriver {
	return fs.derivers
}

func (fs *Gauge) Add(v float64) {
	fs.Lock()
	defer fs.Unlock()
	fs.value = v
	fs.sum += v
	fs.samples++
}

func (fs *Gauge) Produce(reset bool) Value {
	fs.Lock()
	defer fs.Unlock()
	ret := &GaugeValue{
		Samples: int64(fs.samples),
		Value:   float64(fs.value),
		Sum:     float64(fs.sum),
	}
	if reset {
		fs.value = 0   // Reset the last value after peeking
		fs.samples = 0 // Reset the sample count after peeking
		fs.sum = 0     // Reset the total after peeking
	}
	return ret
}

func (fs *Gauge) String() string {
	return fs.Produce(false).String()
}

type GaugeValue struct {
	Samples int64   `json:"samples"`
	Sum     float64 `json:"sum"`
	Value   float64 `json:"value"`
	// Optional derived values, such as moving averages
	DerivedValues map[string]Value `json:"derived,omitempty"`
}

func (gp *GaugeValue) String() string {
	b, _ := json.Marshal(gp)
	return string(b)
}

func (cp *GaugeValue) SetDerivedValue(name string, value Value) {
	if cp.DerivedValues == nil {
		cp.DerivedValues = make(map[string]Value)
	}
	cp.DerivedValues[name] = value
}
