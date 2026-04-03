package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunShutdownHooksRunsInReverseOrderAndClearsHooks(t *testing.T) {
	prevHooks := shutdownHooks
	shutdownHooks = nil
	t.Cleanup(func() {
		shutdownHooks = prevHooks
	})

	var order []string
	AddShutdownHook(func() { order = append(order, "first") })
	AddShutdownHook(func() { order = append(order, "second") })
	AddShutdownHook(func() { order = append(order, "third") })

	RunShutdownHooks()

	require.Equal(t, []string{"third", "second", "first"}, order)
	require.Nil(t, shutdownHooks)

	RunShutdownHooks()
	require.Equal(t, []string{"third", "second", "first"}, order)
}
