package metric

import (
	"testing"
)

func TestAllowName(t *testing.T) {
	tests := []struct {
		name     string
		args     string
		patterns []string
		allowed  bool
	}{
		{
			name:     "exact match",
			args:     "cpu:usage",
			patterns: []string{"cpu:usage"},
			allowed:  true,
		},
		{
			name:     "wildcard match",
			args:     "cpu:usage",
			patterns: []string{"cpu:*"},
			allowed:  true,
		},
		{
			name:     "question mark match",
			args:     "cpu:user",
			patterns: []string{"cpu:us?r"},
			allowed:  true,
		},
		{
			name:     "no match",
			args:     "mem:usage",
			patterns: []string{"cpu:*"},
			allowed:  false,
		},
		{
			name:     "multiple patterns, one matches",
			args:     "disk:read",
			patterns: []string{"cpu:*", "disk:read"},
			allowed:  true,
		},
		{
			name:     "multiple patterns, none match",
			args:     "net:in",
			patterns: []string{"cpu:*", "disk:*"},
			allowed:  false,
		},
	}

	for _, tt := range tests {
		called := false
		of := func(p Product) error {
			called = true
			return nil
		}
		filter := IncludeNames(of, tt.patterns...)
		filter(Product{
			Name: tt.args,
		})
		if called != tt.allowed {
			t.Errorf("%s: expected allowed=%v, got %v", tt.name, tt.allowed, called)
		}
	}
}

func TestDenyName(t *testing.T) {
	tests := []struct {
		name     string
		args     string
		patterns []string
		allowed  bool
	}{
		{
			name:     "exact deny match",
			args:     "cpu:usage",
			patterns: []string{"cpu:usage"},
			allowed:  false,
		},
		{
			name:     "wildcard deny match",
			args:     "cpu:usage",
			patterns: []string{"cpu:*"},
			allowed:  false,
		},
		{
			name:     "question mark deny match",
			args:     "cpu:user",
			patterns: []string{"cpu:us?r"},
			allowed:  false,
		},
		{
			name:     "no deny match",
			args:     "mem:usage",
			patterns: []string{"cpu:*"},
			allowed:  true,
		},
		{
			name:     "multiple patterns, one denies",
			args:     "disk:read",
			patterns: []string{"cpu:*", "disk:read"},
			allowed:  false,
		},
		{
			name:     "multiple patterns, none deny",
			args:     "net:in",
			patterns: []string{"cpu:*", "disk:*"},
			allowed:  true,
		},
	}

	for _, tt := range tests {
		called := false
		of := func(p Product) error {
			called = true
			return nil
		}
		filter := ExcludeNames(of, tt.patterns...)
		filter(Product{
			Name: tt.args,
		})
		if called != tt.allowed {
			t.Errorf("%s: expected allowed=%v, got %v", tt.name, tt.allowed, called)
		}
	}
}

func TestCompilePatterns(t *testing.T) {
	tests := []struct {
		pattern    []string
		separators []rune
		input      string
		want       bool
	}{
		{[]string{"abc", "def", "ghi*"}, nil, "abc", true},
		{[]string{"abc", "def", "ghi*"}, nil, "def", true},
		{[]string{"abc", "def", "ghi*"}, nil, "ghibelline", true},
		{[]string{"abc", "def", "ghi*"}, nil, "defy", false},
		{[]string{"abc", "def", "ghi*"}, nil, "xyz", false},
		{[]string{"abc:*:def"}, []rune{':'}, "abc:def", false},
		{[]string{"abc:*:def"}, []rune{':'}, "abc:xyz:def", true},
		{[]string{"abc:*:def"}, []rune{':'}, "abc:opq:xyz:ghi", false},
		{[]string{"abc:*:def"}, []rune{':'}, "abc:foo:def", true},
		{[]string{"metric:field[0-3]"}, []rune{':'}, "metric:field0", true},
		{[]string{"metric:field[0-3]"}, []rune{':'}, "metric:field1", true},
		{[]string{"metric:field[0-3]"}, []rune{':'}, "metric:field2", true},
		{[]string{"metric:field[0-3]"}, []rune{':'}, "metric:field3", true},
		{[]string{"metric:field[0-3]"}, []rune{':'}, "metric:field4", false},
		{[]string{"metric:field[0-3]"}, []rune{':'}, "metric:field", false},
		{[]string{"metric:field[0-3]"}, []rune{':'}, "metric:field10", false},
		{[]string{"abc", "metric:field[1-2]"}, []rune{':'}, "abc", true},
		{[]string{"abc", "metric:field[1-2]"}, []rune{':'}, "metric:field1", true},
		{[]string{"abc", "metric:field[1-2]"}, []rune{':'}, "metric:field2", true},
		{[]string{"abc", "metric:field[1-2]"}, []rune{':'}, "metric:field3", false},
		{[]string{"abc", "metric:field:[1-2]"}, []rune{':'}, "metric:field:1", true},
		{[]string{"abc", "metric:field:[1-2]"}, []rune{':'}, "metric:field:2", true},
		{[]string{"abc", "metric:field:[1-2]"}, []rune{':'}, "metric:field:3", false},
	}

	for _, tt := range tests {
		f, err := Compile(tt.pattern, tt.separators...)
		if err != nil {
			t.Fatalf("Compile returned error: %v", err)
		}
		got := f.Match(tt.input)
		if got != tt.want {
			t.Errorf("Match(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestCompileEmptyPatterns(t *testing.T) {
	f, err := Compile([]string{})
	if err != nil {
		t.Fatalf("Compile returned error: %v", err)
	}
	if f != nil {
		t.Errorf("Expected nil filter for empty patterns, got %v", f)
	}
}

func TestAndFilter(t *testing.T) {
	trueFilter := MustCompile([]string{"foo"})
	falseFilter := MustCompile([]string{"bar"})

	and := AndFilter(trueFilter, trueFilter)
	if !and.Match("foo") {
		t.Error("AndFilter: expected true when both filters match")
	}

	and = AndFilter(trueFilter, falseFilter)
	if and.Match("foo") {
		t.Error("AndFilter: expected false when one filter does not match")
	}

	and = AndFilter(falseFilter, falseFilter)
	if and.Match("foo") {
		t.Error("AndFilter: expected false when both filters do not match")
	}

	and = AndFilter(nil, trueFilter)
	if !and.Match("foo") {
		t.Error("AndFilter: expected true when one filter is nil and the other matches")
	}

	and = AndFilter(nil, nil)
	if and != nil {
		t.Error("AndFilter: expected nil when both filters are nil")
	}
}

func TestOrFilter(t *testing.T) {
	trueFilter := MustCompile([]string{"foo"})
	falseFilter := MustCompile([]string{"bar"})

	or := OrFilter(trueFilter, trueFilter)
	if !or.Match("foo") {
		t.Error("OrFilter: expected true when both filters match")
	}

	or = OrFilter(trueFilter, falseFilter)
	if !or.Match("foo") {
		t.Error("OrFilter: expected true when one filter matches")
	}

	or = OrFilter(falseFilter, falseFilter)
	if or.Match("baz") {
		t.Error("OrFilter: expected false when both filters do not match")
	}

	or = OrFilter(nil, trueFilter)
	if !or.Match("foo") {
		t.Error("OrFilter: expected true when one filter is nil and the other matches")
	}

	or = OrFilter(nil, nil)
	if or != nil {
		t.Error("OrFilter: expected nil when both filters are nil")
	}
}
