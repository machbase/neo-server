package shell

import (
	"fmt"
	"regexp"
	"strings"
)

type QuoteKind int

const (
	QuoteNone QuoteKind = iota
	QuoteSingle
	QuoteDouble
)

type Word struct {
	Fragments []WordFragment
	Explicit  bool
}

type WordFragment struct {
	Text      string
	QuoteKind QuoteKind
}

func (w Word) IsEmpty() bool {
	return w.String() == ""
}

func (w Word) String() string {
	if len(w.Fragments) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, fragment := range w.Fragments {
		builder.WriteString(fragment.Text)
	}
	return builder.String()
}

func (w Word) HasQuotedFragment() bool {
	for _, fragment := range w.Fragments {
		if fragment.QuoteKind != QuoteNone {
			return true
		}
	}
	return false
}

func (w Word) HasUnquotedFragment() bool {
	for _, fragment := range w.Fragments {
		if fragment.QuoteKind == QuoteNone {
			return true
		}
	}
	return false
}

// Command represents a complete parsed shell command line that may contain
// multiple statements connected by operators like ; or &&.
//
// Example: "echo hello; ls -la && cat file.txt" parses into:
//   - Statement 1: "echo hello" with operator ";"
//   - Statement 2: "ls -la" with operator "&&"
//   - Statement 3: "cat file.txt"
type Command struct {
	Raw        string       // Original unparsed command string
	Statements []*Statement // List of statements separated by ; or &&
}

// Statement represents a single command statement that may contain multiple
// commands connected by pipes. Statements are separated by ; or && operators.
//
// Example: "cat file.txt | grep test | wc -l" is a single statement with three
// pipelines connected by pipe operators.
type Statement struct {
	Pipelines []*Pipeline // Commands connected by pipes (|)
	Operator  string      // Operator connecting to next statement: ";" or "&&", empty for last statement
}

// Assignment represents a NAME=VALUE prefix before a command.
// The ValueWord preserves the parsed fragments for proper expansion later.
type Assignment struct {
	Name      string
	ValueWord Word
	Value     string // populated after expansion
}

// Pipeline represents a single command in a pipeline chain with its arguments
// and optional I/O redirections.
//
// Example: "grep test < input.txt > output.txt" has:
//   - Command: "grep"
//   - Args: ["test"]
//   - Stdin: redirection from "input.txt"
//   - Stdout: redirection to "output.txt"
type Pipeline struct {
	Assignments []Assignment // NAME=VALUE prefixes before the command
	ParseError  string       // parser-detected shell error for this pipeline
	CommandWord *Word        // Parsed command word before expansion
	ArgWords    []Word       // Parsed argument words before expansion
	Command     string       // The command name/path to execute
	Args        []string     // Command-line arguments
	Stdin       *Redirect    // Input redirection (<), nil if not specified
	Stdout      *Redirect    // Output redirection (> or >>), nil if not specified
	Stderr      *Redirect    // Error output redirection (currently unused, reserved for future use)
}

// Redirect represents an I/O redirection operation, specifying the type
// of redirection and the target file path.
//
// Supported redirection types:
//   - "<"  : Input redirection (read from file)
//   - ">"  : Output redirection (write to file, overwrite)
//   - ">>" : Output redirection (append to file)
type Redirect struct {
	Type       string // Redirection operator: "<", ">", or ">>"
	Target     string // Target file path or descriptor
	TargetWord *Word  // Parsed redirect target before expansion
}

