package model

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserScopeContext(t *testing.T) {
	t.Run("round_trip", func(t *testing.T) {
		ctx := ContextWithUserScope(context.Background(), UserScope{User: "alice"})
		scope, ok := UserScopeFromContext(ctx)
		require.True(t, ok)
		require.Equal(t, UserScope{User: "alice"}, scope)
	})

	t.Run("missing", func(t *testing.T) {
		scope, ok := UserScopeFromContext(context.Background())
		require.False(t, ok)
		require.Equal(t, UserScope{}, scope)
	})

	t.Run("preserves_parent_values", func(t *testing.T) {
		type otherKey struct{}
		parent := context.WithValue(context.Background(), otherKey{}, "parent-value")
		ctx := ContextWithUserScope(parent, UserScope{User: "bob"})
		require.Equal(t, "parent-value", ctx.Value(otherKey{}))
		scope, ok := UserScopeFromContext(ctx)
		require.True(t, ok)
		require.Equal(t, "bob", scope.User)
	})

	// Guards ContextWithUserScopeFunc (machbase/neo#1468, CLI neo-shell
	// `connect`/`login` re-authentication case): unlike ContextWithUserScope,
	// the scope must be re-resolved on every lookup, not snapshotted once at
	// context-creation time, so that a later identity change (e.g. via
	// jsh/session.SwitchUser) is visible to subsequent UserScopeFromContext
	// calls sharing the same context.
	t.Run("func_variant_reflects_later_changes", func(t *testing.T) {
		current := UserScope{User: "alice"}
		ctx := ContextWithUserScopeFunc(context.Background(), func() UserScope { return current })

		scope, ok := UserScopeFromContext(ctx)
		require.True(t, ok)
		require.Equal(t, "alice", scope.User)

		current = UserScope{User: "bob"}

		scope, ok = UserScopeFromContext(ctx)
		require.True(t, ok)
		require.Equal(t, "bob", scope.User, "UserScopeFromContext must re-invoke fn, not return a stale snapshot")
	})
}

func TestProviderInputValidation(t *testing.T) {
	provider := NewProvider()
	var nilContext context.Context
	require.EqualError(t, provider.normalizeContext(nilContext), "context is nil")
	require.NoError(t, provider.normalizeContext(context.Background()))
	_, err := provider.normalizeUserScope(UserScope{})
	require.EqualError(t, err, "user scope is empty")
	user, err := provider.normalizeUserScope(UserScope{User: "  Alice "})
	require.NoError(t, err)
	require.Equal(t, "ALICE", user)

	require.EqualError(t, provider.SaveTimer(context.Background(), nil), "timer definition not specified")
	require.EqualError(t, provider.SaveSubscriber(context.Background(), nil), "subscriber definition not specified")
	require.EqualError(t, provider.SaveBridge(context.Background(), UserScope{User: "sys"}, nil), "bridge definition not specified")
	require.Error(t, provider.SetTimerRuntimeError(context.Background(), 1, "error"))
	require.Error(t, provider.SetSubscriberRuntimeError(context.Background(), 1, "error"))
}
