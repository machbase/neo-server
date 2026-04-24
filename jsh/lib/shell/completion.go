package shell

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/hymkor/go-multiline-ny"
	"github.com/nyaosorg/go-readline-ny"
)

type candidateKind int

const (
	candidateCommand candidateKind = iota
	candidateFile
	candidateDirectory
)

type completionCandidate struct {
	Insert  string
	Display string
	Kind    candidateKind
}

type completionContext struct {
	fields          []string
	segment         []string
	currentWord     string
	commandName     string
	commandPosition bool
	expectingPath   bool
	onlyDirectory   bool
	redirection     bool
}

type shellCompletionCommand struct {
	shell     *Shell
	editor    *multiline.Editor
	delimiter string
	enclosure string
}

func newShellCompletionCommand(sh *Shell) *shellCompletionCommand {
	return &shellCompletionCommand{
		shell:     sh,
		delimiter: "&|><;",
		enclosure: `"'`,
	}
}

func (cmd *shellCompletionCommand) String() string {
	return "SHELL_COMPLETION"
}

func (cmd *shellCompletionCommand) SetEditor(ed *multiline.Editor) {
	cmd.editor = ed
}

func (cmd *shellCompletionCommand) Call(_ context.Context, buffer *readline.Buffer) readline.Result {
	fieldsBeforeCurrentLine := cmd.fieldsBeforeCurrentLine()
	fieldsBeforeCursor, lastWordStart := splitBufferFields(buffer, cmd.enclosure, cmd.delimiter)
	rawCurrentWord := buffer.SubString(lastWordStart, buffer.Cursor)
	fields := make([]string, 0, len(fieldsBeforeCurrentLine)+len(fieldsBeforeCursor))
	fields = append(fields, fieldsBeforeCurrentLine...)
	fields = append(fields, fieldsBeforeCursor...)

	candidates := cmd.shell.completionCandidates(fields)
	if len(candidates) == 0 {
		return readline.CONTINUE
	}

	context := cmd.shell.buildCompletionContext(fields)
	if len(candidates) == 1 {
		insert := formatCompletionInsert(rawCurrentWord, candidates[0].Insert, candidates[0].Kind, false)
		buffer.ReplaceAndRepaint(lastWordStart, insert)
		return readline.CONTINUE
	}

	commonPrefix := longestCommonPrefix(candidateInserts(candidates))
	if len(commonPrefix) > len(context.currentWord) {
		commonKind := candidateFile
		allDirectories := true
		for _, candidate := range candidates {
			if candidate.Kind != candidateDirectory {
				allDirectories = false
				break
			}
		}
		if allDirectories {
			commonKind = candidateDirectory
		}
		buffer.ReplaceAndRepaint(lastWordStart, formatCompletionInsert(rawCurrentWord, commonPrefix, commonKind, true))
		return readline.CONTINUE
	}

	fmt.Fprintln(buffer.Out)
	for _, line := range candidateDisplays(candidates) {
		fmt.Fprintln(buffer.Out, line)
	}
	buffer.RepaintAll()
	return readline.CONTINUE
}

func (cmd *shellCompletionCommand) fieldsBeforeCurrentLine() []string {
	if cmd.editor == nil || cmd.editor.CursorLine() <= 0 {
		return nil
	}
	fields := []string{}
	for _, line := range cmd.editor.Lines()[:cmd.editor.CursorLine()] {
		fields = append(fields, lineToFields(line, cmd.enclosure, cmd.delimiter)...)
	}
	return fields
}

func (sh *Shell) getCompletionCandidates(fields []string) (forCompletion []string, forListing []string) {
	for _, candidate := range sh.completionCandidates(fields) {
		forCompletion = append(forCompletion, candidate.Insert)
		forListing = append(forListing, candidate.Display)
	}
	return forCompletion, forListing
}

func (sh *Shell) completionCandidates(fields []string) []completionCandidate {
	if sh == nil || sh.env == nil {
		return nil
	}
	ctx := sh.buildCompletionContext(fields)
	if ctx.commandPosition {
		return sh.completeCommand(ctx.currentWord)
	}
	if ctx.expectingPath || ctx.onlyDirectory || shouldCompletePath(ctx.commandName, ctx.currentWord) {
		return sh.completePath(ctx.currentWord, ctx.onlyDirectory)
	}
	return nil
}

