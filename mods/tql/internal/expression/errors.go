package expression

import "fmt"

type ParseError struct {
	Kind    string
	Message string
	Span    SourceSpan
	Near    string
	Cause   error
}

func (e *ParseError) Error() string {
	if e == nil {
		return ""
	}
	if e.Span.Start.Line > 0 && e.Span.Start.Column > 0 {
		if e.Near != "" {
			return fmt.Sprintf("%s (line=%d, column=%d, near=%q)", e.Message, e.Span.Start.Line, e.Span.Start.Column, e.Near)
		}
		return fmt.Sprintf("%s (line=%d, column=%d)", e.Message, e.Span.Start.Line, e.Span.Start.Column)
	}
	return e.Message
}

func (e *ParseError) Unwrap() error {
	return e.Cause
}

func newParseError(kind string, span SourceSpan, near string, message string, cause error) error {
	return &ParseError{
		Kind:    kind,
		Message: message,
		Span:    span,
		Near:    near,
		Cause:   cause,
	}
}

type EvaluationError struct {
	Kind    string
	Message string
	Span    SourceSpan
	Expr    string
	Cause   error
}

func (e *EvaluationError) Error() string {
	return e.Message
}

func (e *EvaluationError) Unwrap() error {
	return e.Cause
}
