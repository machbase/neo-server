package tql

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/nums"
	"github.com/machbase/neo-server/v8/mods/util"
	"gonum.org/v1/gonum/stat"
)

func (node *Node) fmCategory(arg any) any {
	switch v := arg.(type) {
	case *string:
		return StatCategoryName(*v)
	case string:
		return StatCategoryName(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

type StatCategoryName string

func (node *Node) fmOrder(args ...string) any {
	ret := make([]string, len(args))
	copy(ret, args)
	return StatCategoryOrder(ret)
}

type StatCategoryOrder []string

func (node *Node) fmHistogram(value any, args ...any) (any, error) {
	var fv float64
	if f, err := util.ToFloat64(value); err != nil {
		return nil, err
	} else {
		fv = f
	}
	var maxBins HistogramMaxBins
	var bins *HistogramStepBins
	var category StatCategoryName
	var orders []StatCategoryName

	for _, arg := range args {
		switch v := arg.(type) {
		case HistogramMaxBins:
			maxBins = v
		case *HistogramStepBins:
			bins = v
		case StatCategoryName:
			category = v
		case StatCategoryOrder:
			for _, str := range v {
				orders = append(orders, StatCategoryName(str))
			}
		}
	}

	var hist Histogram
	if v, ok := node.GetValue("histogram"); ok {
		hist = v.(Histogram)
	} else {
		if bins != nil {
			if bins.min >= bins.max || bins.step <= 0 {
				return nil, ErrArgs("HISTOGRAM", 1, "invalid bins configuration")
			}
			hist = &HistogramPredictedBins{
				bins:    bins,
				orders:  orders,
				buckets: map[StatCategoryName]*HistogramBuckets{},
			}
		} else {
			hist = &HistogramUnpredictedBins{
				orders:  orders,
				buckets: map[StatCategoryName]*nums.Histogram{},
			}
		}
		node.SetValue("histogram", hist)
		node.SetEOF(func(n *Node) {
			id := 0
			switch h := hist.(type) {
			case *HistogramPredictedBins:
				cols := []*api.Column{
					api.MakeColumnRownum(),
					api.MakeColumnDouble("low"),
					api.MakeColumnDouble("high"),
				}
				catNames := hist.orderedCategoryNames()
				for _, catName := range catNames {
					if catName == "" {
						cols = append(cols, api.MakeColumnInt64("count"))
					} else {
						cols = append(cols, api.MakeColumnInt64(string(catName)))
					}
				}
				node.task.SetResultColumns(cols)
				for i := range h.buckets[catNames[0]].buckets {
					vs := []any{}
					countSum := int64(0)
					for _, catName := range catNames {
						cat := h.buckets[catName]
						if len(vs) == 0 {
							vs = append(vs, cat.buckets[i].low, cat.buckets[i].high)
						}
						vs = append(vs, cat.buckets[i].count)
						countSum += cat.buckets[i].count
					}
					if (i == 0 || i == len(h.buckets[catNames[0]].buckets)-1) && countSum == 0 {
						continue
					}
					node.yield(id, vs)
					id++
				}
			case *HistogramUnpredictedBins:
				cols := []*api.Column{
					api.MakeColumnRownum(),
					api.MakeColumnDouble("value"),
				}
				catNames := hist.orderedCategoryNames()
				for _, catName := range catNames {
					if catName == "" {
						cols = append(cols, api.MakeColumnInt64("count"))
					} else {
						cols = append(cols, api.MakeColumnInt64(string(catName)))
					}
				}
				node.task.SetResultColumns(cols)
				for _, catName := range catNames {
					cat := h.buckets[catName]
					for _, bin := range cat.Bins() {
						vs := []any{bin.Value(), bin.Count()}
						node.yield(id, vs)
						id++
					}
				}
			}
		})
	}

	switch h := hist.(type) {
	case *HistogramPredictedBins:
		if bucket, ok := h.buckets[category]; !ok {
			bucket := h.bins.NewBuckets()
			h.buckets[category] = bucket
			bucket.Add(fv)
		} else {
			bucket.Add(fv)
		}
	case *HistogramUnpredictedBins:
		if bucket, ok := h.buckets[category]; !ok {
			bucket = &nums.Histogram{MaxBins: int(maxBins)}
			h.buckets[category] = bucket
			bucket.Add(fv)
		} else {
			bucket.Add(fv)
		}
	}
	return nil, nil
}

type Histogram interface {
	orderedCategoryNames() []StatCategoryName
}

type HistogramUnpredictedBins struct {
	buckets map[StatCategoryName]*nums.Histogram
	orders  []StatCategoryName
}

func (hist *HistogramUnpredictedBins) orderedCategoryNames() []StatCategoryName {
	catNames := []StatCategoryName{}
	for cat := range hist.buckets {
		catNames = append(catNames, cat)
	}
	return sortCategoryNames(catNames, hist.orders)
}

type HistogramPredictedBins struct {
	bins    *HistogramStepBins
	buckets map[StatCategoryName]*HistogramBuckets
	orders  []StatCategoryName
}

func (hist *HistogramPredictedBins) orderedCategoryNames() []StatCategoryName {
	catNames := []StatCategoryName{}
	for cat := range hist.buckets {
		catNames = append(catNames, cat)
	}
	return sortCategoryNames(catNames, hist.orders)
}

func sortCategoryNames(catNames []StatCategoryName, orders []StatCategoryName) []StatCategoryName {
	sort.Slice(catNames, func(i, j int) bool {
		in := -1
		jn := -1
		for n, name := range orders {
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

func (histogramBuckets *HistogramBuckets) Add(fv float64) {
	n := histogramBuckets.bucketIndexer(fv)
	histogramBuckets.buckets[n].count++
}

// A histogram bin for a value in range [low, high)
type HistogramBin struct {
	low   float64
	high  float64
	count int64
}

type HistogramMaxBins int

func (node *Node) fmBins(args ...float64) (any, error) {
	if len(args) == 3 {
		return &HistogramStepBins{
			min:  args[0],
			max:  args[1],
			step: args[2],
		}, nil
	} else if len(args) == 1 {
		return HistogramMaxBins(args[0]), nil
	} else {
		return nil, fmt.Errorf("f(%s) invalid number of args; expected 1 or 3, got %d", "bins", len(args))
	}
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

func (node *Node) fmBoxplot(args ...any) (any, error) {
	var fv float64
	var category StatCategoryName
	var orders []StatCategoryName
	var cumulant = BoxplotCumulant{false, false, false}
	var format string = "standard"

	for _, arg := range args {
		switch v := arg.(type) {
		case StatCategoryName:
			category = v
		case StatCategoryOrder:
			for _, str := range v {
				orders = append(orders, StatCategoryName(str))
			}
		case BoxplotCumulant:
			cumulant = v
		case BoxplotFormat:
			format = string(v)
		default:
			if f, err := util.ToFloat64(v); err != nil {
				return nil, err
			} else {
				fv = f
			}
		}
	}

	var box *Boxplot
	if v, ok := node.GetValue("boxplot"); ok {
		box = v.(*Boxplot)
	} else {
		box = &Boxplot{
			orders:  orders,
			buckets: map[StatCategoryName]*[]float64{},
		}
		node.SetValue("boxplot", box)
		node.SetEOF(func(n *Node) {
			box.resultCatNames = box.orderedCategoryNames()
			box.result = make([]BoxplotResult, len(box.resultCatNames))
			for i, catName := range box.resultCatNames {
				bucket := box.buckets[catName]
				if len(*bucket) == 0 {
					box.result[i].empty = true
					continue
				}
				kind := stat.Empirical
				if cumulant[0] {
					kind = stat.LinInterp
				}
				values := *bucket
				sort.Float64s(values)
				q1 := stat.Quantile(0.25, kind, values, nil)
				kind = stat.Empirical
				if cumulant[1] {
					kind = stat.LinInterp
				}
				q2 := stat.Quantile(0.5, kind, values, nil)
				kind = stat.Empirical
				if cumulant[2] {
					kind = stat.LinInterp
				}
				q3 := stat.Quantile(0.75, kind, values, nil)
				iqr := q3 - q1
				lowerBound := q1 - (1.5 * iqr)
				upperBound := q3 + (1.5 * iqr)
				var outliers []float64
				var min, max float64 = math.Inf(1), math.Inf(-1)
				for _, v := range values {
					if v < lowerBound || v > upperBound {
						outliers = append(outliers, v)
					}
					if v < min {
						min = v
					}
					if v > max {
						max = v
					}
				}
				box.result[i] = BoxplotResult{
					iqr:        iqr,
					lowerBound: lowerBound,
					upperBound: upperBound,
					q1:         q1,
					q2:         q2,
					q3:         q3,
					outlier:    outliers,
					min:        min,
					max:        max,
				}
			}

			if format == "dict" {
				//////////////////////////////////
				// boxplot dictionary format
				cols := []*api.Column{
					api.MakeColumnRownum(),
				}
				for id, catName := range box.resultCatNames {
					if catName == "" {
						cols = append(cols, api.MakeColumnDict(fmt.Sprintf("boxplot_%d", id)))
					} else {
						cols = append(cols, api.MakeColumnDict(string(catName)))
					}
				}
				node.task.SetResultColumns(cols)

				row := []any{}
				for _, result := range box.result {
					itm := map[string]any{
						"min":     result.min,
						"max":     result.max,
						"q1":      result.q1,
						"q2":      result.q2,
						"q3":      result.q3,
						"lower":   result.lowerBound,
						"upper":   result.upperBound,
						"iqr":     result.iqr,
						"outlier": result.outlier,
					}
					if result.empty {
						itm = nil
					}
					row = append(row, itm)
				}
				node.yield(1, row)
			} else if format == "chart" {
				//////////////////////////////////
				// boxplot chart format
				cols := []*api.Column{
					api.MakeColumnRownum(),
					api.MakeColumnString("CATEGORY"),
					api.MakeColumnList("BOXPLOT"),
					api.MakeColumnList("OUTLIER"),
				}
				node.task.SetResultColumns(cols)

				for i, result := range box.result {
					// echarts data [lower,  Q1,  median (or Q2),  Q3,  upper]
					itm := []any{
						result.lowerBound,
						result.q1,
						result.q2,
						result.q3,
						result.upperBound,
					}
					if result.empty {
						itm = nil
					}
					catName := string(box.resultCatNames[i])

					var outlier []any
					for _, o := range result.outlier {
						outlier = append(outlier, []any{catName, o})
					}

					node.yield(1, []any{catName, itm, outlier})
				}
			} else {
				//////////////////////////////////
				// boxplot standard
				cols := []*api.Column{
					api.MakeColumnRownum(),
					api.MakeColumnString("CATEGORY"),
				}
				for id, catName := range box.resultCatNames {
					if catName == "" {
						cols = append(cols, api.MakeColumnDouble(fmt.Sprintf("boxplot_%d", id)))
					} else {
						cols = append(cols, api.MakeColumnDouble(string(catName)))
					}
				}
				node.task.SetResultColumns(cols)

				rowQ1, rowQ2, rowQ3 := []any{"Q1"}, []any{"Q2"}, []any{"Q3"}
				rowIqr, rowLowerBound, rowUpperBound := []any{"IQR"}, []any{"LOWER"}, []any{"UPPER"}
				rowMin, rowMax := []any{"MIN"}, []any{"MAX"}
				rowOutlier := []any{"OUTLIER"}
				for _, result := range box.result {
					if !result.empty {
						rowQ1 = append(rowQ1, result.q1)
						rowQ2 = append(rowQ2, result.q2)
						rowQ3 = append(rowQ3, result.q3)
						rowIqr = append(rowIqr, result.iqr)
						rowLowerBound = append(rowLowerBound, result.lowerBound)
						rowUpperBound = append(rowUpperBound, result.upperBound)
						rowMin = append(rowMin, result.min)
						rowMax = append(rowMax, result.max)
						rowOutlier = append(rowOutlier, result.outlier)
					} else {
						rowQ1 = append(rowQ1, nil)
						rowQ2 = append(rowQ2, nil)
						rowQ3 = append(rowQ3, nil)
						rowIqr = append(rowIqr, nil)
						rowLowerBound = append(rowLowerBound, nil)
						rowUpperBound = append(rowUpperBound, nil)
						rowMin = append(rowMin, nil)
						rowMax = append(rowMax, nil)
						rowOutlier = append(rowOutlier, nil)
					}
				}
				node.yield(1, rowMin)
				node.yield(2, rowLowerBound)
				node.yield(3, rowQ1)
				node.yield(4, rowQ2)
				node.yield(5, rowQ3)
				node.yield(6, rowUpperBound)
				node.yield(7, rowMax)
				node.yield(8, rowIqr)
				node.yield(9, rowOutlier)
			}
		})
	}

	if bucket, ok := box.buckets[category]; !ok {
		box.buckets[category] = &[]float64{fv}
	} else {
		*bucket = append(*bucket, fv)
	}
	return nil, nil
}

type Boxplot struct {
	buckets        map[StatCategoryName]*[]float64
	orders         []StatCategoryName
	resultCatNames []StatCategoryName
	result         []BoxplotResult
}

type BoxplotResult struct {
	empty      bool
	iqr        float64
	lowerBound float64
	upperBound float64
	q1, q2, q3 float64
	min, max   float64
	outlier    []float64
}

func (hist *Boxplot) orderedCategoryNames() []StatCategoryName {
	catNames := []StatCategoryName{}
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

type BoxplotCumulant [3]bool

func (node *Node) fmBoxplotInterp(q1, q2, q3 bool) any {
	return BoxplotCumulant{q1, q2, q3}
}

type BoxplotFormat string

func (node *Node) fmBoxplotOutputFormat(format string) any {
	switch strings.ToLower(format) {
	case "standard", "dict", "chart":
		return BoxplotFormat(strings.ToLower(format))
	default:
		return BoxplotFormat("standard")
	}
}
