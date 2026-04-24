package shell

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"unsafe"

	"github.com/dop251/goja"
	"github.com/hymkor/go-multiline-ny"
	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/nyaosorg/go-readline-ny"
)

type stubHistory []string

func (h stubHistory) Len() int {
	return len(h)
}

func (h stubHistory) At(i int) string {
	return h[i]
}

func TestPredictShellHistory(t *testing.T) {
	tests := []struct {
		name    string
		current string
		history readline.IHistory
		want    string
	}{
		{
			name:    "single line history",
			current: "sele",
			history: stubHistory{"help", "select * from example"},
			want:    "select * from example",
		},
		{
			name:    "prefer latest match",
			current: "sel",
			history: stubHistory{"select * from old", "select * from latest"},
			want:    "select * from latest",
		},
		{
			name:    "strip continuation marker from prediction",
			current: "echo hel",
			history: stubHistory{"echo hello \\\nworld", "noop"},
			want:    "echo hello ",
		},
		{
			name:    "do not predict on continuation line",
			current: "echo hello \\",
			history: stubHistory{"echo hello \\\nworld"},
			want:    "",
		},
		{
			name:    "ignore whitespace only current",
			current: "   ",
			history: stubHistory{"select * from example"},
			want:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := predictShellHistory(tc.current, tc.history); got != tc.want {
				t.Fatalf("predictShellHistory(%q) = %q, want %q", tc.current, got, tc.want)
			}
		})
	}
}

func TestShouldAcceptPrediction(t *testing.T) {
	tests := []struct {
		name       string
		cursor     int
		bufferLen  int
		cursorLine int
		lineCount  int
		want       bool
	}{
		{
			name:       "not at end of line",
			cursor:     2,
			bufferLen:  5,
			cursorLine: 0,
			lineCount:  1,
			want:       false,
		},
		{
			name:       "accept at end of last line",
			cursor:     5,
			bufferLen:  5,
			cursorLine: 0,
			lineCount:  1,
			want:       true,
		},
		{
			name:       "do not accept in middle line",
			cursor:     4,
			bufferLen:  4,
			cursorLine: 0,
			lineCount:  2,
			want:       false,
		},
		{
			name:       "empty line state treated as last line",
			cursor:     0,
			bufferLen:  0,
			cursorLine: 0,
			lineCount:  0,
			want:       true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldAcceptPrediction(tc.cursor, tc.bufferLen, tc.cursorLine, tc.lineCount)
			if got != tc.want {
				t.Fatalf("shouldAcceptPrediction(%d, %d, %d, %d) = %v, want %v", tc.cursor, tc.bufferLen, tc.cursorLine, tc.lineCount, got, tc.want)
			}
		})
	}
}

func TestModuleAndConstructor(t *testing.T) {
	rt := goja.New()
	module := rt.NewObject()
	exports := rt.NewObject()
	if err := module.Set("exports", exports); err != nil {
		t.Fatalf("set exports: %v", err)
	}

	Module(context.Background(), rt, module)
	if goja.IsUndefined(exports.Get("Shell")) {
		t.Fatal("exports.Shell is undefined")
	}
	if goja.IsUndefined(exports.Get("Repl")) {
		t.Fatal("exports.Repl is undefined")
	}

	obj := shell(rt)(goja.ConstructorCall{})
	if goja.IsUndefined(obj.Get("run")) {
		t.Fatal("constructed shell.run is undefined")
	}
}

func TestPromptAndSubmitBehavior(t *testing.T) {
	env := engine.NewEnv()
	env.Set("PWD", "/work/demo")
	sh := &Shell{}
	prompt := sh.prompt(env)

	var first bytes.Buffer
	if _, err := prompt(&first, 0); err != nil {
		t.Fatalf("prompt first line: %v", err)
	}
	if got, want := first.String(), "\x1b[34m/work/demo\x1B[31m >\x1B[0m "; got != want {
		t.Fatalf("prompt first line = %q, want %q", got, want)
	}

	var next bytes.Buffer
	if _, err := prompt(&next, 1); err != nil {
		t.Fatalf("prompt next line: %v", err)
	}
	if got, want := next.String(), "             "; got != want {
		t.Fatalf("prompt next line = %q, want %q", got, want)
	}

	if got := sh.submitOnEnterWhen([]string{"echo hello"}, 0); !got {
		t.Fatal("submitOnEnterWhen(single line) = false, want true")
	}
	if got := sh.submitOnEnterWhen([]string{"echo \\"}, 0); got {
		t.Fatal("submitOnEnterWhen(continuation) = true, want false")
	}
}

