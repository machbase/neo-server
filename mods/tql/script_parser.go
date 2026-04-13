package tql

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/machbase/neo-server/v8/mods/tql/internal/expression"
)

func ParseScript(source string, functions map[string]expression.Function) (*TQLScript, error) {
	return ParseScriptReader(strings.NewReader(source), functions)
}

func absolutizeStatementParseError(err error, positions []expression.SourcePosition, stmtSpan expression.SourceSpan) error {
	var parseErr *expression.ParseError
	if !errors.As(err, &parseErr) {
		return err
	}
	adjusted := *parseErr
	startOffset := stmtSpan.Start.Offset + parseErr.Span.Start.Offset
	endOffset := stmtSpan.Start.Offset + parseErr.Span.End.Offset
	if startOffset < 0 {
		startOffset = 0
	}
	if endOffset < startOffset {
		endOffset = startOffset
	}
	if startOffset < len(positions) {
		adjusted.Span.Start = positions[startOffset]
	}
	if endOffset < len(positions) {
		adjusted.Span.End = positions[endOffset]
	}
	return &adjusted
}

func ParseScriptReader(r io.Reader, functions map[string]expression.Function) (*TQLScript, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	source := string(buf)
	if functions == nil {
		functions = NewNode(nil).functions
	}

	lines, err := scanLines(bytes.NewBuffer(buf), functions)
	if err != nil {
		return nil, err
	}

	ret := &TQLScript{Source: source}
	positions := buildSourcePositions(source)
	for _, line := range lines {
		stmt := &Statement{
			Text:      line.text,
			Line:      line.line,
			IsComment: line.isComment,
			IsPragma:  line.isPragma,
			Span:      makeStatementSpan(source, positions, line.line, line.text, line.start, line.end),
		}
		if stmt.IsPragma {
			stmt.Kind = StatementPragma
		} else if stmt.IsComment {
			stmt.Kind = StatementComment
		} else {
			var expr *expression.Expression
			if len(line.tokens) > 0 {
				expr, err = expression.NewFromTokensWithExpression(line.tokens, line.text)
			} else {
				expr, err = expression.NewWithFunctions(line.text, functions)
			}
			if err != nil {
				return nil, absolutizeStatementParseError(err, positions, stmt.Span)
			}
			stmt.Expr = expr
			stmt.Name = asNodeName(expr)
			stmt.Kind = classifyStatementKind(stmt.Name)
		}
		ret.Statements = append(ret.Statements, stmt)
	}
	return ret, nil
}

func buildSourcePositions(source string) []expression.SourcePosition {
	runes := []rune(source)
	positions := make([]expression.SourcePosition, len(runes)+1)
	line := 1
	column := 1
	for i, r := range runes {
		positions[i] = expression.SourcePosition{Offset: i, Line: line, Column: column}
		if r == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}
	positions[len(runes)] = expression.SourcePosition{Offset: len(runes), Line: line, Column: column}
	return positions
}

func makeStatementSpan(source string, positions []expression.SourcePosition, startLine int, text string, startOffset int, endOffset int) expression.SourceSpan {
	runes := []rune(source)
	if startOffset < 0 {
		startOffset = 0
	}
	if startOffset > len(runes) {
		startOffset = len(runes)
	}
	if endOffset < startOffset {
		endOffset = startOffset
	}
	if endOffset > len(runes) {
		endOffset = len(runes)
	}

	if startOffset == endOffset && len(text) > 0 {
		fallbackEnd := startOffset + len([]rune(text))
		if fallbackEnd > len(runes) {
			fallbackEnd = len(runes)
		}
		endOffset = fallbackEnd
	}

	start := positions[startOffset]
	end := positions[endOffset]
	if start.Line == 0 {
		start = expression.SourcePosition{Offset: startOffset, Line: startLine, Column: 1}
	}
	if end.Line == 0 {
		end = expression.SourcePosition{Offset: endOffset, Line: start.Line, Column: start.Column}
	}
	return expression.SourceSpan{Start: start, End: end}
}

func classifyStatementKind(name string) StatementKind {
	trimmed := strings.TrimSuffix(name, "()")
	if kind, ok := statementKindByFunctionName(trimmed); ok {
		return kind
	}
	if name != "" {
		return StatementMap
	}
	return StatementUnknown
}

type ScriptError struct {
	Kind          string
	Message       string
	Span          expression.SourceSpan
	StatementSpan expression.SourceSpan
	StatementText string
	Cause         error
}

func (e *ScriptError) Error() string {
	if e == nil {
		return ""
	}
	message := e.Message
	if e.Span.Start.Line > 0 && e.Span.Start.Column > 0 {
		message = fmt.Sprintf("line %d, column %d: %s", e.Span.Start.Line, e.Span.Start.Column, message)
	} else if e.Span.Start.Line > 0 {
		message = fmt.Sprintf("line %d: %s", e.Span.Start.Line, message)
	}
	if e.StatementText != "" {
		snippet := strings.Join(strings.Fields(e.StatementText), " ")
		if len(snippet) > 120 {
			snippet = snippet[:117] + "..."
		}
		message = fmt.Sprintf("%s [statement: %s]", message, snippet)
	}
	return message
}

func (e *ScriptError) Unwrap() error {
	return e.Cause
}

func newScriptError(kind string, stmt *Statement, message string, cause error) error {
	err := &ScriptError{
		Kind:    kind,
		Message: message,
		Cause:   cause,
	}
	if stmt != nil {
		err.Span = stmt.Span
		err.StatementSpan = stmt.Span
		err.StatementText = stmt.Text
	}
	return err
}
