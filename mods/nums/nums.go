package nums

import (
	"math"
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

func Linspace50(start float64, stop float64) []float64 {
	return Linspace(start, stop, 50)
}

func Linspace(start float64, stop float64, num int) []float64 {
	ret := make([]float64, num)
	step := (stop - start) / float64(num-1)
	for i := range ret {
		ret[i] = start + float64(i)*step
	}
	ret[len(ret)-1] = stop
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
