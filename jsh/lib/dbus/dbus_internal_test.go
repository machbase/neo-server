package dbus

import (
	"reflect"
	"testing"

	godbus "github.com/godbus/dbus/v5"
)

func TestEnsureSupportedPlatform(t *testing.T) {
	if err := ensureSupportedPlatform("linux"); err != nil {
		t.Fatalf("linux should be supported, got error: %v", err)
	}
	if err := ensureSupportedPlatform("darwin"); err == nil {
		t.Fatal("expected non-linux platform to be rejected")
	}
}

func TestParseTypedCallArg(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    any
		parsed  bool
		wantErr bool
	}{
		{name: "no-type-prefix", input: "plain-text", parsed: false},
		{name: "unknown-type-prefix", input: "custom:42", parsed: false},
		{name: "uint16", input: "uint16:123", want: uint16(123), parsed: true},
		{name: "int32", input: "int32:-9", want: int32(-9), parsed: true},
		{name: "bool", input: "bool:true", want: true, parsed: true},
		{name: "string-with-colon", input: "string:a:b:c", want: "a:b:c", parsed: true},
		{name: "objectpath", input: "objectpath:/org/freedesktop/DBus", want: godbus.ObjectPath("/org/freedesktop/DBus"), parsed: true},
		{name: "invalid-uint16", input: "uint16:not-a-number", parsed: false, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, parsed, err := parseTypedCallArg(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
			if parsed != tc.parsed {
				t.Fatalf("parse flag mismatch for %q, got %v, want %v", tc.input, parsed, tc.parsed)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("value mismatch for %q, got %#v, want %#v", tc.input, got, tc.want)
			}
		})
	}
}

func TestConvertCallArgs(t *testing.T) {
	args := []any{"uint16:7", "plain", 123}
	got, err := convertCallArgs(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("unexpected result length: got %d, want 3", len(got))
	}
	if v, ok := got[0].(uint16); !ok || v != 7 {
		t.Fatalf("arg[0] mismatch: got %#v", got[0])
	}
	if got[1] != "plain" {
		t.Fatalf("arg[1] mismatch: got %#v, want plain", got[1])
	}
	if got[2] != 123 {
		t.Fatalf("arg[2] mismatch: got %#v, want 123", got[2])
	}
}
