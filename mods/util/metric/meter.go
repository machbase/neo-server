package metric

import (
	"encoding/json"
	"sync"
)

func NewMeter() *Meter {
	return &Meter{}
}

func NewMeterWithValue(v *MeterValue) *Meter {
	return &Meter{
		first:   v.First,
		last:    v.Last,
		min:     v.Min,
		max:     v.Max,
		sum:     v.Sum,
		samples: v.Samples,
	}
}

var _ Producer = (*Meter)(nil)

type Meter struct {
	sync.Mutex
	first   float64
	last    float64
	min     float64
	max     float64
	sum     float64
	samples int64
}

func (m *Meter) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Produce(false))
}

func (m *Meter) UnmarshalJSON(data []byte) error {
	p := &MeterValue{}
	if err := json.Unmarshal(data, p); err != nil {
		return err
	}
	m.first = p.First
	m.last = p.Last
	m.min = p.Min
	m.max = p.Max
	m.sum = p.Sum
	m.samples = p.Samples
	return nil
}

func (m *Meter) Add(v float64) {
	m.Lock()
	defer m.Unlock()
	if m.samples == 0 {
		m.first = v
		m.min = v
		m.max = v
	}
	if v < m.min {
		m.min = v
	}
	if v > m.max {
		m.max = v
	}
	m.sum += v
	m.last = v
	m.samples++
}

func (m *Meter) Produce(reset bool) Value {
	m.Lock()
	defer m.Unlock()
	ret := &MeterValue{
		Samples: int64(m.samples),
		First:   float64(m.first),
		Last:    float64(m.last),
		Min:     float64(m.min),
		Max:     float64(m.max),
		Sum:     float64(m.sum),
	}
	if reset {
		m.first = 0
		m.last = 0
		m.min = 0
		m.max = 0
		m.sum = 0
		m.samples = 0
	}
	return ret
}

func (m *Meter) String() string {
	b, _ := json.Marshal(m.Produce(false))
	return string(b)
}

type MeterValue struct {
	Samples int64   `json:"samples"`
	Sum     float64 `json:"sum"`
	First   float64 `json:"first"`
	Last    float64 `json:"last"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
}

func (mp *MeterValue) String() string {
	b, _ := json.Marshal(mp)
	return string(b)
}
