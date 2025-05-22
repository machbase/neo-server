package tql

import (
	"bufio"
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
}

var functions = NewNode(nil).functions

func readLines(_ *Task, codeReader io.Reader) ([]*Line, error) {
	reader := bufio.NewReader(codeReader)
	parts := []byte{}
	stmt := []string{}
	expressions := []*Line{}
	lineNo := 0
	lineFrom := 0
	for {
		b, isPrefix, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				if len(stmt) > 0 {
					line := &Line{
						text: strings.Join(stmt, "\n"),
						line: lineNo,
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
		parts = parts[:0]

		trimLineText := strings.TrimSpace(lineText)
		if trimLineText == "" {
			continue
		}
		if strings.HasPrefix(trimLineText, "#pragma") {
			expressions = append(expressions, &Line{text: trimLineText[7:], line: lineNo, isComment: true, isPragma: true})
			continue
		}
		if strings.HasPrefix(trimLineText, "//+") {
			expressions = append(expressions, &Line{text: trimLineText[3:], line: lineNo, isComment: true, isPragma: true})
			continue
		}
		if strings.HasPrefix(trimLineText, "//") {
			stmt = append(stmt, "")
			expressions = append(expressions, &Line{text: trimLineText[2:], line: lineNo, isComment: true})
			continue
		}
		if strings.HasPrefix(trimLineText, "#") {
			expressions = append(expressions, &Line{text: trimLineText[1:], line: lineNo, isComment: true})
			continue
		}

		aStmt := strings.Join(append(stmt, lineText), "\n")
		_, pos, err := expression.ParseTokens(aStmt, functions)
		if utf8.RuneCountInString(aStmt) > pos /* && utf8.RuneCountInString(lineText) > pos */ {
			// lineText = string([]rune(lineText)[0:pos])
			lineText = strings.TrimPrefix(string([]rune(aStmt)[0:pos]), strings.Join(stmt, "\n")+"\n")
		}
		if err != nil && err.Error() == "unbalanced parenthesis" {
			if lineFrom == 0 {
				lineFrom = lineNo
			}
			stmt = append(stmt, lineText)
			continue
		} else if err != nil {
			return nil, err
		} else {
			stmt = append(stmt, lineText)

			line := &Line{
				text: strings.Join(stmt, "\n"),
				line: lineNo,
			}
			if lineFrom != 0 {
				line.line = lineFrom
			}
			if len(strings.TrimSpace(line.text)) > 0 {
				expressions = append(expressions, line)
			}
			stmt = stmt[:0]
			lineFrom = 0
		}
	}
	return expressions, nil
}
