package expression

type ExpressionToken struct {
	Kind  TokenKind
	Value any
}

type TokenKind int

const (
	UNKNOWN TokenKind = iota

	PREFIX
	NUMERIC
	BOOLEAN
	STRING
	PATTERN
	TIME
	VARIABLE
	FUNCTION
	SEPARATOR
	ACCESSOR

	COMPARATOR
	LOGICALOP
	MODIFIER

	CLAUSE
	CLAUSE_CLOSE

	TERNARY
)

func (kind TokenKind) String() string {
	switch kind {
	case PREFIX:
		return "PREFIX"
	case NUMERIC:
		return "NUMERIC"
	case BOOLEAN:
		return "BOOLEAN"
	case STRING:
		return "STRING"
	case PATTERN:
		return "PATTERN"
	case TIME:
		return "TIME"
	case VARIABLE:
		return "VARIABLE"
	case FUNCTION:
		return "FUNCTION"
	case SEPARATOR:
		return "SEPARATOR"
	case COMPARATOR:
		return "COMPARATOR"
	case LOGICALOP:
		return "LOGICALOP"
	case MODIFIER:
		return "MODIFIER"
	case CLAUSE:
		return "CLAUSE"
	case CLAUSE_CLOSE:
		return "CLAUSE_CLOSE"
	case TERNARY:
		return "TERNARY"
	case ACCESSOR:
		return "ACCESSOR"
	}
	return "UNKNOWN"
}

type tokenStream struct {
	tokens []ExpressionToken
	index  int
	length int
}

func newTokenStream(tokens []ExpressionToken) *tokenStream {
	return &tokenStream{
		tokens: tokens,
		index:  0,
		length: len(tokens),
	}
}

func (ts *tokenStream) rewind() {
	ts.index -= 1
}

func (ts *tokenStream) next() ExpressionToken {
	tok := ts.tokens[ts.index]
	ts.index++
	return tok
}

func (ts *tokenStream) hasNext() bool {
	return ts.index < ts.length
}
