package metric

import (
	"encoding/json"
	"sync"
)

func NewOdometer() *Odometer {
	return &Odometer{}
}

var _ Producer = (*Odometer)(nil)

type Odometer struct {
	sync.Mutex
	first      float64
	last       float64
	validFirst bool
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
	om.validFirst = !p.Empty
	return nil
}

func (om *Odometer) Add(v float64) {
	om.Lock()
	defer om.Unlock()
	if !om.validFirst {
		om.first = v
		om.last = v
		om.validFirst = true
		return
	}
	om.last = v
}

func (om *Odometer) Produce(reset bool) Value {
	om.Lock()
	defer om.Unlock()
	v := &OdometerValue{
		First: om.first,
		Last:  om.last,
	}
	if !om.validFirst {
		v.Empty = true
	}
	if reset {
		om.first = om.last
	}
	return v
}

func (om *Odometer) String() string {
	b, _ := json.Marshal(om.Produce(false))
	return string(b)
}

type OdometerValue struct {
	First float64 `json:"first"`
	Last  float64 `json:"last"`
	Empty bool    `json:"empty,omitempty"`
}

func (ov *OdometerValue) String() string {
	b, _ := json.Marshal(ov)
	return string(b)
}

func (ov *OdometerValue) Diff() float64 {
	if ov.Empty {
		return 0
	}
	return ov.Last - ov.First
}

func (ov *OdometerValue) NonNegativeDiff() float64 {
	if ov.Empty {
		return 0
	}
	return max(ov.Last-ov.First, 0)
}

func (ov *OdometerValue) NonAbsDiff() float64 {
	if ov.Empty {
		return 0
	}
	ret := ov.Last - ov.First
	if ret < 0 {
		return -ret
	}
	return ret
}
