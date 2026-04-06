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
	case lexer.EQEQUAL, lexer.NOTEQUAL, lexer.LESS, lexer.LESSEQUAL, lexer.GREATER, lexer.GREATEREQUAL, lexer.IN, lexer.IS:
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

func isCompareOp(t lexer.TokenType, peek lexer.TokenType) bool {
	switch t {
	case lexer.EQEQUAL, lexer.NOTEQUAL, lexer.LESS, lexer.LESSEQUAL, lexer.GREATER, lexer.GREATEREQUAL, lexer.IN:
		return true
	case lexer.IS:
		return true
	case lexer.NOT:
		return peek == lexer.IN
	default:
		return false
	}
}

func (p *Parser) isOperator(t lexer.TokenType) bool {
	return t == lexer.PLUS || t == lexer.MINUS || t == lexer.STAR || t == lexer.SLASH || t == lexer.DOUBLESLASH || t == lexer.PERCENT
}
