package util_test

import (
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/stretchr/testify/require"
)

func TestHelpShortcuts(t *testing.T) {
	help := util.HelpShortcuts()
	lines := strings.Split(help, "\n")

	require.Len(t, lines, 19)
	require.Equal(t, "      Ctrl + A             Beginning of line", lines[0])
	require.Contains(t, help, "Ctrl + D             Delete one character / Abort")
	require.True(t, strings.HasSuffix(help, "ESC                  Cancel / Delete whole line"))
}

func TestHelpShortcutsLegacy(t *testing.T) {
	help := util.HelpShortcutsLegacy()

	require.Contains(t, help, "\"Meta + B\" means press 'ESC' and 'B' separately.")
	require.Contains(t, help, "Shortcut in Search Mode ('Ctrl+S' or 'Ctrl+R' to enter this mode)")
	require.Contains(t, help, "Shortcut in Complete Select Mode (double 'Tab' to enter this mode)")
	require.Contains(t, help, "Meta+Backspace")
	require.Contains(t, help, "Cut previous word")
	require.Contains(t, help, "Ctrl + C / Ctrl + G")
	require.Contains(t, help, "Exit Complete Select Mode")
}
