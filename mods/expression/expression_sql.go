package expression

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// Returns a string representing this expression as if it were written in SQL.
// This function assumes that all parameters exist within the same table, and that the table essentially represents
// a serialized object of some sort (e.g., hibernate).
// If your data model is more normalized, you may need to consider iterating through each actual token given by `Tokens()`
// to create your query.
//
// Boolean values are considered to be "1" for true, "0" for false.
//
// Times are formatted according to this.QueryDateFormat.
func (ee Expression) ToSQLQuery() (string, error) {
	stream := newTokenStream(ee.tokens)
	transactions := new(expressionOutputStream)
	for stream.hasNext() {
		transaction, err := ee.findNextSQLString(stream, transactions)
		if err != nil {
			return "", err
		}
		transactions.add(transaction)
	}
	return transactions.createString(" "), nil
}

func (ee Expression) findNextSQLString(stream *tokenStream, transactions *expressionOutputStream) (string, error) {
	var ret string
	token := stream.next()

	switch token.Kind {
	case STRING:
		ret = fmt.Sprintf("'%v'", token.Value)
	case PATTERN:
		ret = fmt.Sprintf("'%s'", token.Value.(*regexp.Regexp).String())
	case TIME:
		ret = fmt.Sprintf("'%s'", token.Value.(time.Time).Format(ee.QueryDateFormat))
	case LOGICALOP:
		switch logicalSymbols[token.Value.(string)] {
		case AND:
			ret = "AND"
		case OR:
			ret = "OR"
		}
	case BOOLEAN:
		if token.Value.(bool) {
			ret = "1"
		} else {
			ret = "0"
		}
	case VARIABLE:
		ret = fmt.Sprintf("[%s]", token.Value.(string))
	case NUMERIC:
		ret = fmt.Sprintf("%g", token.Value.(float64))
	case COMPARATOR:
		switch comparatorSymbols[token.Value.(string)] {
		case EQ:
			ret = "="
		case NEQ:
			ret = "<>"
		case REQ:
			ret = "RLIKE"
		case NREQ:
			ret = "NOT RLIKE"
		default:
			ret = token.Value.(string)
		}
	case TERNARY:
		switch ternarySymbols[token.Value.(string)] {
		case COALESCE:
			left := transactions.rollback()
			right, err := ee.findNextSQLString(stream, transactions)
			if err != nil {
				return "", err
			}
			ret = fmt.Sprintf("COALESCE(%v, %v)", left, right)
		case TERNARY_TRUE:
			fallthrough
		case TERNARY_FALSE:
			return "", errors.New("ternary operators are unsupported in SQL output")
		}
	case PREFIX:
		switch prefixSymbols[token.Value.(string)] {
		case INVERT:
			ret = "NOT"
		default:
			right, err := ee.findNextSQLString(stream, transactions)
			if err != nil {
				return "", err
			}
			ret = fmt.Sprintf("%s%s", token.Value.(string), right)
		}
	case MODIFIER:
		switch modifierSymbols[token.Value.(string)] {
		case EXPONENT:
			left := transactions.rollback()
			right, err := ee.findNextSQLString(stream, transactions)
			if err != nil {
				return "", err
			}
			ret = fmt.Sprintf("POW(%s, %s)", left, right)
		case MODULUS:
			left := transactions.rollback()
			right, err := ee.findNextSQLString(stream, transactions)
			if err != nil {
				return "", err
			}
			ret = fmt.Sprintf("MOD(%s, %s)", left, right)
		default:
			ret = token.Value.(string)
		}
	case CLAUSE:
		ret = "("
	case CLAUSE_CLOSE:
		ret = ")"
	case SEPARATOR:
		ret = ","
	default:
		return "", fmt.Errorf("unrecognized query token '%s' of kind '%s'", token.Value, token.Kind)
	}

	return ret, nil
}

// Holds a series of "transactions" which represent each token as it is output by an outputter (such as ToSQLQuery()).
// Some outputs (such as SQL) require a function call or non-c-like syntax to represent an expression.
// To accomplish this, this struct keeps track of each translated token as it is output, and can return and rollback those transactions.
type expressionOutputStream struct {
	transactions []string
}

func (eos *expressionOutputStream) add(transaction string) {
	eos.transactions = append(eos.transactions, transaction)
}

func (eos *expressionOutputStream) rollback() string {
	index := len(eos.transactions) - 1
	ret := eos.transactions[index]
	eos.transactions = eos.transactions[:index]
	return ret
}

func (eos *expressionOutputStream) createString(delimiter string) string {
	var retBuffer bytes.Buffer
	var transaction string
	penultimate := len(eos.transactions) - 1
	for i := 0; i < penultimate; i++ {
		transaction = eos.transactions[i]
		retBuffer.WriteString(transaction)
		retBuffer.WriteString(delimiter)
	}
	retBuffer.WriteString(eos.transactions[penultimate])
	return retBuffer.String()
}
