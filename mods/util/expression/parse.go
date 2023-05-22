package expression

import "errors"

func parseTokens(input string, functions map[string]ExpressionFunction) ([]ExpressionToken, error) {
	var ret []ExpressionToken
	var token ExpressionToken
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

func readToken(stream *lexerStream, state lexerState, functions map[string]ExpressionFunction) (ExpressionToken, error, bool) {
	var ret ExpressionToken
	return ret, nil, false
}

func getFirstRune(candidate string) rune {
	for _, r := range candidate {
		return r
	}
	return 0
}

// checks the balance of tokens which have multiple parts, such as parenthesis.
func checkBalance(tokens []ExpressionToken) error {
	var stream *tokenStream
	var token ExpressionToken
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
