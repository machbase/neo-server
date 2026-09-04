package internal

import (
	"fmt"
	"strings"
)

// FormatArray renders a flattened ARRAY column value (a []any produced by
// client.Unbox from api.Array) using the same textual form as api.Array.String(),
// e.g. "[1.2,3.4,5.6]". Nil elements are rendered as "null".
func FormatArray(values []any) string {
	parts := make([]string, len(values))
	for i, v := range values {
		if v == nil {
			parts[i] = "null"
		} else {
			parts[i] = fmt.Sprint(v)
		}
	}
	return "[" + strings.Join(parts, ",") + "]"
}
