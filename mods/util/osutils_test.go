package util_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/mods/util"
)

func TestRandomString(t *testing.T) {
	result := util.RandomString(10)
	if len(result) != 10 {
		t.Fatal("invalid result, ", result)
	}
}