func (sh *Shell) buildCompletionContext(fields []string) completionContext {
	ctx := completionContext{fields: append([]string{}, fields...)}
	if len(fields) == 0 {
		ctx.commandPosition = true
		return ctx
	}

	if isCompletionOperator(fields[len(fields)-1]) {
		ctx.currentWord = ""
		ctx.segment = currentSegment(fields)
	} else {
		ctx.currentWord = fields[len(fields)-1]
		ctx.segment = currentSegment(fields[:len(fields)-1])
	}

	ctx.commandName = firstCommandToken(ctx.segment)
	ctx.commandPosition = ctx.commandName == ""
	if len(ctx.segment) > 0 {
		prev := ctx.segment[len(ctx.segment)-1]
		if isRedirectionOperator(prev) {
			ctx.redirection = true
			ctx.expectingPath = true
		}
	}
	if ctx.commandName == "cd" {
		ctx.expectingPath = true
		ctx.onlyDirectory = true
	}
	return ctx
}

func currentSegment(fields []string) []string {
	start := 0
	for idx, field := range fields {
		if field == "|" || field == ";" || field == "&&" {
			start = idx + 1
		}
	}
	return append([]string{}, fields[start:]...)
}

func firstCommandToken(segment []string) string {
	skipNext := false
	for _, field := range segment {
		if skipNext {
			skipNext = false
			continue
		}
		if isRedirectionOperator(field) {
			skipNext = true
			continue
		}
		if isCompletionOperator(field) || isAssignmentToken(field) || field == "" {
			continue
		}
		return field
	}
	return ""
}

func shouldCompletePath(commandName string, currentWord string) bool {
	if currentWord == "" {
		return commandName == "ls" || commandName == "cat"
	}
	if strings.HasPrefix(currentWord, "/") || strings.HasPrefix(currentWord, "./") || strings.HasPrefix(currentWord, "../") || strings.HasPrefix(currentWord, "~/") {
		return true
	}
	if strings.HasPrefix(currentWord, "$") || strings.Contains(currentWord, "/") {
		return true
	}
	return commandName == "ls" || commandName == "cat"
}

func isAssignmentToken(token string) bool {
	if token == "" {
		return false
	}
	idx := strings.IndexByte(token, '=')
	if idx <= 0 {
		return false
	}
	name := token[:idx]
	for i, ch := range name {
		if i == 0 {
			if !(ch == '_' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z') {
				return false
			}
			continue
		}
		if !(ch == '_' || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9') {
			return false
		}
	}
	return true
}

func isCompletionOperator(token string) bool {
	return token == "" || token == "|" || token == ";" || token == "&&" || isRedirectionOperator(token)
}

func isRedirectionOperator(token string) bool {
	return token == "<" || token == ">" || token == ">>"
}

func (sh *Shell) completeCommand(prefix string) []completionCandidate {
	if sh == nil || sh.env == nil {
		return nil
	}
	seen := map[string]struct{}{}
	results := []completionCandidate{}
	appendCandidate := func(name string) {
		if !strings.HasPrefix(name, prefix) {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		results = append(results, completionCandidate{Insert: name, Display: name, Kind: candidateCommand})
	}

	for _, name := range []string{"alias", "cd", "setenv", "unsetenv", "which", "exit", "quit"} {
		appendCandidate(name)
	}

	aliasNames := make([]string, 0, len(sh.env.Aliases()))
	for name := range sh.env.Aliases() {
		aliasNames = append(aliasNames, name)
	}
	sort.Strings(aliasNames)
	for _, name := range aliasNames {
		appendCandidate(name)
	}

	if sh.env.Filesystem() == nil {
		return results
	}
	if pathValue, ok := sh.env.Get("PATH").(string); ok {
		for _, dir := range strings.Split(pathValue, ":") {
			resolvedDir := sh.env.ResolveAbsPath(dir)
			entries, err := sh.readDir(resolvedDir)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				name := entry.Name()
				if entry.IsDir() {
					if sh.pathExists(path.Join(resolvedDir, name, "index.js")) {
						appendCandidate(name)
					}
					continue
				}
				if strings.HasSuffix(name, ".js") {
					appendCandidate(strings.TrimSuffix(name, ".js"))
				}
			}
		}
	}
	return results
}

