package args

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestXxx(t *testing.T) {
	err := doHelp("serve", "")
	require.Nil(t, err)

	err = doHelp("shell", "sql")
	require.Nil(t, err)

	err = doHelp("timeformat", "")
	require.Nil(t, err)

	err = doHelp("tz", "")
	require.Nil(t, err)
}
