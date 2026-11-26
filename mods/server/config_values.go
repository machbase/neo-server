package server

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/zclconf/go-cty/cty"
)

func int64FromCty(value cty.Value) (int64, error) {
	switch value.Type() {
	case cty.Number:
		l, accuracy := value.AsBigFloat().Int64()
		if accuracy != big.Exact {
			return 0, fmt.Errorf("value is not an exact int64")
		}
		return l, nil
	case cty.String:
		s := value.AsString()
		for _, unit := range []struct {
			suffix string
			scale  int64
		}{{"ms", int64(time.Millisecond)}, {"s", int64(time.Second)}, {"m", int64(time.Minute)}, {"h", int64(time.Hour)}} {
			if strings.HasSuffix(s, unit.suffix) {
				n, err := strconv.ParseInt(strings.TrimSuffix(s, unit.suffix), 10, 64)
				if err != nil {
					return 0, err
				}
				if n > math.MaxInt64/unit.scale || n < math.MinInt64/unit.scale {
					return 0, fmt.Errorf("duration %q overflows int64", s)
				}
				return n * unit.scale, nil
			}
		}
		return strconv.ParseInt(s, 10, 64)
	default:
		return 0, fmt.Errorf("value is not a number, %s", value.Type())
	}
}

func uint64FromCty(value cty.Value) (uint64, error) {
	switch value.Type() {
	case cty.Number:
		l, accuracy := value.AsBigFloat().Uint64()
		if accuracy != big.Exact {
			return 0, fmt.Errorf("value is not an exact uint64")
		}
		return l, nil
	case cty.String:
		s := value.AsString()
		for _, unit := range []struct {
			suffix string
			scale  uint64
		}{{"s", uint64(time.Second)}, {"m", uint64(time.Minute)}, {"h", uint64(time.Hour)}} {
			if strings.HasSuffix(s, unit.suffix) {
				n, err := strconv.ParseUint(strings.TrimSuffix(s, unit.suffix), 10, 64)
				if err != nil {
					return 0, err
				}
				if n > math.MaxUint64/unit.scale {
					return 0, fmt.Errorf("duration %q overflows uint64", s)
				}
				return n * unit.scale, nil
			}
		}
		return strconv.ParseUint(s, 10, 64)
	default:
		return 0, fmt.Errorf("value is not a number, %s", value.Type())
	}
}

func float64FromCty(value cty.Value) (float64, error) {
	switch value.Type() {
	case cty.Number:
		f := value.AsBigFloat()
		l, _ := f.Float64()
		if math.IsInf(l, 0) {
			return 0, fmt.Errorf("value overflows float64")
		}
		return l, nil
	case cty.String:
		s := value.AsString()
		if l, err := strconv.ParseFloat(s, 64); err == nil {
			if math.IsNaN(l) || math.IsInf(l, 0) {
				return 0, fmt.Errorf("value must be finite")
			}
			return l, err
		} else {
			return 0, fmt.Errorf("value is not a number-compatible, %s", value.Type())
		}
	default:
		return 0, fmt.Errorf("value is not a number, %s", value.Type())
	}
}

func boolFromCty(value cty.Value) (bool, error) {
	switch value.Type() {
	case cty.Bool:
		return value.True(), nil
	case cty.String:
		s := value.AsString()
		switch strings.ToLower(s) {
		case "true", "t", "yes", "y":
			return true, nil
		case "false", "f", "no", "n":
			return false, nil
		default:
			return false, fmt.Errorf("%s is not bool compatible", s)
		}
	default:
		return false, fmt.Errorf("value is not a bool, %s", value.Type())
	}
}
