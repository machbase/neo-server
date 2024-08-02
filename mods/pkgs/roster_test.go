package pkgs_test

import (
	"github.com/machbase/neo-server/mods/pkgs"
)

func ExamplePkgManager_Sync() {
	roster, err := pkgs.NewRoster("./tmp/roster", "./tmp/pkgs")
	if err != nil {
		panic(err)
	}
	err = roster.Sync()
	if err != nil {
		panic(err)
	}
	// Output:
}
