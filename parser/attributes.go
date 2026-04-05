package parser

import (
	l "rahu/lexer"
	a "rahu/parser/ast"
)

func (p *Parser) parseAttribute(left a.NodeID) a.NodeID {
	start := p.tree.Nodes[left].Start
	p.advance() // consume `.`

	if p.current.Type != l.NAME {
		p.errorCurrent("expected name after `.`")
		return left
	}

	attr := p.tree.NewNameNode(p.current.Start, p.current.End, p.current.Literal)

	p.advance() // consume name

	ret := p.tree.NewNode(a.NodeAttribute, start, p.tree.Nodes[attr].End)
	p.tree.AddChild(ret, left)
	p.tree.AddChild(ret, attr)
	return ret
}
