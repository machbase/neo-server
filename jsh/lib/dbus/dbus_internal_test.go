package dbus

import "testing"

func TestEnsureSupportedPlatform(t *testing.T) {
	if err := ensureSupportedPlatform("linux"); err != nil {
		t.Fatalf("linux should be supported, got error: %v", err)
	}
	if err := ensureSupportedPlatform("darwin"); err == nil {
		t.Fatal("expected non-linux platform to be rejected")
	}
}
