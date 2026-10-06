package util

import (
	"encoding/json"
	"fmt"

	client "github.com/machbase/neo-client/v2"
	"github.com/machbase/neo-client/v2/api"
)

// VectorFromJSON converts a JSON numeric array to the native VECTOR type.
func VectorFromJSON(value any) (api.Vector, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode VECTOR: %w", err)
	}
	return api.ParseVector(string(encoded))
}

// Unbox keeps every VECTOR scan-buffer shape numeric for JavaScript and JSON.
// In particular, database/sql can hand Neo a **api.Vector destination.
func Unbox(value any) any {
	switch vector := value.(type) {
	case api.Vector:
		if vector == nil {
			return nil
		}
		return append([]float32(nil), vector...)
	case *api.Vector:
		if vector == nil || *vector == nil {
			return nil
		}
		return append([]float32(nil), (*vector)...)
	case **api.Vector:
		if vector == nil || *vector == nil || **vector == nil {
			return nil
		}
		return append([]float32(nil), (**vector)...)
	case []float32:
		if vector == nil {
			return nil
		}
		return append([]float32(nil), vector...)
	default:
		return client.Unbox(value)
	}
}
