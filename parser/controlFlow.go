package parser

import (
	l "rahu/lexer"
	a "rahu/parser/ast"
)

func (p *Parser) parseFor() a.NodeID {
	startPos := p.current.Start
	p.advance()

	ret := p.tree.NewNode(a.NodeFor, startPos, startPos)

	target := p.parseForTarget()
	if target == a.NoNode {
		p.errorCurrent("invalid expression for loop target")
		target = p.tree.NewNode(a.NodeErrExp, p.current.Start, p.current.Start)
	}
	p.tree.AddChild(ret, target)

	if p.current.Type != l.IN {
		p.errorCurrent("expected 'in' after loop variable")
		p.syncTo(l.IN, l.COLON, l.NEWLINE, l.EOF)
		if p.current.Type != l.IN {
			p.tree.Nodes[ret].End = p.current.Start
			return ret
		}
	}
	p.advance()

	iter := p.parseExpression(LOWEST)
	if iter == a.NoNode {
		p.errorCurrent("invalid expression for loop iterator")
		iter = p.tree.NewNode(a.NodeErrExp, p.current.Start, p.current.Start)
		p.tree.AddChild(ret, iter)
		p.tree.Nodes[ret].End = p.current.Start
		return ret
	}
	p.tree.AddChild(ret, iter)

	if p.current.Type != l.COLON {
		p.errorCurrent("expected ':' after for clause")
		p.syncTo(l.COLON, l.NEWLINE, l.EOF)
		if p.current.Type != l.COLON {
			p.tree.Nodes[ret].End = p.current.Start
			return ret
		}
	}
	p.advance()

	if p.current.Type != l.NEWLINE {
		p.errorCurrent("expected newline after ':'")
		p.syncTo(l.NEWLINE, l.EOF)
		if p.current.Type != l.NEWLINE {
			p.tree.Nodes[ret].End = p.current.Start
			return ret
		}
	}
	p.advance()

	if p.current.Type != l.INDENT {
		p.errorCurrent("expected indent after for statement")
		p.syncTo(l.INDENT, l.DEDENT, l.EOF)
		if p.current.Type != l.INDENT {
			p.tree.Nodes[ret].End = p.current.Start
			return ret
		}
	}
	p.advance()

	body := []a.NodeID{}
	for p.current.Type != l.DEDENT && p.current.Type != l.EOF {
		stmt := p.parseStatement()
		if stmt != a.NoNode {
			body = append(body, stmt)
		}
	}

	bodyStart, bodyEnd := p.current.Start, p.current.Start
	if len(body) > 0 {
		bodyStart = p.tree.Nodes[body[0]].Start
		bodyEnd = p.tree.Nodes[body[len(body)-1]].End
	}
	bodyBlock := p.tree.NewNode(a.NodeBlock, bodyStart, bodyEnd)
	for _, child := range body {
		p.tree.AddChild(bodyBlock, child)
	}
	p.tree.AddChild(ret, bodyBlock)

	endPos := p.current.Start
	if p.current.Type == l.DEDENT {
		p.advance()
	}

	orelse := []a.NodeID{}
	if p.current.Type == l.ELSE {
		p.advance()

		if p.current.Type != l.COLON {
			p.errorCurrent("expected ':' after else")
			p.syncTo(l.COLON, l.NEWLINE, l.EOF)
			if p.current.Type != l.COLON {
				p.tree.Nodes[ret].End = endPos
				return ret
			}
		}
		p.advance()

		if p.current.Type != l.NEWLINE {
			p.errorCurrent("expected newline after 'else:'")
			p.syncTo(l.NEWLINE, l.EOF)
			if p.current.Type != l.NEWLINE {
				p.tree.Nodes[ret].End = endPos
				return ret
			}
		}
		p.advance()

		if p.current.Type != l.INDENT {
			p.errorCurrent("expected indent block after 'else:'")
			p.syncTo(l.INDENT, l.DEDENT, l.EOF)
			if p.current.Type != l.INDENT {
				p.tree.Nodes[ret].End = endPos
				return ret
			}
		}
		p.advance()

		for p.current.Type != l.DEDENT && p.current.Type != l.EOF {
			stmt := p.parseStatement()
			if stmt != a.NoNode {
				orelse = append(orelse, stmt)
			}
		}

		elseStart, elseEnd := p.current.Start, p.current.Start
		if len(orelse) > 0 {
			elseStart = p.tree.Nodes[orelse[0]].Start
			elseEnd = p.tree.Nodes[orelse[len(orelse)-1]].End
		}
		elseBlock := p.tree.NewNode(a.NodeBlock, elseStart, elseEnd)
		for _, child := range orelse {
			p.tree.AddChild(elseBlock, child)
		}
		p.tree.AddChild(ret, elseBlock)

		endPos = p.current.Start
		if p.current.Type == l.DEDENT {
			p.advance()
		}
	}

	p.tree.Nodes[ret].End = endPos
	return ret
}

