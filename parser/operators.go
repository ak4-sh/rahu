package parser

import (
	"rahu/lexer"
	a "rahu/parser/ast"
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

func (p *Parser) tokenTypeToOperator(t lexer.TokenType) a.Operator {
	switch t {
	case lexer.PLUS:
		return a.Add
	case lexer.MINUS:
		return a.Sub
	case lexer.STAR:
		return a.Mult
	case lexer.SLASH:
		return a.Div
	case lexer.DOUBLESLASH:
		return a.FloorDiv
	case lexer.PERCENT:
		return a.Mod
	case lexer.DOUBLESTAR:
		return a.Pow
	default:
		return a.Add
	}
}

func tokenTypeToCompareOp(t lexer.TokenType) a.CompareOp {
	switch t {
	case lexer.EQEQUAL:
		return a.Eq
	case lexer.NOTEQUAL:
		return a.NotEq
	case lexer.LESS:
		return a.Lt
	case lexer.LESSEQUAL:
		return a.LtE
	case lexer.GREATER:
		return a.Gt
	case lexer.GREATEREQUAL:
		return a.GtE
	default:
		return a.Eq
	}
}
