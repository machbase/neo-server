package expression

import (
	"fmt"
	"time"
)

var stageSymbolMap = map[OperatorSymbol]evaluationOperator{
	EQ:             equalStage,
	NEQ:            notEqualStage,
	GT:             gtStage,
	LT:             ltStage,
	GTE:            gteStage,
	LTE:            lteStage,
	REQ:            regexStage,
	NREQ:           notRegexStage,
	AND:            andStage,
	OR:             orStage,
	IN:             inStage,
	BITWISE_OR:     bitwiseOrStage,
	BITWISE_AND:    bitwiseAndStage,
	BITWISE_XOR:    bitwiseXORStage,
	BITWISE_LSHIFT: leftShiftStage,
	BITWISE_RSHIFT: rightShiftStage,
	PLUS:           addStage,
	MINUS:          subtractStage,
	MULTIPLY:       multiplyStage,
	DIVIDE:         divideStage,
	MODULUS:        modulusStage,
	EXPONENT:       exponentStage,
	NEGATE:         negateStage,
	INVERT:         invertStage,
	BITWISE_NOT:    bitwiseNotStage,
	TERNARY_TRUE:   ternaryIfStage,
	TERNARY_FALSE:  ternaryElseStage,
	COALESCE:       ternaryElseStage,
	SEPARATE:       separatorStage,
}

func planStages(tokens []Token) (*evaluationStage, error) {
	stream := newTokenStream(tokens)
	stage, err := parseExpression(stream, 0)
	if err != nil {
		return nil, err
	}
	if stream.hasNext() {
		tok := stream.peek()
		return nil, newParseError("unexpected_token", tok.Span, tok.Raw, fmt.Sprintf("unexpected token '%v'", tok.Value), nil)
	}
	stage = elideLiterals(stage)
	return stage, nil
}

func parseExpression(stream *tokenStream, minBP int) (*evaluationStage, error) {
	left, err := parsePrefix(stream)
	if err != nil {
		return nil, err
	}

	for stream.hasNext() {
		tok := stream.peek()
		if tok.Kind == CLAUSE_CLOSE {
			break
		}
		symbol, ok := infixOperatorForToken(tok)
		if !ok {
			break
		}
		bp, ok := infixBindingPowerFor(symbol)
		if !ok || bp.left < minBP {
			break
		}
		stream.next()

		switch symbol {
		case TERNARY_TRUE:
			left, err = parseTernary(stream, left, tok)
		default:
			right, rightErr := parseExpression(stream, bp.right)
			if rightErr != nil {
				return nil, rightErr
			}
			left = makeInfixStage(symbol, tok, left, right)
		}
		if err != nil {
			return nil, err
		}
	}
	return left, nil
}

func parsePrefix(stream *tokenStream) (*evaluationStage, error) {
	if !stream.hasNext() {
		return nil, newParseError("unexpected_end", SourceSpan{}, "", "unexpected end of expression", nil)
	}

	tok := stream.next()
	switch tok.Kind {
	case PREFIX:
		symbol, ok := prefixSymbols[tok.Value.(string)]
		if !ok {
			return nil, newParseError("invalid_prefix", tok.Span, tok.Raw, fmt.Sprintf("invalid prefix '%v'", tok.Value), nil)
		}
		right, err := parseExpression(stream, 120)
		if err != nil {
			return nil, err
		}
		checks := findTypeChecks(symbol)
		return &evaluationStage{
			symbol:          symbol,
			rightStage:      right,
			operator:        stageSymbolMap[symbol],
			leftTypeCheck:   checks.left,
			rightTypeCheck:  checks.right,
			typeCheck:       checks.combined,
			typeErrorFormat: prefixErrorFormat,
			span:            mergeSpan(tok.Span, right.span),
		}, nil
	case CLAUSE:
		if stream.hasNext() && stream.peek().Kind == CLAUSE_CLOSE {
			closeTok := stream.next()
			return &evaluationStage{
				symbol:   NOOP,
				operator: noopStageRight,
				span:     mergeSpan(tok.Span, closeTok.Span),
			}, nil
		}
		expr, err := parseExpression(stream, 0)
		if err != nil {
			return nil, err
		}
		if !stream.hasNext() || stream.peek().Kind != CLAUSE_CLOSE {
			return nil, newParseError("unclosed_clause", tok.Span, tok.Raw, "unbalanced parenthesis", nil)
		}
		closeTok := stream.next()
		expr.span = mergeSpan(tok.Span, closeTok.Span)
		return expr, nil
	case FUNCTION:
		args, span, err := parseCallArguments(stream, tok)
		if err != nil {
			return nil, err
		}
		return &evaluationStage{
			symbol:          FUNCTIONAL,
			rightStage:      args,
			operator:        makeFunctionStage(tok.Value.(Function)),
			typeErrorFormat: "Unable to run function '%v': %v",
			span:            span,
		}, nil
	case ACCESSOR:
		args, span, err := parseOptionalCallArguments(stream, tok)
		if err != nil {
			return nil, err
		}
		return &evaluationStage{
			symbol:          ACCESS,
			rightStage:      args,
			operator:        makeAccessorStage(tok.Value.([]string)),
			typeErrorFormat: "Unable to access parameter field or method '%v': %v",
			span:            span,
		}, nil
	case VARIABLE:
		return &evaluationStage{
			symbol:   VALUE,
			operator: makeParameterStage(tok.Value.(string)),
			span:     tok.Span,
		}, nil
	case NUMERIC, STRING, PATTERN, BOOLEAN:
		return &evaluationStage{
			symbol:   LITERAL,
			operator: makeLiteralStage(tok.Value),
			span:     tok.Span,
		}, nil
	case TIME:
		return &evaluationStage{
			symbol:   LITERAL,
			operator: makeLiteralStage(float64(tok.Value.(time.Time).Unix())),
			span:     tok.Span,
		}, nil
	case CLAUSE_CLOSE:
		return nil, newParseError("unexpected_clause_close", tok.Span, tok.Raw, "unexpected closing parenthesis", nil)
	default:
		return nil, newParseError("unexpected_token", tok.Span, tok.Raw, fmt.Sprintf("Unable to plan token kind: '%s', value: '%v'", tok.Kind.String(), tok.Value), nil)
	}
}

