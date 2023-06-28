package tql

import (
	"bufio"
	"io"
	"strings"

	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/tql/fmap"
	"github.com/machbase/neo-server/mods/tql/fsink"
	"github.com/machbase/neo-server/mods/tql/fsrc"
)

var tqlFunctions = map[string]expression.Function{}

func init() {
	for _, f := range fsrc.Functions() {
		tqlFunctions[f] = nil
	}
	for _, f := range fsink.Functions() {
		tqlFunctions[f] = nil
	}
	for _, f := range fmap.Functions() {
		tqlFunctions[f] = nil
	}
}

type Line struct {
	text      string
	line      int
	isComment bool
}

func readLines(codeReader io.Reader) ([]*Line, error) {
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
		if strings.HasPrefix(strings.TrimSpace(lineText), "//") {
			expressions = append(expressions, &Line{text: strings.TrimSpace(lineText[2:]), line: lineNo, isComment: true})
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(lineText), "#") {
			expressions = append(expressions, &Line{text: strings.TrimSpace(lineText[1:]), line: lineNo, isComment: true})
			continue
		}

		aStmt := strings.Join(append(stmt, lineText), "")
		_, err = expression.ParseTokens(aStmt, tqlFunctions)
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
