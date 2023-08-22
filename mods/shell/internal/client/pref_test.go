package client_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/machbase/neo-server/mods/shell/internal/client"
	"github.com/stretchr/testify/require"
)

func TestPrefDir(t *testing.T) {
	path := client.PrefDir()
	home, _ := os.UserHomeDir()
	require.Equal(t, filepath.Join(home, ".config", "machbase", "neoshell"), path)

	if runtime.GOOS == "windows" {
		os.Unsetenv("USERPROFILE")
		defer os.Setenv("USERPROFILE", home)
	} else {
		os.Unsetenv("HOME")
		defer os.Setenv("HOME", home)
	}
	path = client.PrefDir()
	binDir := "."
	if binPath, err := os.Executable(); err == nil {
		binDir = filepath.Dir(binPath)
	}
	require.Equal(t, filepath.Join(binDir, ".config", "machbase", "neoshell"), path)
}

func TestLoadPref(t *testing.T) {
	pref, err := client.LoadPref()
	require.Nil(t, err)
	require.NotNil(t, pref)

	pref, err = client.LoadPrefDir("../../../tmp/machbase_pref/neoshell")
	require.Nil(t, err)
	require.NotNil(t, pref)

	itm := pref.BoxStyle()
	require.Equal(t, "light", itm.Value())
	require.Equal(t, "box style [simple,bold,double,light,round]", itm.Description())

	// itm = pref.Heading()
	// require.Equal(t, "on", itm.Value())

	// itm = pref.TimeZone()
	// require.Equal(t, time.Local, itm.TimezoneValue())
}
