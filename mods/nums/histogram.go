package nums

import (
	"fmt"
	"sync"
)

type Bin struct {
	value float64
	count float64
}

// The value of the bin.
// This is the average of the values that fall into this bin.
// If you have 10 values that fall into this bin, the value will be the average of those 10 values.
// So you can calculate the total value of the bin by multiplying the value by the count.
func (b Bin) Value() float64 {
	return b.value
}

// The count of the bin.
// This is the number of values that fall into this bin.
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

// Quantile returns the value of the quantile.
// The quantile is a value that divides the data into two parts.
// For example, the median is the 50th percentile.
// It divides the data into two parts, with half of the data below the median and half above.
// The 90th percentile divides the data into two parts, with 90% of the data below the 90th percentile and 10% above.
// The 99th percentile divides the data into two parts, with 99% of the data below the 99th percentile and 1% above.
// The value of the quantile is the value that divides the data into two parts.
//
// ex) h.Quantile(0.5) // 50th percentile
// ex) h.Quantile(0.9) // 90th percentile
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
