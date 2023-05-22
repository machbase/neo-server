package expression

import "fmt"

// Parameters is a collection of named parameters that can be used by an EvaluableExpression to retrieve parameters
// when an expression tries to use them.
type Parameters interface {
	// Get gets the parameter of the given name, or an error if the parameter is unavailable.
	// Failure to find the given parameter should be indicated by returning an error.
	Get(name string) (any, error)
}

type MapParameters map[string]any

func (p MapParameters) Get(name string) (any, error) {
	value, found := p[name]
	if !found {
		return nil, fmt.Errorf("no parameter '%s' found", name)
	}
	return value, nil
}

// sanitizedParameters is a wrapper for Parameters that does sanitization as
// parameters are accessed.
type sanitizedParameters struct {
	orig Parameters
}

func (p sanitizedParameters) Get(key string) (any, error) {
	value, err := p.orig.Get(key)
	if err != nil {
		return nil, err
	}
	return castToFloat64(value), nil
}

func castToFloat64(value any) any {
	switch v := value.(type) {
	case uint8:
		return float64(v)
	case uint16:
		return float64(v)
	case uint32:
		return float64(v)
	case uint64:
		return float64(v)
	case int8:
		return float64(v)
	case int16:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case int:
		return float64(v)
	case float32:
		return float64(v)
	}
	return value
}
