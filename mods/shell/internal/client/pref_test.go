package client_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

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

	pref, err := client.LoadPref()
	require.Nil(t, err)
	require.NotNil(t, pref)

	pref, err = client.LoadPrefDir("../../../tmp/machbase_pref/neoshell")
	require.Nil(t, err)
	require.NotNil(t, pref)

	itm := pref.BoxStyle()
	require.Equal(t, "light", itm.Value())
	require.Equal(t, "box style [simple,bold,double,light,round]", itm.Description())
	itm.SetValue("round")
	itm = pref.BoxStyle()
	require.Equal(t, "round", itm.Value())

	itm = pref.Heading()
	require.Nil(t, itm)

	itm = pref.Format()
	require.Nil(t, itm)

	itm = pref.TimeZone()
	require.Equal(t, time.Local, itm.TimezoneValue())
	itm.SetValue("KST")
	itm = pref.TimeZone()
	tz, _ := time.LoadLocation("Asia/Seoul")
	require.Equal(t, tz, itm.TimezoneValue())
	itm.SetValue("local")
	itm = pref.TimeZone()
	require.Equal(t, time.Local, itm.TimezoneValue())

	itm = pref.Timeformat()
	require.Equal(t, "2006-01-02 15:04:05.999", itm.Value())
	itm.SetValue("s")
	itm = pref.Timeformat()
	require.Equal(t, "s", itm.Value())
	itm.SetValue("15:04:05")
	itm = pref.Timeformat()
	require.Equal(t, "15:04:05", itm.Value())

	itm = pref.Server()
	require.Equal(t, "tcp://127.0.0.1:5655", itm.Value())

	// ERR $HOME is not defined
	itm = pref.ServerCert()
	require.Equal(t, "", itm.Value())
	itm.SetValue("server-cert")
	itm = pref.ServerCert()
	require.Equal(t, "server-cert", itm.Value())

	// ERR $HOME is not defined
	itm = pref.ClientCert()
	require.Equal(t, "", itm.Value())
	itm.SetValue("client-cert")
	itm = pref.ClientCert()
	require.Equal(t, "client-cert", itm.Value())

	// ERR $HOME is not defined
	itm = pref.ClientKey()
	require.Equal(t, "", itm.Value())
	itm.SetValue("client-key")
	itm = pref.ClientKey()
	require.Equal(t, "client-key", itm.Value())

	items := pref.Items()
	require.Equal(t, 8, len(items))
}
