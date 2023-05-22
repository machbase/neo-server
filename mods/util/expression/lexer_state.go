package expression

import "fmt"

type lexerState struct {
	isEOF          bool
	isNullable     bool
	kind           TokenKind
	validNextKinds []TokenKind
}

func (ls lexerState) canTransitionTo(kind TokenKind) bool {
	for _, k := range ls.validNextKinds {
		if k == kind {
			return true
		}
	}
	return false
}

func checkExpressionSyntax(tokens []Token) error {
	var lastToken Token
	var err error
	state := validLexerStates[0]
	for _, tok := range tokens {
		if !state.canTransitionTo(tok.Kind) {
			if lastToken.Kind == VARIABLE && tok.Kind == CLAUSE {
				return fmt.Errorf("undefined function %s", lastToken.Value.(string))
			}
			firstStateName := fmt.Sprintf("%s [%v]", state.kind.String(), lastToken.Value)
			nextStateName := fmt.Sprintf("%s [%v]", tok.Kind.String(), tok.Value)
			return fmt.Errorf("cannot transition token types from %s to %s", firstStateName, nextStateName)
		}
		state, err = getLexerStateForToken(tok.Kind)
		if err != nil {
			return err
		}
		if !state.isNullable && tok.Value == nil {
			return fmt.Errorf("token kind '%v' cannot have a nil value", tok.Kind.String())
		}
		lastToken = tok
	}
	if !state.isEOF {
		return fmt.Errorf("unexpected end of expression")
	}
	return nil
}

func getLexerStateForToken(kind TokenKind) (lexerState, error) {
	for _, possibleState := range validLexerStates {
		if possibleState.kind == kind {
			return possibleState, nil
		}
	}
	return validLexerStates[0], fmt.Errorf("no lexer state found for token kind '%v'", kind.String())
}

// lexer states.
// Constant for all purposes except compiler.
var validLexerStates = []lexerState{
	{
		kind:       UNKNOWN,
		isEOF:      false,
		isNullable: true,
		validNextKinds: []TokenKind{
			PREFIX,
			NUMERIC,
			BOOLEAN,
			VARIABLE,
			PATTERN,
			FUNCTION,
			ACCESSOR,
			STRING,
			TIME,
			CLAUSE,
		},
	},
	{
		kind:       CLAUSE,
		isEOF:      false,
		isNullable: true,
		validNextKinds: []TokenKind{
			PREFIX,
			NUMERIC,
			BOOLEAN,
			VARIABLE,
			PATTERN,
			FUNCTION,
			ACCESSOR,
			STRING,
			TIME,
			CLAUSE,
			CLAUSE_CLOSE,
		},
	},
	{
		kind:       CLAUSE_CLOSE,
		isEOF:      true,
		isNullable: true,
		validNextKinds: []TokenKind{
			COMPARATOR,
			MODIFIER,
			NUMERIC,
			BOOLEAN,
			VARIABLE,
			STRING,
			PATTERN,
			TIME,
			CLAUSE,
			CLAUSE_CLOSE,
			LOGICALOP,
			TERNARY,
			SEPARATOR,
		},
	},
	{
		kind:       NUMERIC,
		isEOF:      true,
		isNullable: false,
		validNextKinds: []TokenKind{
			MODIFIER,
			COMPARATOR,
			LOGICALOP,
			CLAUSE_CLOSE,
			TERNARY,
			SEPARATOR,
		},
	},
	{
		kind:       BOOLEAN,
		isEOF:      true,
		isNullable: false,
		validNextKinds: []TokenKind{
			MODIFIER,
			COMPARATOR,
			LOGICALOP,
			CLAUSE_CLOSE,
			TERNARY,
			SEPARATOR,
		},
	},
	{
		kind:       STRING,
		isEOF:      true,
		isNullable: false,
		validNextKinds: []TokenKind{
			MODIFIER,
			COMPARATOR,
			LOGICALOP,
			CLAUSE_CLOSE,
			TERNARY,
			SEPARATOR,
		},
	},
	{
		kind:       TIME,
		isEOF:      true,
		isNullable: false,
		validNextKinds: []TokenKind{
			MODIFIER,
			COMPARATOR,
			LOGICALOP,
			CLAUSE_CLOSE,
			SEPARATOR,
		},
	},
	{
		kind:       PATTERN,
		isEOF:      true,
		isNullable: false,
		validNextKinds: []TokenKind{
			MODIFIER,
			COMPARATOR,
			LOGICALOP,
			CLAUSE_CLOSE,
			SEPARATOR,
		},
	},
	{
		kind:       VARIABLE,
		isEOF:      true,
		isNullable: false,
		validNextKinds: []TokenKind{
			MODIFIER,
			COMPARATOR,
			LOGICALOP,
			CLAUSE_CLOSE,
			TERNARY,
			SEPARATOR,
		},
	},
	{
		kind:       MODIFIER,
		isEOF:      false,
		isNullable: false,
		validNextKinds: []TokenKind{
			PREFIX,
			NUMERIC,
			VARIABLE,
			FUNCTION,
			ACCESSOR,
			STRING,
			BOOLEAN,
			CLAUSE,
			CLAUSE_CLOSE,
		},
	},
	{
		kind:       COMPARATOR,
		isEOF:      false,
		isNullable: false,
		validNextKinds: []TokenKind{
			PREFIX,
			NUMERIC,
			BOOLEAN,
			VARIABLE,
			FUNCTION,
			ACCESSOR,
			STRING,
			TIME,
			CLAUSE,
			CLAUSE_CLOSE,
			PATTERN,
		},
	},
	{
		kind:       LOGICALOP,
		isEOF:      false,
		isNullable: false,
		validNextKinds: []TokenKind{
			PREFIX,
			NUMERIC,
			BOOLEAN,
			VARIABLE,
			FUNCTION,
			ACCESSOR,
			STRING,
			TIME,
			CLAUSE,
			CLAUSE_CLOSE,
		},
	},
	{
		kind:       PREFIX,
		isEOF:      false,
		isNullable: false,
		validNextKinds: []TokenKind{
			NUMERIC,
			BOOLEAN,
			VARIABLE,
			FUNCTION,
			ACCESSOR,
			CLAUSE,
			CLAUSE_CLOSE,
		},
	},
	{
		kind:       TERNARY,
		isEOF:      false,
		isNullable: false,
		validNextKinds: []TokenKind{
			PREFIX,
			NUMERIC,
			BOOLEAN,
			STRING,
			TIME,
			VARIABLE,
			FUNCTION,
			ACCESSOR,
			CLAUSE,
			SEPARATOR,
		},
	},
	{
		kind:       FUNCTION,
		isEOF:      false,
		isNullable: false,
		validNextKinds: []TokenKind{
			CLAUSE,
		},
	},
	{
		kind:       ACCESSOR,
		isEOF:      true,
		isNullable: false,
		validNextKinds: []TokenKind{
			CLAUSE,
			MODIFIER,
			COMPARATOR,
			LOGICALOP,
			CLAUSE_CLOSE,
			TERNARY,
			SEPARATOR,
		},
	},
	{
		kind:       SEPARATOR,
		isEOF:      false,
		isNullable: true,
		validNextKinds: []TokenKind{
			PREFIX,
			NUMERIC,
			BOOLEAN,
			STRING,
			TIME,
			VARIABLE,
			FUNCTION,
			ACCESSOR,
			CLAUSE,
		},
	},
}
