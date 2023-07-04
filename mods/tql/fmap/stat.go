package fmap

import (
	"fmt"

	"github.com/machbase/neo-server/mods/tql/context"
	"github.com/machbase/neo-server/mods/tql/conv"
	"gonum.org/v1/gonum/stat"
)

func mapf_STAT(args ...any) (any, error) {
	// CTX
	_, err := conv.Context(args, 0, "STAT")
	if err != nil {
		return nil, err
	}
	// K : any
	K, err := conv.Any(args, 1, "STAT", "K")
	if err != nil {
		return nil, err
	}
	// V : value
	V, err := conv.Array(args, 2, "STAT")
	if err != nil {
		return nil, err
	}
	if len(V) == 0 {
		return nil, nil
	}
	// method
	var method statMethod
	if len(args) >= 4 {
		if m, ok := args[3].(statMethod); ok {
			method = m
		}
	}
	if method == nil {
		return nil, conv.ErrWrongTypeOfArgs("STAT", 3, "stat method [mean()]", "none")
	}

	if _, ok := V[0].(float64); ok {
		arr := make([]float64, len(V))
		for i := range V {
			arr[i] = V[i].(float64)
		}
		result := method(arr)
		return &context.Param{K: K, V: result}, nil
	} else if _, ok := V[0].([]any); ok {
		marr := [][]float64{}
		colsNum := 0
		for _, vv := range V {
			if vvv, ok := vv.([]any); ok {
				for len(marr) < len(vvv) {
					marr = append(marr, []float64{})
					colsNum++
				}
				if _, ok := vvv[0].(float64); ok {
					for i := range vvv {
						marr[i] = append(marr[i], vvv[i].(float64))
					}
				}
			}
		}
		arr := make([]any, colsNum)
		for i := range arr {
			arr[i] = method(marr[i])
		}
		return &context.Param{K: K, V: arr}, nil
	} else {
		fmt.Println("ERR", "f(STAT) unknown input data type")
		return nil, nil
	}
}

type statMethod func([]float64) float64

func optf_stat_mean(args ...any) (any, error) {
	return statMethod(func(x []float64) float64 {
		return stat.Mean(x, nil)
	}), nil
}

func optf_stat_stddev(args ...any) (any, error) {
	return statMethod(func(x []float64) float64 {
		return stat.StdDev(x, nil)
	}), nil
}