func TestPredictHistoryAndCompletionCandidates(t *testing.T) {
	sh := &Shell{}
	buf := &readline.Buffer{
		Editor: &readline.Editor{
			History: stubHistory{"select * from dual"},
			Cursor:  4,
		},
	}
	buf.InsertString(0, "sele")
	if got := sh.predictHistory(buf); got != "select * from dual" {
		t.Fatalf("predictHistory = %q, want %q", got, "select * from dual")
	}

	ed := &multiline.Editor{}
	sh.bindPredictionKeys(ed)

	forCompletion, forListing := sh.getCompletionCandidates([]string{"echo"})
	if forCompletion != nil || forListing != nil {
		t.Fatalf("getCompletionCandidates() = (%v, %v), want (nil, nil)", forCompletion, forListing)
	}
}

func TestCompletePath(t *testing.T) {
	fileSystem := fstest.MapFS{
		"work/file.txt":     &fstest.MapFile{Data: []byte("hello")},
		"work/folder/a.txt": &fstest.MapFile{Data: []byte("a")},
		"bin/echo.js":       &fstest.MapFile{Data: []byte("echo")},
	}
	sh := &Shell{env: engine.NewEnv(engine.WithFilesystem(fileSystem))}
	sh.env.Set("PWD", "/")

	got := sh.completePath("/wo", false)
	if len(got) != 1 || got[0].Insert != "/work/" || got[0].Kind != candidateDirectory {
		t.Fatalf("completePath(/wo) = %+v, want /work/ directory candidate", got)
	}

	got = sh.completePath("/work/fi", false)
	if len(got) != 1 || got[0].Insert != "/work/file.txt" || got[0].Kind != candidateFile {
		t.Fatalf("completePath(/work/fi) = %+v, want /work/file.txt file candidate", got)
	}

	got = sh.completePath("/work/f", true)
	if len(got) != 1 || got[0].Insert != "/work/folder/" || got[0].Kind != candidateDirectory {
		t.Fatalf("completePath(/work/f, dir only) = %+v, want /work/folder/ directory candidate", got)
	}
}

func TestCompleteCommand(t *testing.T) {
	fileSystem := fstest.MapFS{
		"bin/echo.js":         &fstest.MapFile{Data: []byte("echo")},
		"bin/env/index.js":    &fstest.MapFile{Data: []byte("env")},
		"usr/bin/which.js":    &fstest.MapFile{Data: []byte("which")},
		"work/sample.txt":     &fstest.MapFile{Data: []byte("sample")},
		"work/node_modules/x": &fstest.MapFile{Data: []byte("x")},
		"node_modules/.bin/x": &fstest.MapFile{Data: []byte("x")},
	}
	env := engine.NewEnv(engine.WithFilesystem(fileSystem))
	env.Set("PWD", "/")
	env.Set("PATH", "/bin:/usr/bin")
	env.SetAlias("ec", []string{"echo"})
	sh := &Shell{env: env}

	got := sh.completeCommand("e")
	inserts := candidateInserts(got)
	if !containsString(inserts, "echo") || !containsString(inserts, "ec") || !containsString(inserts, "env") || !containsString(inserts, "exit") {
		t.Fatalf("completeCommand(e) = %v, want echo/ec/env/exit candidates", inserts)
	}
	if containsString(inserts, "which") {
		t.Fatalf("completeCommand(e) = %v, did not expect which", inserts)
	}
}