func (sh *Shell) completePath(prefix string, onlyDirectory bool) []completionCandidate {
	if sh == nil || sh.env == nil {
		return nil
	}
	if sh.env.Filesystem() == nil {
		return nil
	}
	typedDirPrefix, typedBasePrefix := splitTypedPath(prefix)
	lookupDir := typedDirPrefix
	if lookupDir == "" {
		lookupDir = "."
	}
	resolvedDir := sh.env.ResolveAbsPath(lookupDir)
	entries, err := sh.readDir(resolvedDir)
	if err != nil {
		return nil
	}

	results := make([]completionCandidate, 0)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, typedBasePrefix) {
			continue
		}
		if onlyDirectory && !entry.IsDir() {
			continue
		}
		insert := joinTypedPath(typedDirPrefix, name)
		display := insert
		kind := candidateFile
		if entry.IsDir() {
			insert += "/"
			display += "/"
			kind = candidateDirectory
		}
		results = append(results, completionCandidate{Insert: insert, Display: display, Kind: kind})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Kind != results[j].Kind {
			return results[i].Kind == candidateDirectory
		}
		return results[i].Display < results[j].Display
	})
	return results
}

func splitTypedPath(prefix string) (string, string) {
	idx := strings.LastIndex(prefix, "/")
	if idx == -1 {
		return "", prefix
	}
	return prefix[:idx+1], prefix[idx+1:]
}

func joinTypedPath(dirPrefix string, entryName string) string {
	switch dirPrefix {
	case "", ".":
		return entryName
	default:
		return dirPrefix + entryName
	}
}

func (sh *Shell) pathExists(name string) bool {
	if sh == nil || sh.env == nil || sh.env.Filesystem() == nil {
		return false
	}
	fd, err := sh.openPath(name)
	if err != nil {
		return false
	}
	fd.Close()
	return true
}

func (sh *Shell) readDir(name string) ([]fs.DirEntry, error) {
	if sh == nil || sh.env == nil || sh.env.Filesystem() == nil {
		return nil, fs.ErrInvalid
	}
	entries, err := fs.ReadDir(sh.env.Filesystem(), name)
	if err == nil {
		return entries, nil
	}
	alt := strings.TrimPrefix(name, "/")
	if alt == "" {
		alt = "."
	}
	if alt == name {
		return nil, err
	}
	return fs.ReadDir(sh.env.Filesystem(), alt)
}

func (sh *Shell) openPath(name string) (fs.File, error) {
	if sh == nil || sh.env == nil || sh.env.Filesystem() == nil {
		return nil, fs.ErrInvalid
	}
	fd, err := sh.env.Filesystem().Open(name)
	if err == nil {
		return fd, nil
	}
	alt := strings.TrimPrefix(name, "/")
	if alt == "" {
		alt = "."
	}
	if alt == name {
		return nil, err
	}
	return sh.env.Filesystem().Open(alt)
}

func candidateInserts(candidates []completionCandidate) []string {
	results := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		results = append(results, candidate.Insert)
	}
	return results
}

func candidateDisplays(candidates []completionCandidate) []string {
	results := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		results = append(results, candidate.Display)
	}
	return results
}

func formatCompletionInsert(rawWord string, insert string, kind candidateKind, keepEditing bool) string {
	if kind == candidateCommand {
		if keepEditing {
			return insert
		}
		return insert + " "
	}

	needsQuote := needsQuotedCompletion(insert)
	quoteChar, activeQuote := currentQuoteChar(rawWord)
	if !activeQuote && !needsQuote {
		if kind == candidateDirectory || keepEditing {
			return insert
		}
		return insert + " "
	}
	if quoteChar == 0 {
		quoteChar = '"'
	}
	if quoteChar == '\'' && strings.ContainsRune(insert, '\'') {
		quoteChar = '"'
	}
	escaped := escapeCompletionText(insert, quoteChar)
	formatted := string(quoteChar) + escaped
	if kind == candidateDirectory || keepEditing {
		return formatted
	}
	return formatted + string(quoteChar) + " "
}

func needsQuotedCompletion(value string) bool {
	return strings.ContainsAny(value, " \t&|><;\"'")
}

