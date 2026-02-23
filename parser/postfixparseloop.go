package parser

import (
	"rahu/lexer"
	a "rahu/parser/ast"
)

func (p *Parser) postfixParseLoop(left a.Expression) a.Expression {
	for {
		switch p.current.Type {
		case lexer.LPAR:
			left = p.parseCall(left)

		case lexer.DOT:
			left = p.parseAttribute(left)

		default:
			return left
		}
	}
}
