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
		Text: p.current.Literal,
		Pos: a.Range{
			Start: int(p.current.Start),
			End:   int(p.current.End),
		},
		ID: p.newNodeID(),
	}

	p.advance() // consume name

	return &a.Attribute{
		Pos: a.Range{
			Start: int(start),
			End:   int(p.current.End),
		},
		Attr:  attr,
		Value: left,
		ID:    p.newNodeID(),
	}
}
