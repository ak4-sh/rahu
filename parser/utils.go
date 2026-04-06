package parser

import (
	"rahu/lexer"
	a "rahu/parser/ast"
)

func canStartExpression(t lexer.TokenType) bool {
	switch t {
	case lexer.NAME, lexer.NUMBER, lexer.STRING, lexer.FSTRING, lexer.LPAR, lexer.LSQB, lexer.MINUS, lexer.PLUS, lexer.NOT, lexer.TRUE, lexer.FALSE, lexer.NONE:
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

func (p *Parser) consumeBlankLinesBeforeIndent() {
	for p.current.Type == lexer.NEWLINE {
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
	case lexer.VBAR:
		return a.BitOr
	case lexer.DOUBLESTAR:
		return a.Pow
	default:
		return a.Add
	}
}

func tokenTypesToCompareOp(t lexer.TokenType, peek lexer.TokenType) a.CompareOp {
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
	case lexer.IN:
		return a.In
	case lexer.NOT:
		if peek == lexer.IN {
			return a.NotIn
		}
	case lexer.IS:
		if peek == lexer.NOT {
			return a.IsNot
		}
		return a.Is
	}
	return a.Eq
}

func compareOpTokenWidth(t lexer.TokenType, peek lexer.TokenType) int {
	if t == lexer.NOT && peek == lexer.IN {
		return 2
	}
	if t == lexer.IS && peek == lexer.NOT {
		return 2
	}
	if isCompareOp(t, peek) {
		return 1
	}
	return 0
}
