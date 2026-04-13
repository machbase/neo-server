package tql

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/machbase/neo-server/v8/mods/tql/internal/expression"
)

type Line struct {
	text      string
	line      int
	isComment bool
	isPragma  bool
	tokens    []expression.Token
	start     int
	end       int
}

var functions = NewNode(nil).functions

func absolutizeParseError(err error, startLine int, startOffset int) error {
	var parseErr *expression.ParseError
	if !errors.As(err, &parseErr) {
		return err
	}
	adjusted := *parseErr
	adjusted.Span.Start.Offset = startOffset + parseErr.Span.Start.Offset
	adjusted.Span.End.Offset = startOffset + parseErr.Span.End.Offset
	if parseErr.Span.Start.Line > 0 {
		adjusted.Span.Start.Line = startLine + parseErr.Span.Start.Line - 1
	}
	if parseErr.Span.End.Line > 0 {
		adjusted.Span.End.Line = startLine + parseErr.Span.End.Line - 1
	}
	return &adjusted
}

func scanLines(codeReader io.Reader, functions map[string]expression.Function) ([]*Line, error) {
	reader := bufio.NewReader(codeReader)
	parts := []byte{}
	stmt := []string{}
	expressions := []*Line{}
	lineNo := 0
	lineFrom := 0
	lineOffset := 0
	stmtOffset := -1
	for {
		b, isPrefix, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				if len(stmt) > 0 {
					start := 0
					if stmtOffset >= 0 {
						start = stmtOffset
					}
					text := strings.Join(stmt, "\n")
					line := &Line{
						text:  text,
						line:  lineNo,
						start: start,
						end:   start + utf8.RuneCountInString(text),
					}
					if len(strings.TrimSpace(line.text)) > 0 {
						expressions = append(expressions, line)
					}
				}
				break
			}
			return nil, err
		}
		parts = append(parts, b...)
		if isPrefix {
			continue
		}
		lineNo++

		lineText := string(parts)
		lineStartOffset := lineOffset
		lineOffset += utf8.RuneCountInString(lineText)
		lineOffset += 1 // newline delimiter
		parts = parts[:0]

		trimLineText := strings.TrimSpace(lineText)
		if trimLineText == "" {
			if len(stmt) > 0 {
				stmt = append(stmt, lineText)
			}
			continue
		}
		if strings.HasPrefix(trimLineText, "#pragma") {
			pragmaText := trimLineText[7:]
			start := lineStartOffset + strings.Index(lineText, pragmaText)
			if start < lineStartOffset {
				start = lineStartOffset
			}
			expressions = append(expressions, &Line{text: pragmaText, line: lineNo, isComment: true, isPragma: true, start: start, end: start + utf8.RuneCountInString(pragmaText)})
			continue
		}
		if strings.HasPrefix(trimLineText, "//+") {
			pragmaText := trimLineText[3:]
			start := lineStartOffset + strings.Index(lineText, pragmaText)
			if start < lineStartOffset {
				start = lineStartOffset
			}
			expressions = append(expressions, &Line{text: pragmaText, line: lineNo, isComment: true, isPragma: true, start: start, end: start + utf8.RuneCountInString(pragmaText)})
			continue
		}
		if strings.HasPrefix(trimLineText, "//") {
			stmt = append(stmt, "")
			commentText := trimLineText[2:]
			start := lineStartOffset + strings.Index(lineText, commentText)
			if start < lineStartOffset {
				start = lineStartOffset
			}
			expressions = append(expressions, &Line{text: commentText, line: lineNo, isComment: true, start: start, end: start + utf8.RuneCountInString(commentText)})
			continue
		}
		if strings.HasPrefix(trimLineText, "#") {
			commentText := trimLineText[1:]
			start := lineStartOffset + strings.Index(lineText, commentText)
			if start < lineStartOffset {
				start = lineStartOffset
			}
			expressions = append(expressions, &Line{text: commentText, line: lineNo, isComment: true, start: start, end: start + utf8.RuneCountInString(commentText)})
			continue
		}

		aStmt := strings.Join(append(stmt, lineText), "\n")
		tokens, pos, err := expression.ParseTokens(aStmt, functions)
		stmtStartLine := lineNo
		stmtStartOffsetForError := lineStartOffset
		if lineFrom != 0 {
			stmtStartLine = lineFrom
		}
		if stmtOffset >= 0 {
			stmtStartOffsetForError = stmtOffset
		}
		if utf8.RuneCountInString(aStmt) > pos /* && utf8.RuneCountInString(lineText) > pos */ {
			// lineText = string([]rune(lineText)[0:pos])
			lineText = strings.TrimPrefix(string([]rune(aStmt)[0:pos]), strings.Join(stmt, "\n")+"\n")
		}
		var parseErr *expression.ParseError
		if err != nil && errors.As(err, &parseErr) && parseErr.Kind == "unbalanced_parenthesis" {
			if lineFrom == 0 {
				lineFrom = lineNo
				stmtOffset = lineStartOffset
			}
			stmt = append(stmt, lineText)
			continue
		} else if err != nil {
			return nil, absolutizeParseError(err, stmtStartLine, stmtStartOffsetForError)
		} else {
			start := lineStartOffset
			if lineFrom != 0 && stmtOffset >= 0 {
				start = stmtOffset
			}
			stmt = append(stmt, lineText)

			line := &Line{
				text:   strings.Join(stmt, "\n"),
				line:   lineNo,
				tokens: tokens,
				start:  start,
				end:    start + pos,
			}
			if lineFrom != 0 {
				line.line = lineFrom
			}
			if len(strings.TrimSpace(line.text)) > 0 {
				expressions = append(expressions, line)
			}
			stmt = stmt[:0]
			lineFrom = 0
			stmtOffset = -1
		}
	}
	return expressions, nil
}
