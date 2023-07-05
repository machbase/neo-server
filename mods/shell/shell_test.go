package shell

import (
	"testing"

	"github.com/machbase/neo-server/mods/shell/internal/client"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	cmd := client.FindCmd("help")
	require.NotNil(t, cmd)
}