// parseCommand parses a complete command string into a structured Command object.
// It handles complex shell syntax including multiple statements, pipes, and redirections
// while properly respecting quoted strings.
//
// Parsing hierarchy:
//  1. Splits by statement operators (; or &&)
//  2. For each statement, splits by pipe operators (|)
//  3. For each pipeline, parses command, arguments, and redirections
//
// Example: "cat file.txt | grep test > out.txt && echo done"
//   - Statement 1: "cat file.txt | grep test > out.txt" (operator: &&)
//   - Pipeline 1: "cat file.txt"
//   - Pipeline 2: "grep test > out.txt" (with stdout redirection)
//   - Statement 2: "echo done"
//
// Returns an empty Command structure if input is empty or contains only whitespace.
func parseCommand(input string) *Command {
	cmd := &Command{
		Raw:        input,
		Statements: []*Statement{},
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return cmd
	}

	// Split by statement operators (;, &&) while respecting quotes
	statements := splitStatements(input)

	for i, stmtStr := range statements {
		stmt := &Statement{
			Pipelines: []*Pipeline{},
		}

		// Determine operator to next statement
		// Note: This is a simplified approach; for production use, the operator
		// should be tracked during the split phase for accuracy
		if i < len(statements)-1 {
			// Check what operator was used
			if strings.Contains(input, "&&") {
				stmt.Operator = "&&"
			} else {
				stmt.Operator = ";"
			}
		}

		// Split by pipes while respecting quotes
		pipelineStrings := splitPipes(stmtStr)

		for _, pipeStr := range pipelineStrings {
			pipeline := parsePipeline(pipeStr)
			stmt.Pipelines = append(stmt.Pipelines, pipeline)
		}

		cmd.Statements = append(cmd.Statements, stmt)
	}

	return cmd
}

// splitStatements splits the input string into individual statements separated by
// semicolon (;) or logical AND (&&) operators, while properly handling quoted strings.
//
// Quoted strings (single or double quotes) are preserved and their contents are not
// split, even if they contain semicolons or && sequences. Backslash-escaped quotes
// are not treated as quote delimiters.
//
// Examples:
//   - "cmd1; cmd2" → ["cmd1", "cmd2"]
//   - "cmd1 && cmd2 && cmd3" → ["cmd1", "cmd2", "cmd3"]
//   - `echo "a;b"; echo c` → [`echo "a;b"`, "echo c"]
//   - `echo "a&&b" && echo c` → [`echo "a&&b"`, "echo c"]
//
// Returns a slice of trimmed statement strings. Empty statements are not included.
func splitStatements(input string) []string {
	var result []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)
	var prevCh rune

	runes := []rune(input)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		// Track quote state to avoid splitting within quoted strings
		// Quotes are considered delimiters only if not escaped with backslash
		if (ch == '"' || ch == '\'') && prevCh != '\\' {
			if !inQuote {
				inQuote = true
				quoteChar = ch
			} else if ch == quoteChar {
				inQuote = false
				quoteChar = 0
			}
			current.WriteRune(ch)
			prevCh = ch
			continue
		}

		// Process operators only when outside quoted strings
		if !inQuote {
			// Check for logical AND operator (&&)
			if ch == '&' && i+1 < len(runes) && runes[i+1] == '&' {
				if current.Len() > 0 {
					result = append(result, strings.TrimSpace(current.String()))
					current.Reset()
				}
				i++ // Skip next & character
				prevCh = ch
				continue
			}

			// Check for statement separator (;)
			if ch == ';' {
				if current.Len() > 0 {
					result = append(result, strings.TrimSpace(current.String()))
					current.Reset()
				}
				prevCh = ch
				continue
			}
		}

		current.WriteRune(ch)
		prevCh = ch
	}

	// Append any remaining content as the last statement
	if current.Len() > 0 {
		result = append(result, strings.TrimSpace(current.String()))
	}

	return result
}