func parseCallArguments(stream *tokenStream, callTok Token) (*evaluationStage, SourceSpan, error) {
	if !stream.hasNext() || stream.peek().Kind != CLAUSE {
		return nil, callTok.Span, nil
	}
	openTok := stream.next()
	if stream.hasNext() && stream.peek().Kind == CLAUSE_CLOSE {
		closeTok := stream.next()
		return nil, mergeSpan(callTok.Span, closeTok.Span), nil
	}
	args, err := parseExpression(stream, 0)
	if err != nil {
		return nil, SourceSpan{}, err
	}
	if !stream.hasNext() || stream.peek().Kind != CLAUSE_CLOSE {
		return nil, SourceSpan{}, newParseError("unclosed_call", openTok.Span, openTok.Raw, "unbalanced parenthesis", nil)
	}
	closeTok := stream.next()
	return args, mergeSpan(callTok.Span, closeTok.Span), nil
}

func parseOptionalCallArguments(stream *tokenStream, tok Token) (*evaluationStage, SourceSpan, error) {
	if !stream.hasNext() || stream.peek().Kind != CLAUSE {
		return nil, tok.Span, nil
	}
	return parseCallArguments(stream, tok)
}

func parseTernary(stream *tokenStream, cond *evaluationStage, qTok Token) (*evaluationStage, error) {
	trueExpr, err := parseExpression(stream, 0)
	if err != nil {
		return nil, err
	}
	trueChecks := findTypeChecks(TERNARY_TRUE)
	trueStage := &evaluationStage{
		symbol:          TERNARY_TRUE,
		leftStage:       cond,
		rightStage:      trueExpr,
		operator:        stageSymbolMap[TERNARY_TRUE],
		leftTypeCheck:   trueChecks.left,
		rightTypeCheck:  trueChecks.right,
		typeCheck:       trueChecks.combined,
		typeErrorFormat: ternaryErrorFormat,
		span:            mergeSpan(cond.span, trueExpr.span),
	}
	if !stream.hasNext() {
		return trueStage, nil
	}
	colonTok := stream.peek()
	if colonTok.Kind != TERNARY || colonTok.Value.(string) != ":" {
		return trueStage, nil
	}
	stream.next()
	falseExpr, err := parseExpression(stream, 19)
	if err != nil {
		return nil, err
	}
	falseChecks := findTypeChecks(TERNARY_FALSE)
	return &evaluationStage{
		symbol:          TERNARY_FALSE,
		leftStage:       trueStage,
		rightStage:      falseExpr,
		operator:        stageSymbolMap[TERNARY_FALSE],
		leftTypeCheck:   falseChecks.left,
		rightTypeCheck:  falseChecks.right,
		typeCheck:       falseChecks.combined,
		typeErrorFormat: ternaryErrorFormat,
		span:            mergeSpan(cond.span, falseExpr.span),
	}, nil
}

