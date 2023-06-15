package conv

import (
	"fmt"
	"strconv"
)

func ErrInvalidNumOfArgs(name string, expect int, actual int) error {
	return fmt.Errorf("f(%s) invalid number of args; expect:%d, actual:%d", name, expect, actual)
}

func ErrWrongTypeOfArgs(name string, idx int, expect string, actual any) error {
	return fmt.Errorf("f(%s) arg(%d) should be %s, but %T", name, idx, expect, actual)
}

func Int(raw any, fname string, idx int, expect string) (int, error) {
	switch v := raw.(type) {
	case float64:
		return int(v), nil
	case *float64:
		return int(*v), nil
	case string:
		if fv, err := strconv.ParseInt(v, 10, 32); err != nil {
			return 0, ErrWrongTypeOfArgs(fname, idx, expect, raw)
		} else {
			return int(fv), nil
		}
	default:
		return 0, ErrWrongTypeOfArgs(fname, idx, expect, raw)
	}
}

func Float64(raw any, fname string, idx int, expect string) (float64, error) {
	switch v := raw.(type) {
	case float64:
		return v, nil
	case *float64:
		return *v, nil
	case string:
		if fv, err := strconv.ParseFloat(v, 64); err != nil {
			return 0, ErrWrongTypeOfArgs(fname, idx, expect, raw)
		} else {
			return fv, nil
		}
	default:
		return 0, ErrWrongTypeOfArgs(fname, idx, expect, raw)
	}
}

func Bool(raw any, fname string, idx int, expect string) (bool, error) {
	switch v := raw.(type) {
	case bool:
		return v, nil
	case string:
		if fv, err := strconv.ParseBool(v); err != nil {
			return false, ErrWrongTypeOfArgs(fname, idx, expect, raw)
		} else {
			return fv, nil
		}
	default:
		return false, ErrWrongTypeOfArgs(fname, idx, expect, raw)
	}
}