// splitPipes splits a statement string into individual pipeline commands separated
// by pipe (|) operators, while properly handling quoted strings.
//
// The pipe operator connects the stdout of one command to the stdin of the next.
// Pipe characters within quoted strings are not treated as operators.
//
// Examples:
//   - "cat file.txt | grep test" → ["cat file.txt", "grep test"]
//   - "cmd1 | cmd2 | cmd3" → ["cmd1", "cmd2", "cmd3"]
//   - `echo "a|b" | cat` → [`echo "a|b"`, "cat"]
//
// Returns a slice of trimmed pipeline command strings. Empty pipelines are not included.
func splitPipes(input string) []string {
	var result []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)
	var prevCh rune

	runes := []rune(input)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		// Track quote state to preserve pipe characters within quotes
		if (ch == '"' || ch == '\'') && prevCh != '\\' {
			if !inQuote {
				inQuote = true
				quoteChar = ch
			} else if ch == quoteChar {
				inQuote = false
				quoteChar = 0
			}
			current.WriteRune(ch)
			prevCh = ch
			continue
		}

		// Process pipe operator only when outside quoted strings
		if !inQuote && ch == '|' {
			if current.Len() > 0 {
				result = append(result, strings.TrimSpace(current.String()))
				current.Reset()
			}
			prevCh = ch
			continue
		}

		current.WriteRune(ch)
		prevCh = ch
	}

	// Append any remaining content as the last pipeline command
	if current.Len() > 0 {
		result = append(result, strings.TrimSpace(current.String()))
	}

	return result
}

// validAssignmentName matches valid shell variable names: ^[A-Za-z_][A-Za-z0-9_]*$
var validAssignmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// parseAssignmentWord attempts to parse a Word as a NAME=VALUE assignment.
// It looks for the first unquoted '=' in the word fragments to split name and value.
// Returns (assignment, true, nil) if valid, (zero, false, nil) if not an assignment,
// or (zero, false, error) if it looks like an assignment but has an invalid name.
func parseAssignmentWord(word Word) (Assignment, bool, error) {
	// Find the position within fragments where an unquoted '=' occurs.
	// We scan fragment by fragment; only QuoteNone fragments can contain the delimiter.
	var nameParts []string
	foundEq := false
	eqFragIdx := -1
	eqByteIdx := -1

	for fi, frag := range word.Fragments {
		if frag.QuoteKind != QuoteNone {
			// A quoted fragment before the first unquoted '=' means the token is in
			// assignment position but the variable name is not a valid bare name.
			// Example: 'FOO'=x
			if !foundEq {
				var nameBuilder strings.Builder
				for _, prefixFrag := range word.Fragments[:fi+1] {
					nameBuilder.WriteString(prefixFrag.Text)
				}
				return Assignment{}, false, fmt.Errorf("invalid variable name: %s", nameBuilder.String())
			}
			break
		}
		// Unquoted fragment: look for '='
		idx := strings.IndexByte(frag.Text, '=')
		if idx >= 0 {
			nameParts = append(nameParts, frag.Text[:idx])
			eqFragIdx = fi
			eqByteIdx = idx
			foundEq = true
			break
		}
		nameParts = append(nameParts, frag.Text)
	}

	if !foundEq {
		return Assignment{}, false, nil
	}

	// Reconstruct name and validate
	name := strings.Join(nameParts, "")
	if !validAssignmentName.MatchString(name) {
		return Assignment{}, false, fmt.Errorf("invalid variable name: %s", name)
	}

	// Build ValueWord from the rest: the part after '=' in eqFragIdx, plus subsequent fragments.
	var valueFragments []WordFragment
	// Tail of the fragment where '=' was found (after the '=')
	tailText := word.Fragments[eqFragIdx].Text[eqByteIdx+1:]
	if tailText != "" {
		valueFragments = append(valueFragments, WordFragment{Text: tailText, QuoteKind: QuoteNone})
	}
	for _, frag := range word.Fragments[eqFragIdx+1:] {
		valueFragments = append(valueFragments, frag)
	}

	valueWord := Word{Fragments: valueFragments, Explicit: true}
	return Assignment{Name: name, ValueWord: valueWord}, true, nil
}

