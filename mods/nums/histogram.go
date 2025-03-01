package nums

import (
	"fmt"
	"sync"
)

type Bin struct {
	value float64
	count float64
}

func (b Bin) Value() float64 {
	return b.value
}

func (b Bin) Count() float64 {
	return b.count
}

type Histogram struct {
	sync.Mutex
	MaxBins    int
	bins       []Bin
	totalCount float64
}

func (h *Histogram) String() string {
	return fmt.Sprintf(`{"p50": %f, "p90": %f, "p99": %f}`,
		h.Quantile(0.5).Value(),
		h.Quantile(0.9).Value(),
		h.Quantile(0.99).Value(),
	)
}

func (h *Histogram) Reset() {
	h.Lock()
	defer h.Unlock()
	h.bins = nil
	h.totalCount = 0
}

func (h *Histogram) Add(value float64) {
	h.Lock()
	defer func() {
		h.trim()
		h.Unlock()
	}()

	h.totalCount += 1
	newBin := Bin{value: value, count: 1}
	for i := range h.bins {
		if h.bins[i].value > value {
			h.bins = append(h.bins[:i], append([]Bin{newBin}, h.bins[i:]...)...)
			return
		}
	}
	h.bins = append(h.bins, newBin)
}

func (h *Histogram) trim() {
	maxBins := h.MaxBins
	if maxBins <= 0 {
		maxBins = 100
	}

	for len(h.bins) > maxBins {
		d := float64(0)
		i := 0
		for j := 1; j < len(h.bins); j++ {
			if dv := h.bins[j].value - h.bins[j-1].value; dv < d || j == 1 {
				d = dv
				i = j
			}
		}
		count := h.bins[i].count + h.bins[i-1].count
		merged := Bin{
			value: (h.bins[i].value*h.bins[i].count + h.bins[i-1].value*h.bins[i-1].count) / count,
			count: count,
		}
		h.bins = append(h.bins[:i-1], h.bins[i:]...)
		h.bins[i-1] = merged
	}
}

func (h *Histogram) Bins() []Bin {
	h.Lock()
	ret := make([]Bin, len(h.bins))
	copy(ret, h.bins)
	h.Unlock()
	return ret
}

func (h *Histogram) Quantile(q float64) Bin {
	h.Lock()
	defer h.Unlock()

	count := q * float64(h.totalCount)
	for i := range h.bins {
		count -= h.bins[i].count
		if count <= 0 {
			return h.bins[i]
		}
	}
	return Bin{}
}