func TestBuildCompletionContext(t *testing.T) {
	sh := &Shell{}

	ctx := sh.buildCompletionContext([]string{"echo", "a", ";", "ls", "/wo"})
	if ctx.commandName != "ls" || ctx.currentWord != "/wo" || !shouldCompletePath(ctx.commandName, ctx.currentWord) {
		t.Fatalf("buildCompletionContext(last statement) = %+v, want ls /wo path context", ctx)
	}

	ctx = sh.buildCompletionContext([]string{"echo", "hi", ">", "/wo"})
	if !ctx.expectingPath || !ctx.redirection {
		t.Fatalf("buildCompletionContext(redirection) = %+v, want redirection path context", ctx)
	}

	ctx = sh.buildCompletionContext([]string{"ec"})
	if !ctx.commandPosition || ctx.currentWord != "ec" {
		t.Fatalf("buildCompletionContext(command position) = %+v, want command position for ec", ctx)
	}
}

func TestShellCompletionCommandMetadataAndEditorState(t *testing.T) {
	cmd := newShellCompletionCommand(&Shell{})
	if got, want := cmd.String(), "SHELL_COMPLETION"; got != want {
		t.Fatalf("shellCompletionCommand.String() = %q, want %q", got, want)
	}

	ed := &multiline.Editor{}
	cmd.SetEditor(ed)
	if cmd.editor != ed {
		t.Fatal("shellCompletionCommand.SetEditor() did not store editor")
	}

	setUnexportedField(t, ed, "lines", []string{"echo hello", "ls /wo"})
	setUnexportedField(t, ed, "csrline", 1)
	fields := cmd.fieldsBeforeCurrentLine()
	if len(fields) != 2 || fields[0] != "echo" || fields[1] != "hello" {
		t.Fatalf("fieldsBeforeCurrentLine() = %v, want [echo hello]", fields)
	}
}

func TestGetCompletionCandidatesAndHelpers(t *testing.T) {
	fileSystem := fstest.MapFS{
		"work/demo.txt":    &fstest.MapFile{Data: []byte("hello")},
		"bin/echo.js":      &fstest.MapFile{Data: []byte("echo")},
		"bin/env/index.js": &fstest.MapFile{Data: []byte("env")},
	}
	env := engine.NewEnv(engine.WithFilesystem(fileSystem))
	env.Set("PWD", "/")
	env.Set("PATH", "/bin")
	env.SetAlias("ec", []string{"echo"})
	sh := &Shell{env: env}

	forCompletion, forListing := sh.getCompletionCandidates([]string{"e"})
	if !containsString(forCompletion, "echo") || !containsString(forCompletion, "ec") {
		t.Fatalf("getCompletionCandidates(command) = %v, want echo/ec", forCompletion)
	}
	if !containsString(forListing, "echo") || !containsString(forListing, "ec") {
		t.Fatalf("getCompletionCandidates(listing) = %v, want echo/ec", forListing)
	}

	forCompletion, _ = sh.getCompletionCandidates([]string{"cd", "/wo"})
	if !containsString(forCompletion, "/work/") {
		t.Fatalf("getCompletionCandidates(path) = %v, want /work/", forCompletion)
	}
}

func TestCompletionParsingHelpers(t *testing.T) {
	if !isAssignmentToken("NAME=value") {
		t.Fatal("isAssignmentToken(NAME=value) = false, want true")
	}
	if isAssignmentToken("1NAME=value") {
		t.Fatal("isAssignmentToken(1NAME=value) = true, want false")
	}

	if got := firstCommandToken([]string{"NAME=value", "cat", ">", "out.txt"}); got != "cat" {
		t.Fatalf("firstCommandToken() = %q, want cat", got)
	}

	fields := lineToFields(`echo "hello world"; cat /tmp`, `"'`, "&|><;")
	if len(fields) != 5 || fields[0] != "echo" || fields[1] != "hello world" || fields[2] != ";" || fields[3] != "cat" || fields[4] != "/tmp" {
		t.Fatalf("lineToFields() = %v, unexpected tokenization", fields)
	}

	fields, lastWordStart := splitTextFields(`echo hi >> out.txt`, len(`echo hi >> out.txt`), `"'`, "&|><;")
	if len(fields) != 4 || fields[2] != ">>" || fields[3] != "out.txt" || lastWordStart != len(`echo hi >> `) {
		t.Fatalf("splitTextFields() = (%v, %d), unexpected result", fields, lastWordStart)
	}

	buf, _ := newCompletionTestBuffer(t, `echo hi && cat /wo`)
	fields, lastWordStart = splitBufferFields(buf, `"'`, "&|><;")
	if len(fields) != 5 || fields[2] != "&&" || fields[4] != "/wo" || lastWordStart != len(`echo hi && cat `) {
		t.Fatalf("splitBufferFields() = (%v, %d), unexpected result", fields, lastWordStart)
	}

	if got := currentSegment([]string{"echo", "a", ";", "ls", "/wo"}); len(got) != 2 || got[0] != "ls" || got[1] != "/wo" {
		t.Fatalf("currentSegment() = %v, want [ls /wo]", got)
	}
}

