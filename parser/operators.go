package parser

import (
	"rahu/lexer"
	a "rahu/parser/ast"
)

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
