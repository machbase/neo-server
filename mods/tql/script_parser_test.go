package tql

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/tql/internal/expression"
)

func TestParseScript(t *testing.T) {
	script, err := ParseScript("FAKE(json({\n  [1]\n}))\nMAPVALUE(0, value(0)*10)\nCSV()", nil)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(script.Statements) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(script.Statements))
	}
	if script.Statements[0].Name != "FAKE()" || script.Statements[0].Kind != StatementSource {
		t.Fatalf("unexpected first statement: name=%s kind=%v", script.Statements[0].Name, script.Statements[0].Kind)
	}
	if script.Statements[1].Name != "MAPVALUE()" || script.Statements[1].Kind != StatementMap {
		t.Fatalf("unexpected second statement: name=%s kind=%v", script.Statements[1].Name, script.Statements[1].Kind)
	}
	if script.Statements[2].Name != "CSV()" || script.Statements[2].Kind != StatementSourceOrSink {
		t.Fatalf("unexpected third statement: name=%s kind=%v", script.Statements[2].Name, script.Statements[2].Kind)
	}
}

func TestValidateScriptStructureValid(t *testing.T) {
	script, err := ParseScript("FAKE(json({[1]}))\nMAPVALUE(0, value(0))\nCSV()", nil)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if err := ValidateScriptStructure(script); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateScriptStructureInvalidSource(t *testing.T) {
	script, err := ParseScript("MAPVALUE(0, 1)\nCSV()", nil)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	err = ValidateScriptStructure(script)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var scriptErr *ScriptError
	if !errors.As(err, &scriptErr) {
		t.Fatalf("expected ScriptError, got %T", err)
	}
	if scriptErr.Kind != "invalid_source" {
		t.Fatalf("unexpected script error kind: %s", scriptErr.Kind)
	}
}

func TestValidateScriptStructureInvalidMap(t *testing.T) {
	script, err := ParseScript("FAKE(json({[1]}))\nSQL(`select 1`)\nCSV()", nil)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	err = ValidateScriptStructure(script)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var scriptErr *ScriptError
	if !errors.As(err, &scriptErr) {
		t.Fatalf("expected ScriptError, got %T", err)
	}
	if scriptErr.Kind != "invalid_map" {
		t.Fatalf("unexpected script error kind: %s", scriptErr.Kind)
	}
}

func TestParseScriptKeepsCommentAndPragmaStatements(t *testing.T) {
	script, err := ParseScript("FAKE(json({[1]}))\n//+ stateful\n// comment\nCSV()", nil)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(script.Statements) != 4 {
		t.Fatalf("expected 4 statements, got %d", len(script.Statements))
	}
	if !script.Statements[1].IsPragma || script.Statements[1].Kind != StatementPragma {
		t.Fatalf("unexpected pragma statement: %+v", script.Statements[1])
	}
	if !script.Statements[2].IsComment || script.Statements[2].Kind != StatementComment {
		t.Fatalf("unexpected comment statement: %+v", script.Statements[2])
	}
}

func TestParseScriptMultilineStatement(t *testing.T) {
	script, err := ParseScript("FAKE(json({[1]}))\nMAPVALUE(2,\n value(1) * 10,\n true\n)\nCSV()", nil)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(script.Statements) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(script.Statements))
	}
	if script.Statements[1].Name != "MAPVALUE()" {
		t.Fatalf("unexpected multiline statement name: %s", script.Statements[1].Name)
	}
	if script.Statements[1].Line != 2 {
		t.Fatalf("unexpected multiline statement start line: %d", script.Statements[1].Line)
	}
}

func TestValidateScriptStructureNoSource(t *testing.T) {
	script := &TQLScript{}
	err := ValidateScriptStructure(script)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var scriptErr *ScriptError
	if !errors.As(err, &scriptErr) {
		t.Fatalf("expected ScriptError, got %T", err)
	}
	if scriptErr.Kind != "no_source" {
		t.Fatalf("unexpected script error kind: %s", scriptErr.Kind)
	}
}

