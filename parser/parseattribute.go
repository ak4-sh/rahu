package parser

import (
	l "rahu/lexer"
	a "rahu/parser/ast"
)

func (p *Parser) parseAttribute(left a.Expression) a.Expression {
	start := p.current.Start
	p.advance() // consume `.`

	if p.current.Type != l.NAME {
		p.errorCurrent("expected name after `.`")
		return left
	}

	attr := &a.Name{
		ID: p.current.Literal,
		Pos: a.Range{
			Start: p.current.Start,
			End:   p.current.End,
		},
	}

	p.advance() // consume name

	return &a.Attribute{
		Pos: a.Range{
			Start: start,
			End:   p.current.End,
		},
		Attr:  attr,
		Value: left,
	}
}
