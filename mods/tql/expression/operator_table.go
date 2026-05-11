package expression

type infixBindingPower struct {
	left  int
	right int
}

func infixBindingPowerFor(symbol OperatorSymbol) (infixBindingPower, bool) {
	switch symbol {
	case SEPARATE:
		return infixBindingPower{left: 10, right: 11}, true
	case TERNARY_TRUE:
		return infixBindingPower{left: 20, right: 19}, true
	case COALESCE:
		return infixBindingPower{left: 30, right: 29}, true
	case OR:
		return infixBindingPower{left: 40, right: 41}, true
	case AND:
		return infixBindingPower{left: 50, right: 51}, true
	case EQ, NEQ, GT, LT, GTE, LTE, REQ, NREQ, IN:
		return infixBindingPower{left: 60, right: 61}, true
	case BITWISE_OR, BITWISE_XOR, BITWISE_AND:
		return infixBindingPower{left: 70, right: 71}, true
	case BITWISE_LSHIFT, BITWISE_RSHIFT:
		return infixBindingPower{left: 80, right: 81}, true
	case PLUS, MINUS:
		return infixBindingPower{left: 90, right: 91}, true
	case MULTIPLY, DIVIDE, MODULUS:
		return infixBindingPower{left: 100, right: 101}, true
	case EXPONENT:
		return infixBindingPower{left: 110, right: 110}, true
	default:
		return infixBindingPower{}, false
	}
}

func infixOperatorForToken(tok Token) (OperatorSymbol, bool) {
	switch tok.Kind {
	case MODIFIER:
		s, ok := modifierSymbols[tok.Value.(string)]
		return s, ok
	case COMPARATOR:
		s, ok := comparatorSymbols[tok.Value.(string)]
		return s, ok
	case LOGICALOP:
		s, ok := logicalSymbols[tok.Value.(string)]
		return s, ok
	case TERNARY:
		s, ok := ternarySymbols[tok.Value.(string)]
		return s, ok
	case SEPARATOR:
		return SEPARATE, true
	default:
		return 0, false
	}
}