func (p *Parser) parseForTarget() a.NodeID {
	if p.current.Type != l.NAME {
		p.errorCurrent("expected variable name")
		return p.tree.NewNode(a.NodeErrExp, p.current.Start, p.current.End)
	}

	first := p.tree.NewNode(a.NodeName, p.current.Start, p.current.End)
	firstIdx := uint32(len(p.tree.Names))
	p.tree.Names = append(p.tree.Names, p.current.Literal)
	p.tree.Nodes[first].Data = firstIdx
	p.advance()

	if p.current.Type == l.COMMA {
		targets := []a.NodeID{first}
		for p.current.Type == l.COMMA {
			p.advance()
			if p.current.Type != l.NAME {
				p.errorCurrent("expected variable name")
				id := p.tree.NewNode(a.NodeTuple, p.tree.Nodes[first].Start, p.tree.Nodes[targets[len(targets)-1]].End)
				for _, child := range targets {
					p.tree.AddChild(id, child)
				}
				return id
			}

			newTarget := p.tree.NewNode(a.NodeName, p.current.Start, p.current.End)
			idx := uint32(len(p.tree.Names))
			p.tree.Names = append(p.tree.Names, p.current.Literal)
			p.tree.Nodes[newTarget].Data = idx
			targets = append(targets, newTarget)
			p.advance()
		}
		lastName := p.tree.Nodes[targets[len(targets)-1]]
		tuple := p.tree.NewNode(a.NodeTuple, p.tree.Nodes[first].Start, lastName.End)
		for _, child := range targets {
			p.tree.AddChild(tuple, child)
		}
		return tuple
	}

	return first
}

func (p *Parser) parseWhile() a.NodeID {
	startPos := p.current.Start
	p.advance()
	testExpr := p.parseExpression(LOWEST)
	if testExpr == a.NoNode {
		p.errorCurrent("expected valid expression for while condition")
		testExpr = p.tree.NewNode(a.NodeErrExp, p.current.Start, p.current.End)
	}

	if p.current.Type != l.COLON {
		p.errorCurrent("expected ':' after while condition")
		p.syncTo(l.COLON, l.NEWLINE, l.EOF)
		if p.current.Type != l.COLON {
			id := p.tree.NewNode(a.NodeWhile, startPos, p.tree.Nodes[testExpr].End)
			p.tree.AddChild(id, testExpr)
			return id
		}
	}
	p.advance()

	if p.current.Type != l.NEWLINE {
		p.errorCurrent("expected newline after ':'")
		p.syncTo(l.NEWLINE, l.EOF)
		if p.current.Type != l.NEWLINE {
			id := p.tree.NewNode(a.NodeWhile, startPos, p.tree.Nodes[testExpr].End)
			p.tree.AddChild(id, testExpr)
			return id
		}
	}
	p.advance()

	if p.current.Type != l.INDENT {
		p.errorCurrent("expected indent after while:")
		p.syncTo(l.INDENT, l.DEDENT, l.EOF)
		if p.current.Type != l.INDENT {
			id := p.tree.NewNode(a.NodeWhile, startPos, p.tree.Nodes[testExpr].End)
			p.tree.AddChild(id, testExpr)
			return id
		}
	}
	p.advance()

	body := p.tree.NewNode(a.NodeBlock, p.current.Start, 0)
	bodyStmts := []a.NodeID{}

	for p.current.Type != l.DEDENT && p.current.Type != l.EOF {
		stmt := p.parseStatement()
		if stmt != a.NoNode {
			bodyStmts = append(bodyStmts, stmt)
		}
	}
	if len(bodyStmts) > 0 {
		p.tree.Nodes[body].Start = p.tree.Nodes[bodyStmts[0]].Start
		p.tree.Nodes[body].End = uint32(p.tree.Nodes[bodyStmts[len(bodyStmts)-1]].End)
	}
	endPos := p.current.Start

	if p.current.Type == l.DEDENT {
		p.advance()
	}

	ret := p.tree.NewNode(a.NodeWhile, startPos, endPos)
	p.tree.AddChild(ret, testExpr)
	for _, child := range bodyStmts {
		p.tree.AddChild(body, child)
	}
	p.tree.AddChild(ret, body)

	return ret
}

func (p *Parser) parseClass() a.NodeID {
	panic("unimplemented")
}

func (p *Parser) parseReturn() a.NodeID {
	panic("unimplemented")
}

func (p *Parser) parseIf() a.NodeID {
	panic("unimplemented")
}