// parsePipeline parses a single pipeline command string, extracting the command name,
// arguments, and any I/O redirection operators.
//
// The parser identifies redirection operators (<, >, >>, 2>, 2>>) and their target files,
// separating them from the command and its arguments. The first non-redirection token
// is treated as the command, and subsequent tokens as arguments.
//
// Examples:
//   - "ls -la /tmp" → Command: "ls", Args: ["-la", "/tmp"]
//   - "cat < input.txt" → Command: "cat", Stdin: "input.txt"
//   - "sort data.txt > output.txt" → Command: "sort", Args: ["data.txt"], Stdout: "output.txt"
//   - "grep test >> log.txt" → Command: "grep", Args: ["test"], Stdout: "log.txt" (append)
//
// Returns a Pipeline structure. If input is empty, returns a Pipeline with empty command.
func parsePipeline(input string) *Pipeline {
	pipeline := &Pipeline{
		Assignments: []Assignment{},
		ArgWords:    []Word{},
		Args:        []string{},
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return pipeline
	}

	// Tokenize the input, which separates operators and handles quoted strings
	tokens := tokenize(input)

	var cmdTokens []Word
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]

		if isRedirectOperator(token) && token.String() == "2>&1" {
			pipeline.Stderr = &Redirect{Type: token.String(), Target: "1"}
			continue
		}

		// Check for redirection operators and extract their targets
		if isRedirectOperator(token) && (token.String() == "<" || token.String() == ">" || token.String() == ">>" || token.String() == "2>" || token.String() == "2>>") {
			if i+1 < len(tokens) {
				target := tokens[i+1]
				redirect := &Redirect{
					Type:       token.String(),
					Target:     target.String(),
					TargetWord: cloneWordPtr(target),
				}

				switch token.String() {
				case "<":
					pipeline.Stdin = redirect
				case ">", ">>":
					pipeline.Stdout = redirect
				case "2>", "2>>":
					pipeline.Stderr = redirect
				}

				i++ // Skip the target token (already consumed)
				continue
			}
		}

		// Collect non-redirection tokens as command and arguments
		cmdTokens = append(cmdTokens, token)
	}

	// Extract assignment prefix tokens from cmdTokens.
	// Leading tokens that look like NAME=VALUE are collected as Assignments.
	// Scanning stops at the first non-assignment token (which becomes Command).
	nonAssignIdx := 0
	for nonAssignIdx < len(cmdTokens) {
		token := cmdTokens[nonAssignIdx]
		assignment, isAssignment, err := parseAssignmentWord(token)
		if err != nil {
			pipeline.ParseError = err.Error()
			return pipeline
		}
		if !isAssignment {
			break
		}
		pipeline.Assignments = append(pipeline.Assignments, assignment)
		nonAssignIdx++
	}

	remainingTokens := cmdTokens[nonAssignIdx:]

	// First remaining token is the command name, remaining tokens are arguments
	if len(remainingTokens) > 0 {
		pipeline.CommandWord = cloneWordPtr(remainingTokens[0])
		pipeline.Command = remainingTokens[0].String()
		if len(remainingTokens) > 1 {
			pipeline.ArgWords = cloneWords(remainingTokens[1:])
			pipeline.Args = wordsText(remainingTokens[1:])
		}
	}

	return pipeline
}

