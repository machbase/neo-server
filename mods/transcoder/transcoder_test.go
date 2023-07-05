package transcoder_test

import (
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/transcoder"
	"github.com/stretchr/testify/require"
)

func TestTranscoderScript(t *testing.T) {
	tc := transcoder.New("@transcode_echo", transcoder.OptionPath("../../test"))
	result, err := tc.Process([]any{"name.1", 1680142626000000000, 1.0})
	require.Nil(t, err)
	require.NotNil(t, result)

	arr := result.([]any)
	require.Equal(t, arr[0], "name_1")

	ts := arr[1].(time.Time)
	require.True(t, ts.UnixNano() > time.Now().Add(-10*time.Millisecond).UnixNano() && ts.UnixNano() < time.Now().UnixNano())

	require.Equal(t, arr[2], 1.0)
}
