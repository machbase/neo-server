package shell

import (
	"fmt"
	"io/fs"
	"path"
	"strings"
)

type expandedPart struct {
	Text          string
	GlobProtected bool
}

func (sh *Shell) expandPipeline(pipe *Pipeline) (*Pipeline, error) {
	if sh.env == nil {
		return nil, fmt.Errorf("shell environment is not initialized")
	}

	expanded := &Pipeline{
		CommandWord: clonePipelineWord(pipe.CommandWord),
		ArgWords:    cloneWords(pipe.ArgWords),
		Stdin:       cloneRedirect(pipe.Stdin),
		Stdout:      cloneRedirect(pipe.Stdout),
		Stderr:      cloneRedirect(pipe.Stderr),
	}

	command, err := sh.expandCommandWord(pipe.CommandWord)
	if err != nil {
		return nil, err
	}

	args, err := sh.expandWords(pipe.ArgWords)
	if err != nil {
		return nil, err
	}
	command, args = sh.expandCommandAlias(command, args)
	expanded.Command = command
	expanded.Args = args

	if expanded.Stdin != nil {
		target, err := sh.expandRedirectTarget(expanded.Stdin)
		if err != nil {
			return nil, err
		}
		expanded.Stdin.Target = target
	}
	if expanded.Stdout != nil {
		target, err := sh.expandRedirectTarget(expanded.Stdout)
		if err != nil {
			return nil, err
		}
		expanded.Stdout.Target = target
	}
	if expanded.Stderr != nil && expanded.Stderr.Type != "2>&1" {
		target, err := sh.expandRedirectTarget(expanded.Stderr)
		if err != nil {
			return nil, err
		}
		expanded.Stderr.Target = target
	}

	return expanded, nil
}

func (sh *Shell) expandCommandAlias(command string, args []string) (string, []string) {
	resolvedArgs := append([]string{}, args...)
	if sh.env == nil || command == "" {
		return command, resolvedArgs
	}

	alias := sh.env.Alias(command)
	if !hasAliasExpansion(command, alias) {
		return command, resolvedArgs
	}

	resolvedCommand := alias[0]
	if len(alias) > 1 {
		resolvedArgs = append(append([]string{}, alias[1:]...), resolvedArgs...)
	}
	return resolvedCommand, resolvedArgs
}

func hasAliasExpansion(command string, alias []string) bool {
	if len(alias) == 0 {
		return false
	}
	return len(alias) != 1 || alias[0] != command
}

func (sh *Shell) expandCommandWord(word *Word) (string, error) {
	if word == nil {
		return "", nil
	}
	expanded, err := sh.expandWord(*word)
	if err != nil {
		return "", err
	}
	if len(expanded) != 1 {
		return "", fmt.Errorf("command expansion produced %d words", len(expanded))
	}
	return expanded[0], nil
}

func (sh *Shell) expandWords(words []Word) ([]string, error) {
	if len(words) == 0 {
		return nil, nil
	}
	result := make([]string, 0, len(words))
	for _, word := range words {
		expanded, err := sh.expandWord(word)
		if err != nil {
			return nil, err
		}
		result = append(result, expanded...)
	}
	return result, nil
}

func (sh *Shell) expandRedirectTarget(redir *Redirect) (string, error) {
	if redir == nil || redir.TargetWord == nil {
		return redir.Target, nil
	}
	expanded, err := sh.expandWord(*redir.TargetWord)
	if err != nil {
		return "", err
	}
	if len(expanded) != 1 {
		return "", fmt.Errorf("ambiguous redirect target: %s", redir.TargetWord.String())
	}
	return expanded[0], nil
}

func (sh *Shell) expandWord(word Word) ([]string, error) {
	parts := make([]expandedPart, 0, len(word.Fragments))
	for _, fragment := range word.Fragments {
		part := expandedPart{
			Text:          fragment.Text,
			GlobProtected: fragment.QuoteKind != QuoteNone,
		}
		switch fragment.QuoteKind {
		case QuoteSingle:
		case QuoteDouble, QuoteNone:
			part.Text = sh.env.Expand(fragment.Text)
		}
		parts = append(parts, part)
	}

	assembled := joinExpandedParts(parts)
	if !hasUnprotectedWildcard(parts) {
		return []string{assembled}, nil
	}
	return sh.expandGlob(assembled)
}

func (sh *Shell) expandGlob(pattern string) ([]string, error) {
	if !hasWildcard(pattern) {
		return []string{pattern}, nil
	}

	dirPart, basePart := splitGlobPattern(pattern)
	if dirPart != "" && hasWildcard(dirPart) {
		return []string{pattern}, nil
	}

	dirForRead := dirPart
	if dirForRead == "" {
		dirForRead = "."
	}

	readDirFS, ok := sh.env.Filesystem().(fs.ReadDirFS)
	if !ok {
		return nil, fmt.Errorf("shell filesystem does not support glob expansion")
	}
	entries, err := readDirFS.ReadDir(sh.env.ResolveAbsPath(dirForRead))
	if err != nil {
		return []string{pattern}, nil
	}

	matches := make([]string, 0)
	matchDot := strings.HasPrefix(basePart, ".")
	for _, entry := range entries {
		name := entry.Name()
		if !matchDot && strings.HasPrefix(name, ".") {
			continue
		}
		ok, matchErr := path.Match(basePart, name)
		if matchErr != nil || !ok {
			continue
		}
		matches = append(matches, joinGlobPath(dirPart, name))
	}
	if len(matches) == 0 {
		return []string{pattern}, nil
	}
	return matches, nil
}

func hasWildcard(value string) bool {
	return strings.ContainsAny(value, "*?")
}

func splitGlobPattern(pattern string) (string, string) {
	idx := strings.LastIndex(pattern, "/")
	if idx == -1 {
		return "", pattern
	}
	dirPart := pattern[:idx]
	if dirPart == "" {
		dirPart = "/"
	}
	return dirPart, pattern[idx+1:]
}

func joinGlobPath(dir string, name string) string {
	switch dir {
	case "", ".":
		return name
	case "/":
		return "/" + name
	default:
		return dir + "/" + name
	}
}

func clonePipelineWord(word *Word) *Word {
	if word == nil {
		return nil
	}
	copyWord := cloneWord(*word)
	return &copyWord
}

func cloneRedirect(redir *Redirect) *Redirect {
	if redir == nil {
		return nil
	}
	copyRedirect := *redir
	copyRedirect.TargetWord = clonePipelineWord(redir.TargetWord)
	return &copyRedirect
}

func joinExpandedParts(parts []expandedPart) string {
	if len(parts) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, part := range parts {
		builder.WriteString(part.Text)
	}
	return builder.String()
}

func hasUnprotectedWildcard(parts []expandedPart) bool {
	for _, part := range parts {
		if part.GlobProtected {
			continue
		}
		if hasWildcard(part.Text) {
			return true
		}
	}
	return false
}
