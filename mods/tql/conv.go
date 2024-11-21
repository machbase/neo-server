package tql

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/codec/facility"
	"github.com/machbase/neo-server/v8/mods/nums"
	"github.com/machbase/neo-server/v8/mods/util"
	"golang.org/x/text/encoding"
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

func convInputStream(args []any, idx int, fname string, expect string) (io.Reader, error) {
	if idx >= len(args) {
		return nil, ErrInvalidNumOfArgs(fname, idx+1, len(args))
	}
	if o, ok := args[idx].(io.Reader); ok {
		return o, nil
	}
	return nil, ErrWrongTypeOfArgs(fname, idx, expect, args[idx])
}

func convOutputStream(args []any, idx int, fname string, expect string) (io.Writer, error) {
	if idx >= len(args) {
		return nil, ErrInvalidNumOfArgs(fname, idx+1, len(args))
	}
	if o, ok := args[idx].(io.Writer); ok {
		return o, nil
	}
	return nil, ErrWrongTypeOfArgs(fname, idx, expect, args[idx])
}

func convVolatileFileWriter(args []any, idx int, fname string, expect string) (facility.VolatileFileWriter, error) {
	if idx >= len(args) {
		return nil, ErrInvalidNumOfArgs(fname, idx+1, len(args))
	}
	if o, ok := args[idx].(facility.VolatileFileWriter); ok {
		return o, nil
	}
	return nil, ErrWrongTypeOfArgs(fname, idx, expect, args[idx])
}

func convLogger(args []any, idx int, fname string, expect string) (facility.Logger, error) {
	if idx >= len(args) {
		return nil, ErrInvalidNumOfArgs(fname, idx+1, len(args))
	}
	if o, ok := args[idx].(facility.Logger); ok {
		return o, nil
	}
	return nil, ErrWrongTypeOfArgs(fname, idx, expect, args[idx])
}

func convLatLon(args []any, idx int, fname string, expect string) (*nums.LatLon, error) {
	if idx >= len(args) {
		return nil, ErrInvalidNumOfArgs(fname, idx+1, len(args))
	}
	if o, ok := args[idx].(*nums.LatLon); ok {
		return o, nil
	}
	return nil, ErrWrongTypeOfArgs(fname, idx, expect, args[idx])
}

func convGeography(args []any, idx int, fname string, expect string) (nums.Geography, error) {
	if idx >= len(args) {
		return nil, ErrInvalidNumOfArgs(fname, idx+1, len(args))
	}
	if o, ok := args[idx].(nums.Geography); ok {
		return o, nil
	}
	return nil, ErrWrongTypeOfArgs(fname, idx, expect, args[idx])
}

func convCharset(args []any, idx int, fname string, expect string) (encoding.Encoding, error) {
	if idx >= len(args) {
		return nil, ErrInvalidNumOfArgs(fname, idx+1, len(args))
	}
	if o, ok := args[idx].(encoding.Encoding); ok {
		return o, nil
	}
	return nil, ErrWrongTypeOfArgs(fname, idx, expect, args[idx])
}

func convAny(args []any, idx int, fname string, _ string) (any, error) {
	if idx >= len(args) {
		return nil, ErrInvalidNumOfArgs(fname, idx+1, len(args))
	}
	return args[idx], nil
}

func convTimeLocation(args []any, idx int, fname string, expect string) (*time.Location, error) {
	if idx >= len(args) {
		return nil, ErrInvalidNumOfArgs(fname, idx+1, len(args))
	}
	switch v := args[idx].(type) {
	case *time.Location:
		return v, nil
	case string:
		if timeLocation, _ := util.ParseTimeLocation(v, nil); timeLocation == nil {
			return nil, fmt.Errorf("f(%s) arg(%d) invalid time location: %s", fname, idx+1, v)
		} else {
			return timeLocation, nil
		}
	default:
		return nil, ErrWrongTypeOfArgs(fname, idx, expect, args[idx])
	}
}

func convString(args []any, idx int, fname string, expect string) (string, error) {
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
	case int:
		return strconv.FormatInt(int64(v), 10), nil
	case *int:
		return strconv.FormatInt(int64(*v), 10), nil
	case int16:
		return strconv.FormatInt(int64(v), 10), nil
	case *int16:
		return strconv.FormatInt(int64(*v), 10), nil
	case int32:
		return strconv.FormatInt(int64(v), 10), nil
	case *int32:
		return strconv.FormatInt(int64(*v), 10), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case *int64:
		return strconv.FormatInt(*v, 10), nil
	case bool:
		return strconv.FormatBool(v), nil
	case *bool:
		return strconv.FormatBool(*v), nil
	default:
		return "", ErrWrongTypeOfArgs(fname, idx, expect, raw)
	}
}

func convInt(args []any, idx int, fname string, expect string) (int, error) {
	if idx >= len(args) {
		return 0, ErrInvalidNumOfArgs(fname, idx+1, len(args))
	}
	raw := args[idx]
	switch v := raw.(type) {
	case int:
		return v, nil
	case *int:
		return *v, nil
	case int16:
		return int(v), nil
	case *int16:
		return int(*v), nil
	case int32:
		return int(v), nil
	case *int32:
		return int(*v), nil
	case int64:
		return int(v), nil
	case *int64:
		return int(*v), nil
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

func convInt64(args []any, idx int, fname string, expect string) (int64, error) {
	if idx >= len(args) {
		return 0, ErrInvalidNumOfArgs(fname, idx+1, len(args))
	}
	raw := args[idx]
	switch v := raw.(type) {
	case float64:
		return int64(v), nil
	case *float64:
		return int64(*v), nil
	case string:
		if fv, err := strconv.ParseInt(v, 10, 64); err != nil {
			return 0, ErrWrongTypeOfArgs(fname, idx, expect, raw)
		} else {
			return fv, nil
		}
	default:
		return 0, ErrWrongTypeOfArgs(fname, idx, expect, raw)
	}
}

func convFloat32(args []any, idx int, fname string, expect string) (float32, error) {
	if idx >= len(args) {
		return 0, ErrInvalidNumOfArgs(fname, idx+1, len(args))
	}
	raw := args[idx]
	switch v := raw.(type) {
	case float64:
		return float32(v), nil
	case *float64:
		return float32(*v), nil
	case string:
		if fv, err := strconv.ParseFloat(v, 32); err != nil {
			return 0, ErrWrongTypeOfArgs(fname, idx, expect, raw)
		} else {
			return float32(fv), nil
		}
	default:
		return 0, ErrWrongTypeOfArgs(fname, idx, expect, raw)
	}
}

func convFloat64(args []any, idx int, fname string, expect string) (float64, error) {
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

func convBool(args []any, idx int, fname string, expect string) (bool, error) {
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

func convByte(args []any, idx int, fname string, expect string) (byte, error) {
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

func convDataType(args []any, idx int, fname string, expect string) (api.DataType, error) {
	if idx >= len(args) {
		return "", ErrInvalidNumOfArgs(fname, idx+1, len(args))
	}
	raw := args[idx]
	switch v := raw.(type) {
	case string:
		if len(v) != 1 {
			return "", ErrArgs(fname, idx, "should be a data type")
		}
		return api.DataType(v), nil
	case *string:
		if len(*v) != 1 {
			return "", ErrArgs(fname, idx, "should be a data type")
		}
		return api.DataType(*v), nil
	default:
		return "", ErrWrongTypeOfArgs(fname, idx, expect, raw)
	}
}
