package tagql

import (
	"fmt"
	"testing"

	"github.com/d5/tengo/v2/require"
)

func TestVec(t *testing.T) {
	vec := NewVec(0, 1, 2, 3)
	// [0 1 2 3]
	require.Equal(t, vec.Length(), 4)

	vec, _ = vec.Remove(3)
	// 0 1 2
	require.Equal(t, 3, vec.Length())

	vec, _ = vec.InsertAt(2, "5")
	// [0 1 5 2]
	require.Equal(t, "5", vec.At(2))

	vec, _ = vec.InsertAt(0, true)
	// [true 0 1 5 2]
	require.Equal(t, "5", vec.At(3))

	vec = vec.Append(1.2)
	require.Equal(t, 1.2, vec.At(5))

	for n, e := range []any{true, 0, 1, "5", 2} {
		require.True(t, e == vec[n], fmt.Sprintf("expect [%d] %v", n, e))
	}
}
