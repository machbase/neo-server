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
	// method string
	var method = "mean"
	if len(args) >= 4 {
		method, err = conv.String(args, 3, "STAT", "method string")
		if err != nil {
			return nil, err
		}
	}

	var doStat func(x []float64, weights []float64) float64

	switch method {
	case "mean":
		doStat = stat.Mean
	case "stddev":
		doStat = stat.StdDev
	}

	if _, ok := V[0].(float64); ok {
		arr := make([]float64, len(V))
		for i := range V {
			arr[i] = V[i].(float64)
		}
		mean := doStat(arr, nil)
		return &context.Param{K: K, V: mean}, nil
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
			arr[i] = doStat(marr[i], nil)
		}
		return &context.Param{K: K, V: arr}, nil
	} else {
		fmt.Println("ERR", "f(STAT) unknown input data type")
		return nil, nil
	}
}
