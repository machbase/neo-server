package metric

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
)

type HistBin struct {
	value float64
	count float64
}

func (hb HistBin) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`{"value":%f,"count":%f}`, hb.value, hb.count)), nil
}

func (hb *HistBin) UnmarshalJSON(data []byte) error {
	var obj struct {
		Value float64 `json:"value"`
		Count float64 `json:"count"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	hb.value = obj.Value
	hb.count = obj.Count
	return nil
}

type Histogram struct {
	sync.Mutex
	maxBins int
	bins    []HistBin
	samples float64
	qs      []float64 // Quantile to calculate
}

var _ Producer = (*Histogram)(nil)

func NewHistogram(maxBins int, qs ...float64) *Histogram {
	h := &Histogram{
		maxBins: maxBins,
		qs:      []float64{0.5, 0.90, 0.99},
	}
	if len(qs) > 0 {
		h.qs = qs
	}
	return h
}

func (h *Histogram) MarshalJSON() ([]byte, error) {
	h.Lock()
	defer h.Unlock()
	data := make(map[string]interface{})
	data["samples"] = int64(h.samples)
	data["qs"] = h.qs
	data["bins"] = h.bins
	return json.Marshal(data)
}

func (h *Histogram) UnmarshalJSON(data []byte) error {
	var obj struct {
		Samples int64     `json:"samples"`
		Qs      []float64 `json:"qs"`
		Bins    []HistBin `json:"bins"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	h.samples = float64(obj.Samples)
	h.qs = obj.Qs
	h.bins = obj.Bins
	return nil
}

func (h *Histogram) String() string {
	return h.Produce(false).String()
}

func (h *Histogram) Add(value float64) {
	h.Lock()
	defer func() {
		h.trim()
		h.Unlock()
	}()

	h.samples++
	newBin := HistBin{value: float64(value), count: 1}
	for i := range h.bins {
		if h.bins[i].value > float64(value) {
			h.bins = append(h.bins[:i], append([]HistBin{newBin}, h.bins[i:]...)...)
			return
		}
	}
	h.bins = append(h.bins, newBin)
}

func (h *Histogram) trim() {
	if h.maxBins <= 0 {
		h.maxBins = 100
	}
	for len(h.bins) > h.maxBins {
		d := float64(0)
		i := 0
		for j := 1; j < len(h.bins); j++ {
			if dv := h.bins[j].value - h.bins[j-1].value; dv < d || j == 1 {
				d = dv
				i = j
			}
		}
		count := h.bins[i].count + h.bins[i-1].count
		merged := HistBin{
			value: (h.bins[i].value*h.bins[i].count + h.bins[i-1].value*h.bins[i-1].count) / count,
			count: count,
		}
		h.bins = append(h.bins[:i-1], h.bins[i:]...)
		h.bins[i-1] = merged
	}
}

func (h *Histogram) bin(q float64) HistBin {
	count := q * float64(h.samples)
	for i := range h.bins {
		count -= h.bins[i].count
		if count <= 0 {
			return h.bins[i]
		}
	}
	return HistBin{}
}

func (h *Histogram) Quantile(q float64) float64 {
	h.Lock()
	defer h.Unlock()
	return h.bin(q).value
}

func (h *Histogram) Quantiles(qs ...float64) []float64 {
	h.Lock()
	defer h.Unlock()
	return h.quantile(qs...)
}

func (h *Histogram) quantile(qs ...float64) []float64 {
	ret := make([]float64, len(qs))
	counts := make([]float64, len(qs))
	for i, q := range qs {
		counts[i] = q * float64(h.samples)
	}
	found := 0
	for i := range h.bins {
		for idx := range counts {
			if counts[idx] == counts[idx] {
				counts[idx] -= h.bins[i].count
				if counts[idx] <= 0 {
					ret[idx] = h.bins[i].value
					counts[idx] = math.NaN() // Mark as found
					found++
				}
			}
		}
		if found == len(qs) {
			break
		}
	}
	return ret
}

func (h *Histogram) Produce(reset bool) Product {
	h.Lock()
	defer h.Unlock()
	ret := &HistogramProduct{
		Samples: int64(h.samples),
		P:       h.qs,
		Values:  h.quantile(h.qs...),
	}
	if reset {
		h.bins = nil
		h.samples = 0
	}
	return ret
}

type HistogramProduct struct {
	Samples int64     `json:"samples"`
	P       []float64 `json:"p"`
	Values  []float64 `json:"values"`
}

func (hp HistogramProduct) String() string {
	b, _ := json.Marshal(hp)
	return string(b)
}