func makeInfixStage(symbol OperatorSymbol, tok Token, left, right *evaluationStage) *evaluationStage {
	checks := findTypeChecks(symbol)
	format := ""
	switch symbol {
	case AND, OR:
		format = logicalErrorFormat
	case EQ, NEQ, GT, LT, GTE, LTE, REQ, NREQ, IN:
		format = comparatorErrorFormat
	case PLUS, MINUS, MULTIPLY, DIVIDE, MODULUS, EXPONENT, BITWISE_AND, BITWISE_OR, BITWISE_XOR, BITWISE_LSHIFT, BITWISE_RSHIFT:
		format = modifierErrorFormat
	case COALESCE:
		format = ternaryErrorFormat
	}
	return &evaluationStage{
		symbol:          symbol,
		leftStage:       left,
		rightStage:      right,
		operator:        stageSymbolMap[symbol],
		leftTypeCheck:   checks.left,
		rightTypeCheck:  checks.right,
		typeCheck:       checks.combined,
		typeErrorFormat: format,
		span:            mergeSpan(left.span, right.span),
	}
}

// Convenience function to pass a triplet of typechecks between `findTypeChecks` and `planPrecedenceLevel`.
// Each of these members may be nil, which indicates that type does not matter for that value.
type typeChecks struct {
	left     stageTypeCheck
	right    stageTypeCheck
	combined stageCombinedTypeCheck
}

// Maps a given [symbol] to a set of typechecks to be used during runtime.
func findTypeChecks(symbol OperatorSymbol) typeChecks {
	switch symbol {
	case GT:
		fallthrough
	case LT:
		fallthrough
	case GTE:
		fallthrough
	case LTE:
		return typeChecks{
			combined: comparatorTypeCheck,
		}
	case REQ:
		fallthrough
	case NREQ:
		return typeChecks{
			left:  isString,
			right: isRegexOrString,
		}
	case AND:
		fallthrough
	case OR:
		return typeChecks{
			left:  isBool,
			right: isBool,
		}
	case IN:
		return typeChecks{
			right: isArray,
		}
	case BITWISE_LSHIFT:
		fallthrough
	case BITWISE_RSHIFT:
		fallthrough
	case BITWISE_OR:
		fallthrough
	case BITWISE_AND:
		fallthrough
	case BITWISE_XOR:
		return typeChecks{
			left:  isFloat64,
			right: isFloat64,
		}
	case PLUS:
		return typeChecks{
			combined: additionTypeCheck,
		}
	case MINUS:
		fallthrough
	case MULTIPLY:
		fallthrough
	case DIVIDE:
		fallthrough
	case MODULUS:
		fallthrough
	case EXPONENT:
		return typeChecks{
			left:  isFloat64,
			right: isFloat64,
		}
	case NEGATE:
		return typeChecks{
			right: isFloat64,
		}
	case INVERT:
		return typeChecks{
			right: isBool,
		}
	case BITWISE_NOT:
		return typeChecks{
			right: isFloat64,
		}
	case TERNARY_TRUE:
		return typeChecks{
			left: isBool,
		}
	case EQ:
		fallthrough
	case NEQ:
		return typeChecks{}
	case TERNARY_FALSE:
		fallthrough
	case COALESCE:
		fallthrough
	default:
		return typeChecks{}
	}
}

// Recurses through all operators in the entire tree, eliding operators where both sides are literals.
func elideLiterals(root *evaluationStage) *evaluationStage {
	if root == nil {
		return nil
	}
	if root.leftStage != nil {
		root.leftStage = elideLiterals(root.leftStage)
	}
	if root.rightStage != nil {
		root.rightStage = elideLiterals(root.rightStage)
	}
	return elideStage(root)
}

// Elides a specific stage, if possible.
func elideStage(root *evaluationStage) *evaluationStage {
	var leftValue, rightValue, result interface{}
	var err error

	if root.rightStage == nil ||
		root.rightStage.symbol != LITERAL ||
		root.leftStage == nil ||
		root.leftStage.symbol != LITERAL {
		return root
	}

	switch root.symbol {
	case SEPARATE, IN:
		return root
	}

	leftValue, err = root.leftStage.operator(nil, nil, nil)
	if err != nil {
		return root
	}
	rightValue, err = root.rightStage.operator(nil, nil, nil)
	if err != nil {
		return root
	}

	err = typeCheck(root.leftTypeCheck, leftValue, root.symbol, root.typeErrorFormat)
	if err != nil {
		return root
	}
	err = typeCheck(root.rightTypeCheck, rightValue, root.symbol, root.typeErrorFormat)
	if err != nil {
		return root
	}
	if root.typeCheck != nil && !root.typeCheck(leftValue, rightValue) {
		return root
	}
	result, err = root.operator(leftValue, rightValue, nil)
	if err != nil {
		return root
	}

	return &evaluationStage{
		symbol:   LITERAL,
		operator: makeLiteralStage(result),
		span:     root.span,
	}
}

func mergeSpan(left, right SourceSpan) SourceSpan {
	if left.IsZero() {
		return right
	}
	if right.IsZero() {
		return left
	}
	return SourceSpan{Start: left.Start, End: right.End}
}
