package session

import "testing"

func TestConfigureExtFlagsGet(t *testing.T) {
	flags := ExtFlags{
		{flag: "server", value: "localhost"},
		{flag: "user", value: "sys"},
	}

	if ef := flags.Get("server"); ef == nil || ef.value != "localhost" {
		t.Fatalf("Get('server') = %v", ef)
	}
	if ef := flags.Get("user"); ef == nil || ef.value != "sys" {
		t.Fatalf("Get('user') = %v", ef)
	}
	if ef := flags.Get("nonexistent"); ef != nil {
		t.Fatalf("Get('nonexistent') = %v, want nil", ef)
	}
}
