package util

import (
	"testing"

	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/require"
)

func TestDefaultCronLifecycle(t *testing.T) {
	prev := defaultCron
	defaultCron = nil
	t.Cleanup(func() {
		defaultCron = prev
	})

	require.Nil(t, DefaultCron())

	c := cron.New()
	SetDefaultCron(c)

	require.Same(t, c, DefaultCron())

	require.PanicsWithValue(t, "default cron already set", func() {
		SetDefaultCron(cron.New())
	})
}
