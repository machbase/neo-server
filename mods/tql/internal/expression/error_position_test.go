package expression

import (
	"errors"
	"testing"
)

func TestTokenSpanTracking(t *testing.T) {
	tokens, _, err := ParseTokens("foo + 10\nbar", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tokens) != 4 {
		t.Fatalf("expected 4 tokens, got %d", len(tokens))
	}

	if tokens[0].Raw != "foo" {
		t.Fatalf("expected raw token foo, got %q", tokens[0].Raw)
	}
	if tokens[0].Span.Start.Line != 1 || tokens[0].Span.Start.Column != 1 {
		t.Fatalf("unexpected foo start position: %+v", tokens[0].Span.Start)
	}
	if tokens[1].Raw != "+" || tokens[1].Span.Start.Column != 5 {
		t.Fatalf("unexpected plus token position: raw=%q span=%+v", tokens[1].Raw, tokens[1].Span)
	}
	if tokens[3].Raw != "bar" || tokens[3].Span.Start.Line != 2 || tokens[3].Span.Start.Column != 1 {
		t.Fatalf("unexpected bar token position: raw=%q span=%+v", tokens[3].Raw, tokens[3].Span)
	}
}

func TestParseErrorInvalidTokenPosition(t *testing.T) {
	_, _, err := ParseTokens("1 @ 2", nil)
	if err == nil {
		t.Fatal("expected parse error")
	}

	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected ParseError, got %T", err)
	}

	if parseErr.Kind != "invalid_token" {
		t.Fatalf("unexpected parse error kind: %s", parseErr.Kind)
	}
	if parseErr.Span.Start.Line != 1 || parseErr.Span.Start.Column != 3 {
		t.Fatalf("unexpected invalid token position: %+v", parseErr.Span.Start)
	}
	if parseErr.Near != "@" {
		t.Fatalf("unexpected near text: %q", parseErr.Near)
	}
}

func TestParseErrorUnexpectedEndPosition(t *testing.T) {
	_, err := New("1 +")
	if err == nil {
		t.Fatal("expected parse error")
	}

	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected ParseError, got %T", err)
	}

	if parseErr.Kind != "unexpected_end" {
		t.Fatalf("unexpected parse error kind: %s", parseErr.Kind)
	}
	if parseErr.Span.Start.Line != 1 || parseErr.Span.Start.Column != 4 {
		t.Fatalf("unexpected unexpected-end position: %+v", parseErr.Span.Start)
	}
	if parseErr.Near != "+" {
		t.Fatalf("unexpected near text: %q", parseErr.Near)
	}
}

func TestParseErrorUnbalancedParenthesisPosition(t *testing.T) {
	_, err := New("(1 + 2")
	if err == nil {
		t.Fatal("expected parse error")
	}

	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected ParseError, got %T", err)
	}

	if parseErr.Kind != "unbalanced_parenthesis" {
		t.Fatalf("unexpected parse error kind: %s", parseErr.Kind)
	}
	if parseErr.Span.Start.Line != 1 || parseErr.Span.Start.Column != 1 {
		t.Fatalf("unexpected unbalanced parenthesis position: %+v", parseErr.Span.Start)
	}
	if parseErr.Near != "(" {
		t.Fatalf("unexpected near text: %q", parseErr.Near)
	}
}
