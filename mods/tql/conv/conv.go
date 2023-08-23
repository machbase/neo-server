package conv

import (
	"fmt"
	"io"
	"strconv"

	"github.com/machbase/neo-server/mods/tql/context"
)

func ErrInvalidNumOfArgs(name string, expect int, actual int) error {
	return fmt.Errorf("f(%s) invalid number of args; expect:%d, actual:%d", name, expect, actual)
}

func ErrWrongTypeOfArgs(name string, idx int, expect string, actual any) error {
	return fmt.Errorf("f(%s) arg(%d) should be %s, but %T", name, idx, expect, actual)
}

func ErrArgs(name string, idx int, msg string) error {
	return fmt.Errorf("f(%s) arg(%d) %s", name, idx, msg)
}

func Context(args []any, idx int, fname string) (*context.Context, error) {
	if idx >= len(args) {
		return nil, ErrInvalidNumOfArgs(fname, idx+1, len(args))
	}
	raw := args[idx]
	if ctx, ok := raw.(*context.Context); ok {
		return ctx, nil
	}
	return nil, ErrWrongTypeOfArgs(fname, idx, "Context", raw)
}

func Any(args []any, idx int, fname string, expect string) (any, error) {
	if idx >= len(args) {
		return nil, ErrInvalidNumOfArgs(fname, idx+1, len(args))
	}
	return args[idx], nil
}

func String(args []any, idx int, fname string, expect string) (string, error) {
	if idx >= len(args) {
		return "", ErrInvalidNumOfArgs(fname, idx+1, len(args))
	}
	raw := args[idx]
	switch v := raw.(type) {
	case string:
		return v, nil
	case *string:
		return *v, nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case *float64:
		return strconv.FormatFloat(*v, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	case *bool:
		return strconv.FormatBool(*v), nil
	default:
		return "", ErrWrongTypeOfArgs(fname, idx, expect, raw)
	}
}

func Int(args []any, idx int, fname string, expect string) (int, error) {
	if idx >= len(args) {
		return 0, ErrInvalidNumOfArgs(fname, idx+1, len(args))
	}
	raw := args[idx]
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

func Float64(args []any, idx int, fname string, expect string) (float64, error) {
	if idx >= len(args) {
		return 0, ErrInvalidNumOfArgs(fname, idx+1, len(args))
	}
	raw := args[idx]
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

func Array(args []any, idx int, fname string) ([]any, error) {
	if idx >= len(args) {
		return nil, ErrInvalidNumOfArgs(fname, idx+1, len(args))
	}
	raw := args[idx]
	switch v := raw.(type) {
	case []any:
		return v, nil
	default:
		return nil, ErrWrongTypeOfArgs(fname, idx, "Array", raw)
	}
}

func Bool(args []any, idx int, fname string, expect string) (bool, error) {
	if idx >= len(args) {
		return false, ErrInvalidNumOfArgs(fname, idx+1, len(args))
	}
	raw := args[idx]
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

func Reader(args []any, idx int, fname string, expect string) (io.Reader, error) {
	if idx >= len(args) {
		return nil, ErrInvalidNumOfArgs(fname, idx+1, len(args))
	}
	raw := args[idx]
	switch v := raw.(type) {
	case io.Reader:
		return v, nil
	default:
		return nil, ErrWrongTypeOfArgs(fname, idx, expect, raw)
	}
}

func Byte(args []any, idx int, fname string, expect string) (byte, error) {
	if idx >= len(args) {
		return 0, ErrInvalidNumOfArgs(fname, idx+1, len(args))
	}
	raw := args[idx]
	switch v := raw.(type) {
	case string:
		if len(v) != 1 {
			return 0, ErrArgs(fname, idx, "should be a single byte")
		}
		return v[0], nil
	case *string:
		if len(*v) != 1 {
			return 0, ErrArgs(fname, idx, "should be a single byte")
		}
		return (*v)[0], nil
	case []byte:
		if len(v) == 0 {
			return 0, ErrArgs(fname, idx, "should be a single byte")
		}
		return v[0], nil
	default:
		return 0, ErrWrongTypeOfArgs(fname, idx, expect, raw)
	}
}