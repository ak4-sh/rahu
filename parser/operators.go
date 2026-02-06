package parser

import (
	"rahu/lexer"
)

const (
	LOWEST = iota
	OR
	AND
	NOT
	COMPARE
	SUM
	PRODUCT
	PREFIX
	POW
)

func infixBindingPower(t lexer.TokenType) int {
	switch t {
	case lexer.PLUS, lexer.MINUS:
		return SUM
	case lexer.STAR, lexer.SLASH, lexer.DOUBLESLASH, lexer.PERCENT:
		return PRODUCT
	case lexer.EQEQUAL, lexer.NOTEQUAL, lexer.LESS, lexer.LESSEQUAL, lexer.GREATER, lexer.GREATEREQUAL:
		return COMPARE
	case lexer.OR:
		return OR
	case lexer.AND:
		return AND
	case lexer.DOUBLESTAR:
		return POW
	default:
		return LOWEST
	}
}

func isCompareOp(t lexer.TokenType) bool {
	switch t {
	case lexer.EQEQUAL, lexer.NOTEQUAL, lexer.LESS, lexer.LESSEQUAL, lexer.GREATER, lexer.GREATEREQUAL:
		return true
	default:
		return false
	}
}

func (p *Parser) isOperator(t lexer.TokenType) bool {
	return t == lexer.PLUS || t == lexer.MINUS || t == lexer.STAR || t == lexer.SLASH || t == lexer.DOUBLESLASH || t == lexer.PERCENT
}

func (p *Parser) tokenTypeToOperator(t lexer.TokenType) Operator {
	switch t {
	case lexer.PLUS:
		return Add
	case lexer.MINUS:
		return Sub
	case lexer.STAR:
		return Mult
	case lexer.SLASH:
		return Div
	case lexer.DOUBLESLASH:
		return FloorDiv
	case lexer.PERCENT:
		return Mod
	case lexer.DOUBLESTAR:
		return Pow
	default:
		return Add
	}
}

func tokenTypeToCompareOp(t lexer.TokenType) CompareOp {
	switch t {
	case lexer.EQEQUAL:
		return Eq
	case lexer.NOTEQUAL:
		return NotEq
	case lexer.LESS:
		return Lt
	case lexer.LESSEQUAL:
		return LtE
	case lexer.GREATER:
		return Gt
	case lexer.GREATEREQUAL:
		return GtE
	default:
		return Eq
	}
}
