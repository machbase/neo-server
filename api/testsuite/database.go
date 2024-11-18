package testsuite

import (
	"context"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/stretchr/testify/require"
)

func UserAuth(t *testing.T, db api.Database, ctx context.Context) {
	ok, reason, err := db.UserAuth(ctx, "sys", "mm")
	if err != nil {
		t.Fatalf("UserAuth failed [%T]: %s", db, err.Error())
	}
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, "invalid username or password", reason)

	ok, reason, err = db.UserAuth(ctx, "sys", "manager")
	if err != nil {
		t.Fatalf("UserAuth failed: %s", err.Error())
	}
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "", reason)
}

func Ping(t *testing.T, db api.Database, ctx context.Context) {
	dur, err := db.Ping(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, dur, time.Duration(0))
}
