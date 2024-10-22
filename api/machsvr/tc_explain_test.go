package machsvr_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type Explainer interface {
	// Explain retrieves execution plan of the given SQL statement.
	Explain(ctx context.Context, sqlText string, full bool) (string, error)
}

func TestExplain(t *testing.T) {
	ctx := context.TODO()
	conn, err := db.Connect(ctx, connectOpts...)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	plan, err := conn.Explain(ctx, "select * from complex_tag order by time desc", false)
	require.Nil(t, err)
	require.True(t, len(plan) > 0)
	require.True(t, strings.HasPrefix(plan, " PROJECT"))
	require.True(t, strings.Contains(plan, "KEYVALUE FULL SCAN"))
	require.True(t, strings.Contains(plan, "VOLATILE FULL SCAN"))
}

func TestExplainFull(t *testing.T) {
	ctx := context.TODO()
	conn, err := db.Connect(ctx, connectOpts...)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	plan, err := conn.Explain(ctx, "select * from complex_tag order by time desc", true)
	require.Nil(t, err)
	require.True(t, len(plan) > 0)
	require.True(t, strings.Contains(plan, "********"))
	require.True(t, strings.Contains(plan, " NAME           COUNT   ACCUMULATE(ms)  AVERAGE(ms)"))
}
