package parser

import (
	l "rahu/lexer"
	a "rahu/parser/ast"
)

func (p *Parser) parseDict() a.NodeID {
	start := p.current.Start
	p.advance() // consume '{'

	ret := p.tree.NewNode(a.NodeDict, start, start)
	if p.current.Type == l.RBRACE {
		end := p.current.Start
		p.advance()
		p.tree.Nodes[ret].End = end
		return ret
	}

	for {
		key := p.parseExpression(LOWEST)
		if key == a.NoNode {
			p.errorCurrent("expected expression for dict key")
			return p.tree.NewNode(a.NodeErrExp, start, p.current.End)
		}
		if p.current.Type != l.COLON {
			p.errorCurrent("expected ':' after dict key")
			return p.tree.NewNode(a.NodeErrExp, start, p.current.End)
		}
		p.advance()

		value := p.parseExpression(LOWEST)
		if value == a.NoNode {
			p.errorCurrent("expected expression for dict value")
			return p.tree.NewNode(a.NodeErrExp, start, p.current.End)
		}

		if p.current.Type == l.FOR {
			return p.parseDictComp(start, key, value)
		}

		p.tree.AddChild(ret, key)
		p.tree.AddChild(ret, value)
		p.tree.Nodes[ret].End = p.tree.Nodes[value].End

		if p.current.Type != l.COMMA {
			break
		}
		p.advance()
		if p.current.Type == l.RBRACE {
			break
		}
	}

	if p.current.Type != l.RBRACE {
		p.errorCurrent("expected '}' after dict literal")
		return p.tree.NewNode(a.NodeErrExp, start, p.current.End)
	}
	end := p.current.Start
	p.advance()
	p.tree.Nodes[ret].End = end
	return ret
}

func (p *Parser) parseDictComp(start uint32, key, value a.NodeID) a.NodeID {
	ret := p.tree.NewNode(a.NodeDictComp, start, p.tree.Nodes[value].End)
	p.tree.AddChild(ret, key)
	p.tree.AddChild(ret, value)

	for p.current.Type == l.FOR {
		clause := p.parseComprehensionClause()
		if clause == a.NoNode {
			return p.tree.NewNode(a.NodeErrExp, start, p.current.End)
		}
		p.tree.AddChild(ret, clause)
		p.tree.Nodes[ret].End = p.tree.Nodes[clause].End
	}

	if p.current.Type != l.RBRACE {
		p.errorCurrent("expected '}' after dict comprehension")
		return p.tree.NewNode(a.NodeErrExp, start, p.current.End)
	}
	end := p.current.Start
	p.advance()
	p.tree.Nodes[ret].End = end
	return ret
}