func TestCompletionFormattingHelpers(t *testing.T) {
	if !shouldCompletePath("ls", "") {
		t.Fatal("shouldCompletePath(ls, empty) = false, want true")
	}
	if shouldCompletePath("echo", "plain") {
		t.Fatal("shouldCompletePath(echo, plain) = true, want false")
	}
	if !shouldCompletePath("echo", "$HOME/file") {
		t.Fatal("shouldCompletePath(echo, $HOME/file) = false, want true")
	}

	if !needsQuotedCompletion("my file.txt") {
		t.Fatal("needsQuotedCompletion(space) = false, want true")
	}
	if needsQuotedCompletion("plain.txt") {
		t.Fatal("needsQuotedCompletion(plain) = true, want false")
	}

	if quote, active := currentQuoteChar(`"/work/my`); quote != '"' || !active {
		t.Fatalf("currentQuoteChar(double) = (%q, %v), want (\" , true)", quote, active)
	}
	if quote, active := currentQuoteChar(`/work/my`); quote != 0 || active {
		t.Fatalf("currentQuoteChar(none) = (%q, %v), want (0, false)", quote, active)
	}

	if got := escapeCompletionText(`my"file`, '"'); got != `my\"file` {
		t.Fatalf("escapeCompletionText(double) = %q, want %q", got, `my\"file`)
	}
	if got := escapeCompletionText(`my\file`, '\''); got != `my\\file` {
		t.Fatalf("escapeCompletionText(single) = %q, want %q", got, `my\\file`)
	}

	if got := formatCompletionInsert(`/work/my`, `/work/my dir/`, candidateDirectory, false); got != `"/work/my dir/` {
		t.Fatalf("formatCompletionInsert(directory) = %q, want open quoted dir", got)
	}
	if got := formatCompletionInsert(`/work/my`, `/work/my file.txt`, candidateFile, false); got != `"/work/my file.txt" ` {
		t.Fatalf("formatCompletionInsert(file) = %q, want closed quoted file", got)
	}
	if got := formatCompletionInsert(`plain`, `echo`, candidateCommand, false); got != `echo ` {
		t.Fatalf("formatCompletionInsert(command) = %q, want command with space", got)
	}

	if got := longestCommonPrefix([]string{"alpha", "alpine", "alps"}); got != "alp" {
		t.Fatalf("longestCommonPrefix() = %q, want alp", got)
	}
}

func TestShellCompletionCommandCallQuotesSingleFilePath(t *testing.T) {
	fileSystem := fstest.MapFS{
		"work/my file.txt": &fstest.MapFile{Data: []byte("hello")},
	}
	env := engine.NewEnv(engine.WithFilesystem(fileSystem))
	env.Set("PWD", "/")
	sh := &Shell{env: env}
	cmd := newShellCompletionCommand(sh)
	buf, _ := newCompletionTestBuffer(t, `cat /work/my`)

	cmd.Call(context.Background(), buf)

	if got, want := buf.String(), `cat "/work/my file.txt" `; got != want {
		t.Fatalf("shellCompletionCommand.Call(file with space) = %q, want %q", got, want)
	}
}

