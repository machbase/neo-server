package testsuite

import (
	"context"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/api"
	"github.com/stretchr/testify/require"
)

func Explain(t *testing.T, db api.Database, ctx context.Context) {
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	plan, err := conn.Explain(ctx, "select * from TAG_DATA order by time desc", false)
	require.Nil(t, err)
	require.True(t, len(plan) > 0)
	require.True(t, strings.HasPrefix(plan, " PROJECT"))
	require.True(t, strings.Contains(plan, "KEYVALUE FULL SCAN"))
	require.True(t, strings.Contains(plan, "VOLATILE FULL SCAN"))
}

func ExplainFull(t *testing.T, db api.Database, ctx context.Context) {
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	plan, err := conn.Explain(ctx, "select * from TAG_DATA order by time desc", true)
	require.Nil(t, err)
	require.True(t, len(plan) > 0)
	require.True(t, strings.Contains(plan, "********"))
	require.True(t, strings.Contains(plan, " NAME           COUNT   ACCUMULATE(ms)  AVERAGE(ms)"))
}
