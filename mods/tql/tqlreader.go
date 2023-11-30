package tql

import (
	"bufio"
	"io"
	"strings"

	"github.com/machbase/neo-server/mods/expression"
)

type Line struct {
	text      string
	line      int
	isComment bool
	isPragma  bool
}

var functions = NewNode(nil).functions

func readLines(task *Task, codeReader io.Reader) ([]*Line, error) {
	reader := bufio.NewReader(codeReader)
	parts := []byte{}
	stmt := []string{}
	expressions := []*Line{}
	lineNo := 0

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

		if strings.TrimSpace(lineText) == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(lineText), "//+") {
			expressions = append(expressions, &Line{text: strings.TrimSpace(lineText[3:]), line: lineNo, isComment: true, isPragma: true})
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(lineText), "//") {
			expressions = append(expressions, &Line{text: strings.TrimSpace(lineText[2:]), line: lineNo, isComment: true})
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(lineText), "#") {
			expressions = append(expressions, &Line{text: strings.TrimSpace(lineText[1:]), line: lineNo, isComment: true})
			continue
		}

		aStmt := strings.Join(append(stmt, lineText), " ")
		_, pos, err := expression.ParseTokens(aStmt, functions)
		if len(aStmt) > pos && len(lineText) > pos {
			lineText = lineText[0:pos]
		}
		if err != nil && err.Error() == "unbalanced parenthesis" {
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
			if len(strings.TrimSpace(line.text)) > 0 {
				expressions = append(expressions, line)
			}
			stmt = stmt[:0]
		}
	}
	return expressions, nil
}
