package parser

import (
	"rahu/lexer"
	a "rahu/parser/ast"
)

func canStartExpression(t lexer.TokenType) bool {
	switch t {
	case lexer.NAME, lexer.NUMBER, lexer.STRING, lexer.LPAR, lexer.LSQB, lexer.MINUS, lexer.PLUS, lexer.NOT, lexer.TRUE, lexer.FALSE, lexer.NONE:
		return true
	case lexer.UNTERMINATED_STRING:
		return false
	default:
		return false
	}
}

func (p *Parser) isAugAssign() bool {
	switch p.current.Type {
	case lexer.PLUSEQUAL,
		lexer.MINEQUAL,
		lexer.SLASHEQUAL,
		lexer.STAREQUAL,
		lexer.DOUBLESLASHEQUAL,
		lexer.DOUBLESTAREQUAL,
		lexer.AMPEREQUAL,
		lexer.LEFTSHIFTEQUAL,
		lexer.RIGHTSHIFTEQUAL:
		return true
	default:
		return false
	}
}

func (p *Parser) consumeOptionalNewline() {
	if p.current.Type == lexer.NEWLINE {
		p.advance()
	}
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
