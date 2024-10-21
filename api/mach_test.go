package api

import (
	"testing"

	"github.com/machbase/neo-server/api/types"
	"github.com/stretchr/testify/require"
)

func TestTableType(t *testing.T) {
	require.Equal(t, "LogTable", types.TableTypeLog.String())
}
