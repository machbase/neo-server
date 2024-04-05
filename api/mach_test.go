package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTableType(t *testing.T) {
	require.Equal(t, "LogTable", LogTableType.String())
}