func TestValidateScriptStructureNoSink(t *testing.T) {
	script, err := ParseScript("FAKE(json({[1]}))", nil)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	err = ValidateScriptStructure(script)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var scriptErr *ScriptError
	if !errors.As(err, &scriptErr) {
		t.Fatalf("expected ScriptError, got %T", err)
	}
	if scriptErr.Kind != "no_sink" {
		t.Fatalf("unexpected script error kind: %s", scriptErr.Kind)
	}
}

func TestValidateScriptStructureSourceOrSinkAllowedAsSource(t *testing.T) {
	script, err := ParseScript("CSV(file(\"/tmp/x.csv\"))\nTEXT()", nil)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if err := ValidateScriptStructure(script); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestParseScriptStatementSpanRawMatch(t *testing.T) {
	source := "FAKE(json({[1]})) // trailing\nMAPVALUE(2,\n value(1) * 10,\n true\n)\nCSV()"
	script, err := ParseScript(source, nil)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(script.Statements) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(script.Statements))
	}
	runes := []rune(source)

	first := script.Statements[0]
	firstRaw := first.Span.RawFrom(runes)
	if firstRaw != "FAKE(json({[1]}))" {
		t.Fatalf("unexpected first raw span: %q", firstRaw)
	}

	second := script.Statements[1]
	secondRaw := second.Span.RawFrom(runes)
	if secondRaw != second.Text {
		t.Fatalf("unexpected second raw span: %q", secondRaw)
	}
}

func TestParseScriptErrorUsesAbsoluteLineNumber(t *testing.T) {
	_, err := ParseScript("FAKE( linspace(0, 360, 50))\nMAPVALUE(1, sin((value(0)/180)*PI))\nMAPVALUE(2, cos((value(0)/180)*PI))3\nCHART()", nil)
	if err == nil {
		t.Fatal("expected parse error")
	}
	var parseErr *expression.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected ParseError, got %T", err)
	}
	if parseErr.Span.Start.Line != 3 {
		t.Fatalf("expected absolute line 3, got %d", parseErr.Span.Start.Line)
	}
	if parseErr.Near != "3" {
		t.Fatalf("expected near token 3, got %q", parseErr.Near)
	}
}

func TestParseErrorFormatsLocation(t *testing.T) {
	err := &expression.ParseError{
		Message: "unexpected token '3'",
		Near:    "3",
		Span: expression.SourceSpan{
			Start: expression.SourcePosition{Line: 3, Column: 36},
		},
	}

	formatted := err.Error()
	if formatted != "unexpected token '3' (line=3, column=36, near=\"3\")" {
		t.Fatalf("unexpected formatted error: %q", formatted)
	}
}

func TestCompileLogsAbsoluteParseErrorLocations(t *testing.T) {
	tests := []struct {
		name       string
		code       string
		expectLine int
		expectNear string
	}{
		{
			name:       "line 2 literal",
			code:       "FAKE( linspace(0, 360, 50))\nMAPVALUE(1, sin((value(0)/180)*PI))2\nCHART()",
			expectLine: 2,
			expectNear: "2",
		},
		{
			name:       "line 3 literal",
			code:       "FAKE( linspace(0, 360, 50))\nMAPVALUE(1, sin((value(0)/180)*PI))\nMAPVALUE(2, cos((value(0)/180)*PI))3\nCHART()",
			expectLine: 3,
			expectNear: "3",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			logBuf := &bytes.Buffer{}
			task := NewTaskContext(ctx)
			task.SetLogWriter(logBuf)
			task.SetConsoleLogLevel(ERROR)

			err := task.CompileString(tc.code)
			if err == nil {
				t.Fatal("expected compile error")
			}

			logOutput := logBuf.String()
			if !strings.Contains(logOutput, "Compile unexpected token '") {
				t.Fatalf("expected compile log, got %q", logOutput)
			}
			if !strings.Contains(logOutput, "line="+strconv.Itoa(tc.expectLine)) {
				t.Fatalf("expected line=%d in log, got %q", tc.expectLine, logOutput)
			}
			if !strings.Contains(logOutput, "near=\""+tc.expectNear+"\"") {
				t.Fatalf("expected near=%q in log, got %q", tc.expectNear, logOutput)
			}
		})
	}
}
