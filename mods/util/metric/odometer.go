package metric

import (
	"encoding/json"
	"sync"
)

func NewOdometer() *Odometer {
	return &Odometer{}
}

func NewOdometerWithValue(v *OdometerValue) *Odometer {
	return &Odometer{
		first:       v.First,
		last:        v.Last,
		samples:     v.Samples,
		initialized: !(v.First == 0 && v.Last == 0 && v.Samples == 0),
	}
}

var _ Producer = (*Odometer)(nil)

type Odometer struct {
	sync.Mutex
	first       float64
	last        float64
	samples     int64
	initialized bool
}

func (om *Odometer) MarshalJSON() ([]byte, error) {
	return json.Marshal(om.Produce(false))
}

func (om *Odometer) UnmarshalJSON(data []byte) error {
	p := &OdometerValue{}
	if err := json.Unmarshal(data, p); err != nil {
		return err
	}
	om.first = p.First
	om.last = p.Last
	om.samples = p.Samples
	om.initialized = !(om.first == 0 && om.last == 0 && om.samples == 0)
	return nil
}

func (om *Odometer) Derivers() []Deriver {
	return nil
}

func (om *Odometer) Add(v float64) {
	om.Lock()
	defer om.Unlock()
	om.samples++
	if !om.initialized {
		om.first = v
		om.last = v
		om.initialized = true
		return
	}
	om.last = v
}

func (om *Odometer) Produce(reset bool) Value {
	om.Lock()
	defer om.Unlock()
	v := &OdometerValue{
		First:   om.first,
		Last:    om.last,
		Samples: om.samples,
	}
	if reset {
		om.first = om.last
		om.samples = 0
	}
	return v
}

func (om *Odometer) String() string {
	b, _ := json.Marshal(om.Produce(false))
	return string(b)
}

type OdometerValue struct {
	First   float64 `json:"first"`
	Last    float64 `json:"last"`
	Samples int64   `json:"samples"`
}

func (ov *OdometerValue) String() string {
	b, _ := json.Marshal(ov)
	return string(b)
}

func (ov *OdometerValue) Diff() float64 {
	if ov.Samples == 0 {
		return 0
	}
	return ov.Last - ov.First
}

func (ov *OdometerValue) NonNegativeDiff() float64 {
	if ov.Samples == 0 {
		return 0
	}
	return max(ov.Last-ov.First, 0)
}

func (ov *OdometerValue) AbsDiff() float64 {
	if ov.Samples == 0 {
		return 0
	}
	ret := ov.Last - ov.First
	if ret < 0 {
		return -ret
	}
	return ret
}
