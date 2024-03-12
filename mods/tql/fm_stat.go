package tql

import (
	"fmt"
	"math"
	"sort"

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
	var bins *HistogramStepBins
	var category HistogramBucketName
	var orders []HistogramBucketName

	for _, arg := range args {
		switch v := arg.(type) {
		case *HistogramStepBins:
			bins = v
		case HistogramBucketName:
			category = v
		case HistogramOrder:
			for _, str := range v {
				orders = append(orders, HistogramBucketName(str))
			}
		}
	}

	var hist *Histogram
	if v, ok := node.GetValue("histogram"); ok {
		hist = v.(*Histogram)
	} else {
		if bins == nil || bins.min >= bins.max || bins.step <= 0 {
			return nil, ErrArgs("HISTOGRAM", 1, "invalid bins configuration")
		}
		hist = &Histogram{
			bins:    bins,
			orders:  orders,
			buckets: map[HistogramBucketName]*HistogramBuckets{},
		}
		node.SetValue("histogram", hist)
		node.SetEOF(func(n *Node) {
			cols := []*spi.Column{
				{Name: "ROWNUM", Type: "int"},
				{Name: "low", Type: "double"},
				{Name: "high", Type: "double"},
			}
			catNames := hist.orderedCategoryNames()

			for _, catName := range catNames {
				if catName == "" {
					cols = append(cols, &spi.Column{Name: "count", Type: "int"})
				} else {
					cols = append(cols, &spi.Column{Name: string(catName), Type: "int"})
				}
			}
			node.task.SetResultColumns(cols)
			id := 0
			for i := range hist.buckets[catNames[0]].buckets {
				vs := []any{}
				countSum := int64(0)
				for _, catName := range catNames {
					cat := hist.buckets[catName]
					if len(vs) == 0 {
						vs = append(vs, cat.buckets[i].low, cat.buckets[i].high)
					}
					vs = append(vs, cat.buckets[i].count)
					countSum += cat.buckets[i].count
				}
				if (i == 0 || i == len(hist.buckets[catNames[0]].buckets)-1) && countSum == 0 {
					continue
				}
				node.yield(id, vs)
				id++
			}
		})
	}

	if bucket, ok := hist.buckets[category]; !ok {
		bucket := hist.bins.NewBuckets()
		hist.buckets[category] = bucket
		bucket.Add(fv)
	} else {
		bucket.Add(fv)
	}
	return nil, nil
}

type Histogram struct {
	bins    *HistogramStepBins
	buckets map[HistogramBucketName]*HistogramBuckets
	orders  []HistogramBucketName
}

func (hist *Histogram) orderedCategoryNames() []HistogramBucketName {
	catNames := []HistogramBucketName{}
	for cat := range hist.buckets {
		catNames = append(catNames, cat)
	}

	sort.Slice(catNames, func(i, j int) bool {
		in := -1
		jn := -1
		for n, name := range hist.orders {
			if name == catNames[i] {
				in = n
			}
			if name == catNames[j] {
				jn = n
			}
			if in != -1 && jn != -1 {
				return jn-in > 0
			}
		}
		if in != -1 {
			return true
		}
		if jn != -1 {
			return false
		}
		return catNames[j] > catNames[i]
	})
	return catNames
}

type HistogramBuckets struct {
	bucketIndexer func(float64) int
	buckets       []HistogramBin
}

func (hcat *HistogramBuckets) Add(fv float64) {
	n := hcat.bucketIndexer(fv)
	hcat.buckets[n].count++
}

// A histogram bin for a value in range [low, high)
type HistogramBin struct {
	low   float64
	high  float64
	count int64
}

func (node *Node) fmBins(min, max, step float64) (any, error) {
	return &HistogramStepBins{
		min:  min,
		max:  max,
		step: step,
	}, nil
}

type HistogramStepBins struct {
	min  float64
	max  float64
	step float64
}

func (hc *HistogramStepBins) String() string {
	return fmt.Sprintf("[%f-%f)/%f", hc.min, hc.max, hc.step)
}

func (hc *HistogramStepBins) NewBuckets() *HistogramBuckets {
	cat := &HistogramBuckets{}
	bucketsCount := int((hc.max-hc.min)/hc.step) + 2
	cat.buckets = make([]HistogramBin, bucketsCount)
	for i := 0; i < bucketsCount; i++ {
		if i == 0 {
			cat.buckets[i].low = math.Inf(-1)
		} else {
			cat.buckets[i].low = hc.min + float64(i-1)*hc.step
		}
		if i == bucketsCount-1 {
			cat.buckets[i].high = math.Inf(1)
		} else {
			cat.buckets[i].high = hc.min + float64(i)*hc.step
		}
	}
	cat.bucketIndexer = func(fv float64) int {
		ret := int((fv-hc.min)/hc.step) + 1
		if ret < 0 {
			return 0
		}
		if ret >= bucketsCount {
			return bucketsCount - 1
		}
		return ret
	}
	return cat
}

func (node *Node) fmCategory(arg any) any {
	switch v := arg.(type) {
	case *string:
		return HistogramBucketName(*v)
	case string:
		return HistogramBucketName(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

type HistogramBucketName string

func (node *Node) fmOrder(args ...string) any {
	ret := make([]string, len(args))
	copy(ret, args)
	return HistogramOrder(ret)
}

type HistogramOrder []string
