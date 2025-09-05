package metric

import (
	"testing"
)

func TestAllowName(t *testing.T) {
	type args struct {
		measure string
		field   string
	}
	tests := []struct {
		name     string
		args     args
		patterns []string
		allowed  bool
	}{
		{
			name:     "exact match",
			args:     args{"cpu", "usage"},
			patterns: []string{"cpu:usage"},
			allowed:  true,
		},
		{
			name:     "wildcard match",
			args:     args{"cpu", "usage"},
			patterns: []string{"cpu:*"},
			allowed:  true,
		},
		{
			name:     "question mark match",
			args:     args{"cpu", "user"},
			patterns: []string{"cpu:us?r"},
			allowed:  true,
		},
		{
			name:     "no match",
			args:     args{"mem", "usage"},
			patterns: []string{"cpu:*"},
			allowed:  false,
		},
		{
			name:     "multiple patterns, one matches",
			args:     args{"disk", "read"},
			patterns: []string{"cpu:*", "disk:read"},
			allowed:  true,
		},
		{
			name:     "multiple patterns, none match",
			args:     args{"net", "in"},
			patterns: []string{"cpu:*", "disk:*"},
			allowed:  false,
		},
	}

	for _, tt := range tests {
		called := false
		of := func(p Product) {
			called = true
		}
		filter := AllowNameFilter(of, tt.patterns...)
		filter(Product{
			Measure: tt.args.measure,
			Field:   tt.args.field,
		})
		if called != tt.allowed {
			t.Errorf("%s: expected allowed=%v, got %v", tt.name, tt.allowed, called)
		}
	}
}

func TestDenyName(t *testing.T) {
	type args struct {
		measure string
		field   string
	}
	tests := []struct {
		name     string
		args     args
		patterns []string
		allowed  bool
	}{
		{
			name:     "exact deny match",
			args:     args{"cpu", "usage"},
			patterns: []string{"cpu:usage"},
			allowed:  false,
		},
		{
			name:     "wildcard deny match",
			args:     args{"cpu", "usage"},
			patterns: []string{"cpu:*"},
			allowed:  false,
		},
		{
			name:     "question mark deny match",
			args:     args{"cpu", "user"},
			patterns: []string{"cpu:us?r"},
			allowed:  false,
		},
		{
			name:     "no deny match",
			args:     args{"mem", "usage"},
			patterns: []string{"cpu:*"},
			allowed:  true,
		},
		{
			name:     "multiple patterns, one denies",
			args:     args{"disk", "read"},
			patterns: []string{"cpu:*", "disk:read"},
			allowed:  false,
		},
		{
			name:     "multiple patterns, none deny",
			args:     args{"net", "in"},
			patterns: []string{"cpu:*", "disk:*"},
			allowed:  true,
		},
	}

	for _, tt := range tests {
		called := false
		of := func(p Product) {
			called = true
		}
		filter := DenyNameFilter(of, tt.patterns...)
		filter(Product{
			Measure: tt.args.measure,
			Field:   tt.args.field,
		})
		if called != tt.allowed {
			t.Errorf("%s: expected allowed=%v, got %v", tt.name, tt.allowed, called)
		}
	}
}
