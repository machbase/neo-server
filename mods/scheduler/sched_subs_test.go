package scheduler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInsertPayload(t *testing.T) {
	tt := []struct {
		name    string
		payload string
		expect  []string
	}{
		{
			name:    "array_of_array",
			payload: `{"data":{"columns":["one","two","three"], "rows":[[1,2,3],[4,5,6],[7,8,9]]}}`,
			expect:  []string{"one", "two", "three"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			result := extracColumns([]byte(tc.payload))
			require.EqualValues(t, tc.expect, result)
		})
	}
}
