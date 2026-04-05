package parser

import (
	l "rahu/lexer"
	a "rahu/parser/ast"
)

func (p *Parser) parseSubscript(left a.NodeID) a.NodeID {
	if left == a.NoNode {
		return left
	}

	start := p.tree.Nodes[left].Start
	p.advance() // consume '['

	ret := p.tree.NewNode(a.NodeSubScript, start, start)
	p.tree.AddChild(ret, left)

	if p.current.Type == l.COLON {
		slice := p.parseSlice(a.NoNode)
		p.tree.AddChild(ret, slice)
		p.tree.Nodes[ret].End = p.tree.Nodes[slice].End
		return ret
	}

	index := p.parseExpression(LOWEST)
	if index == a.NoNode {
		p.errorCurrent("expected expression in subscript")
		p.syncTo(l.RSQB, l.NEWLINE, l.EOF)
		end := p.current.Start
		if p.current.Type == l.RSQB {
			end = p.current.Start
			p.advance()
		}
		p.tree.Nodes[ret].End = end
		return ret
	}

	if p.current.Type == l.COMMA {
		tuple := p.tree.NewNode(a.NodeTuple, p.tree.Nodes[index].Start, p.tree.Nodes[index].End)
		p.tree.AddChild(tuple, index)
		for p.current.Type == l.COMMA {
			p.advance()
			if p.current.Type == l.RSQB {
				break
			}
			item := p.parseExpression(LOWEST)
			if item == a.NoNode {
				p.errorCurrent("expected expression after ',' in subscript")
				break
			}
			p.tree.AddChild(tuple, item)
			p.tree.Nodes[tuple].End = p.tree.Nodes[item].End
		}
		index = tuple
	}

	if p.current.Type == l.COLON {
		slice := p.parseSlice(index)
		p.tree.AddChild(ret, slice)
		p.tree.Nodes[ret].End = p.tree.Nodes[slice].End
		return ret
	}

	p.tree.AddChild(ret, index)
	p.tree.Nodes[ret].End = p.tree.Nodes[index].End

	if p.current.Type != l.RSQB {
		p.errorCurrent("expected ']' after subscript")
		p.syncTo(l.RSQB, l.NEWLINE, l.EOF)
		end := p.current.Start
		if p.current.Type == l.RSQB {
			end = p.current.Start
			p.advance()
		}
		p.tree.Nodes[ret].End = end
		return ret
	}

	end := p.current.Start
	p.advance()
	p.tree.Nodes[ret].End = end
	return ret
}

func (p *Parser) parseSlice(startExpr a.NodeID) a.NodeID {
	start := p.current.Start
	slice := p.tree.NewNode(a.NodeSlice, start, start)
	if startExpr != a.NoNode {
		p.tree.AddChild(slice, startExpr)
		p.tree.Nodes[slice].Start = p.tree.Nodes[startExpr].Start
		p.tree.Nodes[slice].End = p.tree.Nodes[startExpr].End
	}

	p.advance() // consume ':'

	if p.current.Type == l.COLON {
		p.errorCurrent("slice step not supported")
		p.syncTo(l.RSQB, l.NEWLINE, l.EOF)
		end := p.current.Start
		if p.current.Type == l.RSQB {
			end = p.current.Start
			p.advance()
		}
		p.tree.Nodes[slice].End = end
		return slice
	}

	if p.current.Type != l.RSQB {
		endExpr := p.parseExpression(LOWEST)
		if endExpr == a.NoNode {
			p.errorCurrent("expected expression after ':' in slice")
			p.syncTo(l.RSQB, l.NEWLINE, l.EOF)
			end := p.current.Start
			if p.current.Type == l.RSQB {
				end = p.current.Start
				p.advance()
			}
			p.tree.Nodes[slice].End = end
			return slice
		}
		p.tree.AddChild(slice, endExpr)
		p.tree.Nodes[slice].End = p.tree.Nodes[endExpr].End

		if p.current.Type == l.COLON {
			p.errorCurrent("slice step not supported")
			p.syncTo(l.RSQB, l.NEWLINE, l.EOF)
			end := p.current.Start
			if p.current.Type == l.RSQB {
				end = p.current.Start
				p.advance()
			}
			p.tree.Nodes[slice].End = end
			return slice
		}
	}

	if p.current != (l.Token{}) && p.current.Type != l.RSQB {
		p.errorCurrent("expected ']' after slice")
		p.syncTo(l.RSQB, l.NEWLINE, l.EOF)
		end := p.current.Start
		if p.current.Type == l.RSQB {
			end = p.current.Start
			p.advance()
		}
		p.tree.Nodes[slice].End = end
		return slice
	}

	end := p.current.Start
	p.advance()
	p.tree.Nodes[slice].End = end
	return slice
}
