package expression

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const autoConvertFirstUpperCaseOfAccessor = false

func ParseTokens(input string, functions map[string]Function) ([]Token, error) {
	var ret []Token
	var token Token
	var found bool
	var err error

	stream := newLexerStream(input)
	state := validLexerStates[0]
	for stream.canRead() {
		token, err, found = readToken(stream, state, functions)
		if err != nil {
			return ret, err
		}
		if !found {
			break
		}
		state, err = getLexerStateForToken(token.Kind)
		if err != nil {
			return ret, err
		}
		ret = append(ret, token)
	}
	err = checkBalance(ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

var ParseStringToTime = false

func readToken(stream *lexerStream, state lexerState, functions map[string]Function) (Token, error, bool) {
	var function Function
	var ret Token
	var tokenValue interface{}
	var tokenTime time.Time
	var tokenString string
	var kind TokenKind
	var character rune
	var found bool
	var completed bool
	var err error

	// numeric is 0-9, or . or 0x followed by digits
	// string starts with '
	// variable is alphanumeric, always starts with a letter
	// bracket always means variable
	// symbols are anything non-alphanumeric
	// all others read into a buffer until they reach the end of the stream
	for stream.canRead() {

		character = stream.readCharacter()

		if unicode.IsSpace(character) {
			continue
		}

		kind = UNKNOWN

		// numeric constant
		if isNumeric(character) {
			if stream.canRead() && character == '0' {
				character = stream.readCharacter()

				if stream.canRead() && character == 'x' {
					tokenString, _ = readUntilFalse(stream, false, true, true, isHexDigit)
					tokenValueInt, err := strconv.ParseUint(tokenString, 16, 64)

					if err != nil {
						return Token{}, fmt.Errorf("unable to parse hex value '%v' to uint64", tokenString), false
					}

					kind = NUMERIC
					tokenValue = float64(tokenValueInt)
					break
				} else {
					stream.rewind(1)
				}
			}

			tokenString = readTokenUntilFalse(stream, isNumeric)
			tokenValue, err = strconv.ParseFloat(tokenString, 64)

			if err != nil {
				return Token{}, fmt.Errorf("unable to parse numeric value '%v' to float64", tokenString), false
			}
			kind = NUMERIC
			break
		}

		// comma, separator
		if character == ',' {
			tokenValue = ","
			kind = SEPARATOR
			break
		}

		// escaped variable
		if character == '[' {
			tokenValue, completed = readUntilFalse(stream, true, false, true, isNotClosingBracket)
			kind = VARIABLE

			if !completed {
				return Token{}, errors.New("unclosed parameter bracket"), false
			}

			// above method normally rewinds us to the closing bracket, which we want to skip.
			stream.rewind(-1)
			break
		}

		if character == '$' {
			if nc := stream.readCharacter(); unicode.IsLetter(nc) {
				tokenString = readTokenUntilFalse(stream, isVariableName)
				tokenValue = "$" + tokenString
				kind = VARIABLE
				break
			} else {
				stream.rewind(1)
			}
		}

		if character == '/' {
			if nc := stream.readCharacter(); nc == '/' {
				readStringUntilEndOfLine(stream)
				continue
			} else {
				stream.rewind(1)
			}
		}
		if character == '`' {
			kind = STRING
			tokenValue = readStringUntilBacktick(stream)
			break
		}

		if character == '{' {
			blockValue := readBlock(stream)
			tokenValue = blockValue
			kind = STRING // BLOCK
			break
		}

		// regular variable - or function?
		if unicode.IsLetter(character) {
			tokenString = readTokenUntilFalse(stream, isVariableName)
			tokenValue = tokenString
			kind = VARIABLE

			// boolean?
			if tokenValue == "true" {
				kind = BOOLEAN
				tokenValue = true
			} else {
				if tokenValue == "false" {
					kind = BOOLEAN
					tokenValue = false
				}
			}

			// textual operator?
			if tokenValue == "in" || tokenValue == "IN" {
				// force lower case for consistency
				tokenValue = "in"
				kind = COMPARATOR
			}

			// function?
			function, found = functions[tokenString]
			if found {
				kind = FUNCTION
				tokenValue = function
			}

			// accessor?
			accessorIndex := strings.Index(tokenString, ".")
			if accessorIndex > 0 {
				// check that it doesn't end with a hanging period
				if tokenString[len(tokenString)-1] == '.' {
					return Token{}, fmt.Errorf("hanging accessor on token '%s'", tokenString), false
				}
				kind = ACCESSOR
				splits := strings.Split(tokenString, ".")
				tokenValue = splits

				// check that none of them are unexported
				for i := 1; i < len(splits); i++ {
					firstCharacter := getFirstRune(splits[i])
					if unicode.ToUpper(firstCharacter) != firstCharacter {
						if autoConvertFirstUpperCaseOfAccessor {
							// instead of raising error, make it upper cased func name.
							// so that script can keep lower cased field and function names
							// for example) obj.field => obj.Field
							raw := []rune(splits[i])
							raw[0] = unicode.ToUpper(firstCharacter)
							splits[i] = string(raw)
						} else {
							return Token{}, fmt.Errorf("unable to access unexported field '%s' in token '%s'", splits[i], tokenString), false
						}
					}
				}
			}
			break
		}

		if !isNotQuote(character) {
			tokenValue, completed = readUntilFalse(stream, true, false, true, isNotQuote)

			if !completed {
				return Token{}, errors.New("unclosed string literal"), false
			}

			// advance the stream one position, since reading until false assumes the terminator is a real token
			stream.rewind(-1)

			// check to see if this can be parsed as a time.
			if ParseStringToTime {
				tokenTime, found = tryParseTime(tokenValue.(string))
				if found {
					kind = TIME
					tokenValue = tokenTime
				} else {
					kind = STRING
				}
			} else {
				kind = STRING
			}
			break
		}

		if character == '(' {
			tokenValue = character
			kind = CLAUSE
			break
		}

		if character == ')' {
			tokenValue = character
			kind = CLAUSE_CLOSE
			break
		}

		// must be a known symbol
		tokenString = readTokenUntilFalse(stream, isNotAlphanumeric)
		tokenValue = tokenString

		// quick hack for the case where "-" can mean "prefixed negation" or "minus", which are used
		// very differently.
		if state.canTransitionTo(PREFIX) {
			_, found = prefixSymbols[tokenString]
			if found {
				kind = PREFIX
				break
			}
		}
		_, found = modifierSymbols[tokenString]
		if found {
			kind = MODIFIER
			break
		}

		_, found = logicalSymbols[tokenString]
		if found {
			kind = LOGICALOP
			break
		}

		_, found = comparatorSymbols[tokenString]
		if found {
			kind = COMPARATOR
			break
		}

		_, found = ternarySymbols[tokenString]
		if found {
			kind = TERNARY
			break
		}

		return ret, fmt.Errorf("invalid token: '%s'", tokenString), false
	}

	ret.Kind = kind
	ret.Value = tokenValue

	return ret, nil, (kind != UNKNOWN)
}

func readStringUntilBacktick(stream *lexerStream) string {
	var tokenBuffer bytes.Buffer
	for stream.canRead() {
		character := stream.readCharacter()
		if character == '`' {
			break
		}
		tokenBuffer.WriteString(string(character))
	}
	return tokenBuffer.String()
}

func readStringUntilEndOfLine(stream *lexerStream) {
	for stream.canRead() {
		character := stream.readCharacter()
		if character == '\n' {
			return
		}
	}
}

func readBlock(stream *lexerStream) string {
	var tokenBuffer bytes.Buffer
	var character rune
	var depth = 1

	for stream.canRead() {
		character = stream.readCharacter()
		switch character {
		case '{':
			depth++
		case '}':
			depth--
		}

		if depth == 0 {
			break
		} else {
			tokenBuffer.WriteString(string(character))
		}
	}
	return tokenBuffer.String()
}

func readTokenUntilFalse(stream *lexerStream, condition func(rune) bool) string {
	stream.rewind(1)
	ret, _ := readUntilFalse(stream, false, true, true, condition)
	return ret
}

// Returns the string that was read until the given [condition] was false, or whitespace was broken.
// Returns false if the stream ended before whitespace was broken or condition was met.
func readUntilFalse(stream *lexerStream, includeWhitespace bool, breakWhitespace bool, allowEscaping bool, condition func(rune) bool) (string, bool) {
	var tokenBuffer bytes.Buffer
	var character rune
	var conditioned bool = false

	for stream.canRead() {
		character = stream.readCharacter()

		// Use backslashes to escape anything
		if allowEscaping && character == '\\' {
			character = stream.readCharacter()
			switch character {
			case 'n':
				tokenBuffer.WriteString("\n")
			case 't':
				tokenBuffer.WriteString("\t")
			default:
				tokenBuffer.WriteString(string(character))
			}
			continue
		}

		if unicode.IsSpace(character) {
			if breakWhitespace && tokenBuffer.Len() > 0 {
				conditioned = true
				break
			}
			if !includeWhitespace {
				continue
			}
		}

		if condition(character) {
			tokenBuffer.WriteString(string(character))
		} else {
			conditioned = true
			stream.rewind(1)
			break
		}
	}

	return tokenBuffer.String(), conditioned
}

// Checks to see if any optimizations can be performed on the given [tokens], which form a complete, valid expression.
// The returns slice will represent the optimized (or unmodified) list of tokens to use.
func optimizeTokens(tokens []Token) ([]Token, error) {
	var token Token
	var symbol OperatorSymbol
	var err error
	var index int

	for index, token = range tokens {
		// if we find a regex operator, and the right-hand value is a constant, precompile and replace with a pattern.
		if token.Kind != COMPARATOR {
			continue
		}

		symbol = comparatorSymbols[token.Value.(string)]
		if symbol != REQ && symbol != NREQ {
			continue
		}

		index++
		token = tokens[index]
		if token.Kind == STRING {
			token.Kind = PATTERN
			token.Value, err = regexp.Compile(token.Value.(string))
			if err != nil {
				return tokens, err
			}
			tokens[index] = token
		}
	}
	return tokens, nil
}

// checks the balance of tokens which have multiple parts, such as parenthesis.
func checkBalance(tokens []Token) error {
	var stream *tokenStream
	var token Token
	var parens int

	stream = newTokenStream(tokens)

	for stream.hasNext() {
		token = stream.next()
		if token.Kind == CLAUSE {
			parens++
			continue
		}
		if token.Kind == CLAUSE_CLOSE {
			parens--
			continue
		}
	}

	if parens != 0 {
		return errors.New("unbalanced parenthesis")
	}
	return nil
}

// func isDigit(character rune) bool {
// 	return unicode.IsDigit(character)
// }

func isHexDigit(character rune) bool {
	character = unicode.ToLower(character)
	return unicode.IsDigit(character) ||
		character == 'a' ||
		character == 'b' ||
		character == 'c' ||
		character == 'd' ||
		character == 'e' ||
		character == 'f'
}

func isNumeric(character rune) bool {
	return unicode.IsDigit(character) || character == '.'
}

func isNotQuote(character rune) bool {
	return character != '\'' && character != '"' && character != '`' && character != '{'
}

func isNotAlphanumeric(character rune) bool {
	return !(unicode.IsDigit(character) ||
		unicode.IsLetter(character) ||
		character == '(' ||
		character == ')' ||
		character == '[' ||
		character == ']' ||
		!isNotQuote(character))
}

func isVariableName(character rune) bool {
	return unicode.IsLetter(character) ||
		unicode.IsDigit(character) ||
		character == '_' ||
		character == '.'
}

func isNotClosingBracket(character rune) bool {
	return character != ']'
}

// Attempts to parse the [candidate] as a Time.
// Tries a series of standardized date formats, returns the Time if one applies,
// otherwise returns false through the second return.
func tryParseTime(candidate string) (time.Time, bool) {
	timeFormats := []string{
		time.ANSIC,
		time.UnixDate,
		time.RubyDate,
		time.Kitchen,
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02",                         // RFC 3339
		"2006-01-02 15:04",                   // RFC 3339 with minutes
		"2006-01-02 15:04:05",                // RFC 3339 with seconds
		"2006-01-02 15:04:05-07:00",          // RFC 3339 with seconds and timezone
		"2006-01-02T15Z0700",                 // ISO8601 with hour
		"2006-01-02T15:04Z0700",              // ISO8601 with minutes
		"2006-01-02T15:04:05Z0700",           // ISO8601 with seconds
		"2006-01-02T15:04:05.999999999Z0700", // ISO8601 with nanoseconds
	}

	for _, format := range timeFormats {
		ret, found := tryParseExactTime(candidate, format)
		if found {
			return ret, true
		}
	}
	return time.Now(), false
}

func tryParseExactTime(candidate string, format string) (time.Time, bool) {
	ret, err := time.ParseInLocation(format, candidate, time.Local)
	if err != nil {
		return time.Now(), false
	}
	return ret, true
}

func getFirstRune(candidate string) rune {
	for _, r := range candidate {
		return r
	}
	return 0
}