// tokenize splits an input string into individual tokens, handling quoted strings
// and redirection operators as special cases.
//
// Tokenization rules:
//   - Whitespace (space, tab) separates tokens, unless within quotes
//   - Quoted strings (single or double quotes) are treated as single tokens
//     with the quote characters removed from the output
//   - Redirection operators (<, >, >>, 2>, 2>>, 2>&1) are extracted as separate tokens
//   - Multiple consecutive whitespace characters are treated as a single separator
//
// Examples:
//   - "ls -la /tmp" → ["ls", "-la", "/tmp"]
//   - `echo "hello world"` → ["echo", "hello world"]
//   - "cat < input.txt" → ["cat", "<", "input.txt"]
//   - "echo test >> file.txt" → ["echo", "test", ">>", "file.txt"]
//   - "cmd   arg1    arg2" → ["cmd", "arg1", "arg2"]
//
// Returns a slice of token strings. Quote characters are not included in the tokens.
func tokenize(input string) []Word {
	var tokens []Word
	var currentFragment strings.Builder
	var currentFragments []WordFragment
	inQuote := false
	quoteChar := rune(0)
	currentQuote := QuoteNone
	inToken := false
	var prevCh rune

	flushFragment := func(force bool) {
		if currentFragment.Len() == 0 && !force {
			return
		}
		currentFragments = append(currentFragments, WordFragment{
			Text:      currentFragment.String(),
			QuoteKind: currentQuote,
		})
		currentFragment.Reset()
	}

	flushToken := func() {
		if !inToken {
			return
		}
		flushFragment(false)
		tokens = append(tokens, Word{
			Fragments: cloneFragments(currentFragments),
			Explicit:  true,
		})
		currentFragments = nil
		currentQuote = QuoteNone
		inToken = false
	}

	runes := []rune(input)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		// Track quote boundaries and exclude quote characters from the token
		// Quotes must not be escaped with backslash to be treated as delimiters
		// Nested quotes (different quote type inside a quoted string) are preserved
		if (ch == '"' || ch == '\'') && prevCh != '\\' {
			if !inQuote {
				// Starting a quote - exclude this character
				flushFragment(false)
				inQuote = true
				quoteChar = ch
				inToken = true
				if ch == '\'' {
					currentQuote = QuoteSingle
				} else {
					currentQuote = QuoteDouble
				}
				prevCh = ch
				continue
			} else if ch == quoteChar {
				// Ending the matching quote - exclude this character
				flushFragment(true)
				inQuote = false
				quoteChar = 0
				currentQuote = QuoteNone
				prevCh = ch
				continue
			}
			// else: nested quote (different from quoteChar) - include it in the token
		}

		// Whitespace acts as token separator only outside quoted strings
		if !inQuote && (ch == ' ' || ch == '\t') {
			flushToken()
			prevCh = ch
			continue
		}

		// Extract redirection operators as separate tokens when outside quotes
		if !inQuote {
			if ch == '2' && i+3 < len(runes) && runes[i+1] == '>' && runes[i+2] == '&' && runes[i+3] == '1' {
				flushToken()
				tokens = append(tokens, newWord("2>&1"))
				i += 3
				prevCh = '1'
				continue
			}

			if ch == '2' && i+1 < len(runes) && runes[i+1] == '>' {
				flushToken()
				if i+2 < len(runes) && runes[i+2] == '>' {
					tokens = append(tokens, newWord("2>>"))
					i += 2
				} else {
					tokens = append(tokens, newWord("2>"))
					i++
				}
				prevCh = '>'
				continue
			}

			// Check for append redirection operator (>>)
			if ch == '>' && i+1 < len(runes) && runes[i+1] == '>' {
				flushToken()
				tokens = append(tokens, newWord(">>"))
				i++ // Skip the next > character
				prevCh = ch
				continue
			}

			// Check for single-character redirection operators: < or >
			if ch == '<' || ch == '>' {
				flushToken()
				tokens = append(tokens, newWord(string(ch)))
				prevCh = ch
				continue
			}
		}

		inToken = true
		currentFragment.WriteRune(ch)
		prevCh = ch
	}

	// Append any remaining content as the last token
	flushToken()

	return tokens
}

func wordsText(words []Word) []string {
	result := make([]string, len(words))
	for i, word := range words {
		result[i] = word.String()
	}
	return result
}

func cloneWords(words []Word) []Word {
	if len(words) == 0 {
		return nil
	}
	result := make([]Word, len(words))
	for i, word := range words {
		result[i] = cloneWord(word)
	}
	return result
}

func cloneWordPtr(word Word) *Word {
	copyWord := cloneWord(word)
	return &copyWord
}

func isRedirectOperator(word Word) bool {
	if word.HasQuotedFragment() {
		return false
	}
	switch word.String() {
	case "<", ">", ">>", "2>", "2>>", "2>&1":
		return true
	default:
		return false
	}
}

func cloneWord(word Word) Word {
	return Word{
		Fragments: cloneFragments(word.Fragments),
		Explicit:  word.Explicit,
	}
}

func cloneFragments(fragments []WordFragment) []WordFragment {
	if len(fragments) == 0 {
		return nil
	}
	result := make([]WordFragment, len(fragments))
	copy(result, fragments)
	return result
}

func newWord(text string) Word {
	return Word{
		Fragments: []WordFragment{
			{Text: text, QuoteKind: QuoteNone},
		},
		Explicit: true,
	}
}