func TestShellCompletionCommandCallKeepsDirectoryQuoteOpen(t *testing.T) {
	fileSystem := fstest.MapFS{
		"work/my dir/file.txt": &fstest.MapFile{Data: []byte("hello")},
	}
	env := engine.NewEnv(engine.WithFilesystem(fileSystem))
	env.Set("PWD", "/")
	sh := &Shell{env: env}
	cmd := newShellCompletionCommand(sh)
	buf, _ := newCompletionTestBuffer(t, `cd /work/my`)

	cmd.Call(context.Background(), buf)

	if got, want := buf.String(), `cd "/work/my dir/`; got != want {
		t.Fatalf("shellCompletionCommand.Call(directory with space) = %q, want %q", got, want)
	}
}

func TestShellCompletionCommandCallExtendsCommonPrefix(t *testing.T) {
	fileSystem := fstest.MapFS{
		"work/alpha.txt":  &fstest.MapFile{Data: []byte("a")},
		"work/alpine.txt": &fstest.MapFile{Data: []byte("b")},
	}
	env := engine.NewEnv(engine.WithFilesystem(fileSystem))
	env.Set("PWD", "/")
	sh := &Shell{env: env}
	cmd := newShellCompletionCommand(sh)
	buf, out := newCompletionTestBuffer(t, `cat /work/al`)

	cmd.Call(context.Background(), buf)

	if got, want := buf.String(), `cat /work/alp`; got != want {
		t.Fatalf("shellCompletionCommand.Call(common prefix) buffer = %q, want %q", got, want)
	}
	if printed := out.String(); containsSubstring(printed, "alpha.txt") || containsSubstring(printed, "alpine.txt") {
		t.Fatalf("shellCompletionCommand.Call(common prefix) output = %q, did not expect candidate list", printed)
	}
}

func TestShellCompletionCommandCallListsAmbiguousCandidates(t *testing.T) {
	fileSystem := fstest.MapFS{
		"work/alpha.txt": &fstest.MapFile{Data: []byte("a")},
		"work/amber.txt": &fstest.MapFile{Data: []byte("b")},
	}
	env := engine.NewEnv(engine.WithFilesystem(fileSystem))
	env.Set("PWD", "/")
	sh := &Shell{env: env}
	cmd := newShellCompletionCommand(sh)
	buf, out := newCompletionTestBuffer(t, `cat /work/a`)

	cmd.Call(context.Background(), buf)

	if got, want := buf.String(), `cat /work/a`; got != want {
		t.Fatalf("shellCompletionCommand.Call(ambiguous list) buffer = %q, want %q", got, want)
	}
	if printed := out.String(); !containsSubstring(printed, "alpha.txt") || !containsSubstring(printed, "amber.txt") {
		t.Fatalf("shellCompletionCommand.Call(ambiguous list) output = %q, want listed candidates", printed)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsSubstring(value string, target string) bool {
	return strings.Contains(value, target)
}

func newCompletionTestBuffer(tb testing.TB, text string) (*readline.Buffer, *bytes.Buffer) {
	tb.Helper()
	out := &bytes.Buffer{}
	ed := &readline.Editor{
		Writer: out,
		PromptWriter: func(io.Writer) (int, error) {
			return 0, nil
		},
	}
	ed.Init()

	buf := &readline.Buffer{
		Editor: ed,
		Buffer: make([]readline.Cell, 0, len(text)),
	}
	setUnexportedInt(tb, buf, "termWidth", 80+int(readline.ScrollMargin))
	buf.InsertString(0, text)
	buf.Cursor = len(buf.Buffer)
	return buf, out
}

func setUnexportedInt(tb testing.TB, target any, fieldName string, value int) {
	tb.Helper()
	field := reflect.ValueOf(target).Elem().FieldByName(fieldName)
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().SetInt(int64(value))
}

func setUnexportedField(tb testing.TB, target any, fieldName string, value any) {
	tb.Helper()
	field := reflect.ValueOf(target).Elem().FieldByName(fieldName)
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
}

func TestExecReturnsExceptionValue(t *testing.T) {
	sh := &Shell{rt: goja.New()}
	value := sh.exec("echo", []string{"hello"})
	if value == nil {
		t.Fatal("exec returned nil, want exception value")
	}
	if got := value.String(); got == "undefined" || got == "null" {
		t.Fatalf("exec returned %q, want error detail", got)
	}
}
