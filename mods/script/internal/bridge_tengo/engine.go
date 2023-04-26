package bridge_tengo

import (
	"context"
	"fmt"
	"reflect"

	"github.com/d5/tengo/v2"
	"github.com/d5/tengo/v2/stdlib"
	"github.com/pkg/errors"
)

func New(rawScript []byte) (*engine, error) {
	il := &engine{}
	il.script = tengo.NewScript(rawScript)
	return il, nil
}

type engine struct {
	script   *tengo.Script
	compiled *tengo.Compiled
	err      error
}

func (eng *engine) SetVar(name string, value any) error {
	return eng.script.Add(name, value)
}

func (eng *engine) SetFunc(name string, userFunc func(...any) (any, error)) {
	ftype := reflect.TypeOf(userFunc)
	argLen := ftype.NumIn()
	argTypes := make([]reflect.Type, argLen)
	for i := range argTypes {
		argTypes[i] = ftype.In(i)
	}

	eng.script.Add(name, &tengo.UserFunction{
		Value: func(args ...tengo.Object) (tengo.Object, error) {
			vargs := make([]any, len(args))
			for i, v := range args {
				vargs[i] = tengo.ToInterface(v)
			}

			if len(args) != argLen {
				return nil, tengo.ErrWrongNumArguments
			}
			rt, err := userFunc(vargs...)
			if err != nil {
				return &tengo.Error{
					Value: &tengo.String{Value: err.Error()},
				}, nil
			}
			o, err := tengo.FromInterface(rt)
			if err != nil {
				return &tengo.Error{
					Value: &tengo.String{Value: err.Error()},
				}, nil
			}
			return o, nil
		},
	})
}

func (eng *engine) GetVar(name string, value any) error {
	sval := eng.compiled.Get(name)
	styp := sval.ValueType()
	switch styp {
	case "int":
		switch vv := value.(type) {
		case *int:
			*vv = sval.Int()
		case *int32:
			*vv = int32(sval.Int())
		case *int64:
			*vv = sval.Int64()
		default:
			return fmt.Errorf("unsupported type conversion %T from %s", vv, styp)
		}
	case "float":
		switch vv := value.(type) {
		case *float32:
			*vv = float32(sval.Float())
		case *float64:
			*vv = sval.Float()
		default:
			return fmt.Errorf("unsupported type conversion %T from %s", vv, styp)
		}
	case "bool":
		switch vv := value.(type) {
		case *bool:
			*vv = sval.Bool()
		}
	case "string":
		switch vv := value.(type) {
		case *string:
			*vv = sval.String()
		default:
			return fmt.Errorf("unsupported type conversion %T from %s", vv, styp)
		}
	case "bytes":
		switch vv := value.(type) {
		case *[]byte:
			*vv = sval.Bytes()
		default:
			return fmt.Errorf("unsupported type conversion %T from %s", vv, styp)
		}
	case "map":
		switch vv := value.(type) {
		case *map[string]any:
			*vv = sval.Map()
		default:
			return fmt.Errorf("unsupported type conversion %T from %s", vv, styp)
		}
	case "array":
		switch vv := value.(type) {
		case *[]any:
			*vv = sval.Array()
		case *any:
			*vv = sval.Array()
		default:
			return fmt.Errorf("unsupported type conversion %T from %s", vv, styp)
		}
	default:
		return fmt.Errorf("unsupported type conversion %T from %s", value, styp)
	}
	return nil
}

func (eng *engine) Run() error {
	eng.script.SetImports(stdlib.GetModuleMap(stdlib.AllModuleNames()...))

	ctx := context.Background()
	eng.compiled, eng.err = eng.script.Compile()
	if eng.err != nil {
		return errors.Wrap(eng.err, "Compile")
	}
	eng.err = eng.compiled.RunContext(ctx)
	if eng.err != nil {
		return errors.Wrap(eng.err, "RunContext")
	}

	return nil
}
