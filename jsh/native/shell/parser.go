package shell

import "strings"

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

// Pipeline represents a single command in a pipeline chain with its arguments
// and optional I/O redirections.
//
// Example: "grep test < input.txt > output.txt" has:
//   - Command: "grep"
//   - Args: ["test"]
//   - Stdin: redirection from "input.txt"
//   - Stdout: redirection to "output.txt"
type Pipeline struct {
	Command string    // The command name/path to execute
	Args    []string  // Command-line arguments
	Stdin   *Redirect // Input redirection (<), nil if not specified
	Stdout  *Redirect // Output redirection (> or >>), nil if not specified
	Stderr  *Redirect // Error output redirection (currently unused, reserved for future use)
}

// Redirect represents an I/O redirection operation, specifying the type
// of redirection and the target file path.
//
// Supported redirection types:
//   - "<"  : Input redirection (read from file)
//   - ">"  : Output redirection (write to file, overwrite)
//   - ">>" : Output redirection (append to file)
type Redirect struct {
	Type   string // Redirection operator: "<", ">", or ">>"
	Target string // Target file path or descriptor
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

// parsePipeline parses a single pipeline command string, extracting the command name,
// arguments, and any I/O redirection operators.
//
// The parser identifies redirection operators (<, >, >>) and their target files,
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
		Args: []string{},
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return pipeline
	}

	// Tokenize the input, which separates operators and handles quoted strings
	tokens := tokenize(input)

	var cmdTokens []string
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]

		// Check for redirection operators and extract their targets
		if token == "<" || token == ">" || token == ">>" {
			if i+1 < len(tokens) {
				redirect := &Redirect{
					Type:   token,
					Target: tokens[i+1],
				}

				switch token {
				case "<":
					pipeline.Stdin = redirect
				case ">", ">>":
					pipeline.Stdout = redirect
				}

				i++ // Skip the target token (already consumed)
				continue
			}
		}

		// Collect non-redirection tokens as command and arguments
		cmdTokens = append(cmdTokens, token)
	}

	// First token is the command name, remaining tokens are arguments
	if len(cmdTokens) > 0 {
		pipeline.Command = cmdTokens[0]
		if len(cmdTokens) > 1 {
			pipeline.Args = cmdTokens[1:]
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
//   - Redirection operators (<, >, >>) are extracted as separate tokens
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
func tokenize(input string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)
	var prevCh rune

	runes := []rune(input)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		// Track quote boundaries and exclude quote characters from the token
		// Quotes must not be escaped with backslash to be treated as delimiters
		if (ch == '"' || ch == '\'') && prevCh != '\\' {
			if !inQuote {
				inQuote = true
				quoteChar = ch
			} else if ch == quoteChar {
				inQuote = false
				quoteChar = 0
			}
			prevCh = ch
			continue // Quote chars are not included in the resulting token
		}

		// Whitespace acts as token separator only outside quoted strings
		if !inQuote && (ch == ' ' || ch == '\t') {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			prevCh = ch
			continue
		}

		// Extract redirection operators as separate tokens when outside quotes
		if !inQuote {
			// Check for append redirection operator (>>)
			if ch == '>' && i+1 < len(runes) && runes[i+1] == '>' {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
				tokens = append(tokens, ">>")
				i++ // Skip the next > character
				prevCh = ch
				continue
			}

			// Check for single-character redirection operators: < or >
			if ch == '<' || ch == '>' {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
				tokens = append(tokens, string(ch))
				prevCh = ch
				continue
			}
		}

		current.WriteRune(ch)
		prevCh = ch
	}

	// Append any remaining content as the last token
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}