func currentQuoteChar(rawWord string) (rune, bool) {
	if rawWord == "" {
		return 0, false
	}
	quote := rune(rawWord[0])
	if quote != '\'' && quote != '"' {
		return 0, false
	}
	count := 0
	escaped := false
	for _, ch := range rawWord {
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == quote {
			count++
		}
	}
	return quote, count%2 == 1
}

func escapeCompletionText(value string, quoteChar rune) string {
	if quoteChar == '\'' {
		return strings.ReplaceAll(value, "\\", "\\\\")
	}
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, string(quoteChar), "\\"+string(quoteChar))
	return value
}

func longestCommonPrefix(values []string) string {
	if len(values) == 0 {
		return ""
	}
	prefix := values[0]
	for _, value := range values[1:] {
		for !strings.HasPrefix(value, prefix) {
			if prefix == "" {
				return ""
			}
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}

func lineToFields(line string, enclosure string, delimiter string) []string {
	fields, _ := splitTextFields(line, len(line), enclosure, delimiter)
	return fields
}

func splitBufferFields(buffer *readline.Buffer, enclosure string, delimiter string) ([]string, int) {
	fields := []string{}
	lastWordStart := buffer.Cursor
	i := 0
	for i < buffer.Cursor {
		for i < buffer.Cursor && buffer.Buffer[i].String() == " " {
			i++
			lastWordStart = i
		}
		if i >= buffer.Cursor {
			fields = append(fields, "")
			break
		}
		start := i
		quoteMask := 0
		for i < buffer.Cursor {
			cell := buffer.Buffer[i].String()
			if isQuotedCell(cell, enclosure, &quoteMask) {
				i++
				continue
			}
			if quoteMask == 0 && cell == " " {
				fields = append(fields, removeQuotes(buffer.SubString(start, i), enclosure))
				lastWordStart = i + 1
				i++
				break
			}
			if quoteMask == 0 && strings.Contains(delimiter, cell) {
				if start != i {
					fields = append(fields, removeQuotes(buffer.SubString(start, i), enclosure))
				}
				operator := cell
				if cell == "&" && i+1 < buffer.Cursor && buffer.Buffer[i+1].String() == "&" {
					operator = "&&"
					i++
				} else if cell == ">" && i+1 < buffer.Cursor && buffer.Buffer[i+1].String() == ">" {
					operator = ">>"
					i++
				}
				fields = append(fields, operator)
				lastWordStart = i + 1
				i++
				break
			}
			i++
		}
		if i >= buffer.Cursor && start < buffer.Cursor {
			fields = append(fields, removeQuotes(buffer.SubString(start, i), enclosure))
			lastWordStart = start
		}
	}
	return fields, lastWordStart
}

func splitTextFields(text string, cursor int, enclosure string, delimiter string) ([]string, int) {
	fields := []string{}
	lastWordStart := cursor
	i := 0
	for i < cursor {
		for i < cursor && text[i] == ' ' {
			i++
			lastWordStart = i
		}
		if i >= cursor {
			fields = append(fields, "")
			break
		}
		start := i
		quoteMask := 0
		for i < cursor {
			cell := string(text[i])
			if isQuotedCell(cell, enclosure, &quoteMask) {
				i++
				continue
			}
			if quoteMask == 0 && text[i] == ' ' {
				fields = append(fields, removeQuotes(text[start:i], enclosure))
				lastWordStart = i + 1
				i++
				break
			}
			if quoteMask == 0 && strings.Contains(delimiter, cell) {
				if start != i {
					fields = append(fields, removeQuotes(text[start:i], enclosure))
				}
				operator := cell
				if text[i] == '&' && i+1 < cursor && text[i+1] == '&' {
					operator = "&&"
					i++
				} else if text[i] == '>' && i+1 < cursor && text[i+1] == '>' {
					operator = ">>"
					i++
				}
				fields = append(fields, operator)
				lastWordStart = i + 1
				i++
				break
			}
			i++
		}
		if i >= cursor && start < cursor {
			fields = append(fields, removeQuotes(text[start:i], enclosure))
			lastWordStart = start
		}
	}
	return fields, lastWordStart
}

func isQuotedCell(cell string, enclosure string, mask *int) bool {
	for idx, quote := range enclosure {
		if cell != string(quote) {
			continue
		}
		*mask ^= 1 << idx
		return true
	}
	return false
}

func removeQuotes(text string, enclosure string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(enclosure, r) {
			return -1
		}
		return r
	}, text)
}
