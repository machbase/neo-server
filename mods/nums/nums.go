package nums

import (
	"math"
	"sort"
)

// greatest common divisor (GCD) via Euclidean algorithm
func GCD[T int | int64](a, b T) T {
	for b != 0 {
		t := b
		b = a % b
		a = t
	}
	return a
}

// find Least Common Multiple (LCM) via GCD
func LCM[T int | int64](a, b T, integers ...T) T {
	result := a * b / GCD(a, b)

	for i := 0; i < len(integers); i++ {
		result = LCM(result, integers[i])
	}

	return result
}

// `round(number, number)`
func Round(num int64, mod int64) float64 {
	if mod == 0 {
		return math.NaN()
	}
	return float64((num / mod) * mod)
}

func Mod(x, y float64) float64 {
	return math.Mod(x, y)
}

func Arrange(start float64, stop float64, step float64) []float64 {
	if start == stop || step == 0 {
		return []float64{}
	}
	if start <= stop && step < 0 {
		return []float64{}
	}
	if start > stop && step > 0 {
		return []float64{}
	}
	i := 0
	if start < stop {
		cap := int(math.Abs(step)) + 1
		ret := make([]float64, 0, cap)
		for v := start; v <= stop; v += step {
			ret = append(ret, v)
			i++
		}
		return ret
	} else {
		cap := int(math.Abs(step)) + 1
		ret := make([]float64, 0, cap)
		for v := start; v >= stop; v += step {
			ret = append(ret, v)
			i++
		}
		return ret
	}
}

func Linspace50(start float64, stop float64) []float64 {
	return Linspace(start, stop, 50)
}

func Linspace(start float64, stop float64, num int) []float64 {
	if num < 0 {
		num = 0
	}
	ret := make([]float64, num)
	multiplier := 1.0
	if num > 1 {
		multiplier = (stop - start) / float64(num-1)
	}
	for i := range ret {
		ret[i] = start + float64(i)*multiplier
	}
	if num > 1 {
		ret[len(ret)-1] = stop
	}
	return ret
}

func Meshgrid(x []float64, y []float64) [][][]float64 {
	ret := make([][][]float64, len(x))

	for i := 0; i < len(x); i++ {
		ret[i] = make([][]float64, len(y))
		for n := 0; n < len(y); n++ {
			ret[i][n] = []float64{x[i], y[n]}
		}
	}
	return ret
}

type WeightedFloat64 [2]float64

func WeightedFloat64ValueWeight(v float64, weight float64) WeightedFloat64 {
	return WeightedFloat64{v, weight}
}

func WeightedFloat64Value(v float64) WeightedFloat64 {
	return WeightedFloat64{v, 1.0}
}

func (x WeightedFloat64) Value() float64  { return x[0] }
func (x WeightedFloat64) Weight() float64 { return x[1] }

type WeightedFloat64Slice []WeightedFloat64

func (x WeightedFloat64Slice) Len() int           { return len(x) }
func (x WeightedFloat64Slice) Less(i, j int) bool { return x[i][0] < x[j][0] }
func (x WeightedFloat64Slice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }
func (x WeightedFloat64Slice) Sort()              { sort.Sort(x) }

func (x WeightedFloat64Slice) Values() []float64 {
	var ret = make([]float64, len(x))
	for i, v := range x {
		ret[i] = v[0]
	}
	return ret
}

func (x WeightedFloat64Slice) Weights() []float64 {
	var ret = make([]float64, len(x))
	for i, v := range x {
		ret[i] = v[1]
	}
	return ret
}
