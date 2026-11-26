package util

import (
	"testing"
	"time"

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

func TestRunShutdownHookCanRegisterAnotherHook(t *testing.T) {
	prevHooks := shutdownHooks
	shutdownHooks = nil
	t.Cleanup(func() {
		shutdownHooks = prevHooks
	})

	var order []string
	AddShutdownHook(func() {
		order = append(order, "first")
		AddShutdownHook(func() { order = append(order, "registered") })
	})

	done := make(chan struct{})
	go func() {
		RunShutdownHooks()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown hook deadlocked while registering another hook")
	}

	require.Equal(t, []string{"first"}, order)
	RunShutdownHooks()
	require.Equal(t, []string{"first", "registered"}, order)
}
