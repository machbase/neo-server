package expression

import (
	"errors"
	"testing"
)

func TestPrattLeftAssociativity(t *testing.T) {
	expr, err := New("10 - 3 - 2")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	v, err := expr.Evaluate(nil)
	if err != nil {
		t.Fatalf("unexpected evaluation error: %v", err)
	}
	if v != 5.0 {
		t.Fatalf("expected 5, got %#v", v)
	}
}

func TestPrattRightAssociativityExponent(t *testing.T) {
	expr, err := New("2 ** 3 ** 2")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	v, err := expr.Evaluate(nil)
	if err != nil {
		t.Fatalf("unexpected evaluation error: %v", err)
	}
	if v != 512.0 {
		t.Fatalf("expected 512, got %#v", v)
	}
}

func TestPrattNestedFunctionArguments(t *testing.T) {
	functions := map[string]Function{
		"bar": func(args ...any) (any, error) { return 4.0, nil },
		"foo": func(args ...any) (any, error) {
			return args[0].(float64) + args[1].(float64), nil
		},
	}
	expr, err := NewWithFunctions("foo(1, 2 + bar())", functions)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	v, err := expr.Evaluate(nil)
	if err != nil {
		t.Fatalf("unexpected evaluation error: %v", err)
	}
	if v != 7.0 {
		t.Fatalf("expected 7, got %#v", v)
	}
}

func TestEvaluationErrorCarriesSpan(t *testing.T) {
	expr, err := New("1 + true")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	_, err = expr.Evaluate(nil)
	if err == nil {
		t.Fatal("expected evaluation error")
	}
	var evalErr *EvaluationError
	if !errors.As(err, &evalErr) {
		t.Fatalf("expected EvaluationError, got %T", err)
	}
	if evalErr.Span.Start.Line != 1 || evalErr.Span.Start.Column != 1 {
		t.Fatalf("unexpected evaluation error span: %+v", evalErr.Span)
	}
	if evalErr.Expr != "1 + true" {
		t.Fatalf("unexpected expr on evaluation error: %q", evalErr.Expr)
	}
}
