package internal

import "testing"

func TestRowsEncoderBaseHeaders(t *testing.T) {
	var base RowsEncoderBase
	if base.HttpHeaders() != nil {
		t.Fatal("expected nil headers initially")
	}

	base.SetHttpHeader("X-Test", "one")
	base.SetHttpHeader("X-Test", "two")
	base.SetHttpHeader("X-Other", "three")

	headers := base.HttpHeaders()
	if len(headers["X-Test"]) != 2 {
		t.Fatalf("X-Test headers=%v, want 2 values", headers["X-Test"])
	}
	if len(headers["X-Other"]) != 1 {
		t.Fatalf("X-Other headers=%v, want 1 value", headers["X-Other"])
	}

	base.DelHttpHeader("X-Test")
	if _, ok := base.HttpHeaders()["X-Test"]; ok {
		t.Fatal("expected X-Test header to be deleted")
	}
}
