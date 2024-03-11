package tql

import (
	"github.com/machbase/neo-server/mods/util"
	spi "github.com/machbase/neo-spi"
)

func (node *Node) fmHistogram(value any, args ...any) (any, error) {
	var fv float64
	if f, err := util.ToFloat64(value); err != nil {
		return nil, err
	} else {
		fv = f
	}
	var hist *histogram
	if v, ok := node.GetValue("histogram"); ok {
		hist = v.(*histogram)
	} else {
		var bins *histBins
		for _, arg := range args {
			switch v := arg.(type) {
			case *histBins:
				bins = v
			}
		}
		if bins == nil || bins.min >= bins.max || bins.count <= 0 {
			return nil, ErrArgs("HIST", 1, "invalid bins configuration")
		}
		hist = &histogram{}
		hist.buckets = make([]histBucket, bins.count)
		hist.scale = (bins.max - bins.min) / float64(bins.count)
		hist.lowBucket.low = bins.min
		hist.lowBucket.high = bins.min
		hist.highBucket.low = bins.max
		hist.highBucket.high = bins.max
		for i := 0; i < bins.count; i++ {
			hist.buckets[i].low = bins.min + float64(i)*hist.scale
			hist.buckets[i].high = bins.min + float64(i+1)*hist.scale
		}
		node.SetValue("histogram", hist)
		node.SetEOF(func(node *Node) {
			cols := []*spi.Column{
				{Name: "ROWNUM", Type: "int"},
				{Name: "low", Type: "double"},
				{Name: "high", Type: "double"},
				{Name: "count", Type: "int"},
			}
			node.task.SetResultColumns(cols)
			id := 0
			if hist.lowBucket.count > 0 {
				node.yield(id, hist.lowBucket.values())
				id++
			}
			for i, b := range hist.buckets {
				id += i
				node.yield(i, b.values())
			}
			if hist.highBucket.count > 0 {
				node.yield(id+1, hist.highBucket.values())
			}
		})
	}

	hist.total++
	n := int((fv - hist.lowBucket.high) / hist.scale)
	if n < 0 {
		hist.lowBucket.count++
		if hist.lowBucket.low > fv {
			hist.lowBucket.low = fv
		}
	} else if n >= len(hist.buckets) {
		hist.highBucket.count++
		if hist.highBucket.high < fv {
			hist.highBucket.high = fv
		}
	} else {
		hist.buckets[n].count++
	}
	return nil, nil
}

type histogram struct {
	total      int // total = sum of all bucket's count
	scale      float64
	lowBucket  histBucket
	highBucket histBucket
	buckets    []histBucket
}

// A histogram bin for a value in range [low, high)
type histBucket struct {
	low   float64
	high  float64
	count int64
}

func (b histBucket) values() []any {
	return []any{
		b.low,
		b.high,
		b.count,
	}
}

func (node *Node) fmBins(min, max float64, count int) (any, error) {
	return &histBins{
		min:   min,
		max:   max,
		count: int(count),
	}, nil
}

type histBins struct {
	min   float64
	max   float64
	count int
}
