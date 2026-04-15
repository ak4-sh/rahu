package parser

import (
	l "rahu/lexer"
	a "rahu/parser/ast"
)

func (p *Parser) parseCall(funcExpr a.NodeID) a.NodeID {
	if funcExpr == a.NoNode {
		return funcExpr
	}
	startPos := p.tree.Nodes[funcExpr].Start
	callID := p.tree.NewNode(a.NodeCall, startPos, startPos)
	p.tree.AddChild(callID, funcExpr)

	p.advance()

	seenKeyword := false
	seenKwStar := false
	if p.current.Type != l.RPAR {
		first := p.parseCallArg(&seenKeyword, &seenKwStar)
		if first == a.NoNode {
			p.syncTo(l.RPAR, l.NEWLINE, l.EOF)
			end := p.current.End
			if p.current.Type == l.RPAR {
				end = p.current.Start
				p.advance()
			}
			p.tree.Nodes[callID].End = end
			return callID
		}

		p.tree.AddChild(callID, first)
		p.tree.Nodes[callID].End = p.tree.Nodes[first].End

		for p.current.Type == l.COMMA {
			p.advance()

			if p.current.Type == l.RPAR {
				break
			}
			arg := p.parseCallArg(&seenKeyword, &seenKwStar)

			if arg == a.NoNode {
				p.syncTo(l.RPAR, l.NEWLINE, l.EOF)
				end := p.current.End
				if p.current.Type == l.RPAR {
					end = p.current.Start
					p.advance()
				}
				p.tree.Nodes[callID].End = end
				return callID
			}
			p.tree.AddChild(callID, arg)
			p.tree.Nodes[callID].End = p.tree.Nodes[arg].End
		}
		if p.current.Type != l.RPAR {
			p.errorCurrent("expected ')' after function arguments")
			p.syncTo(l.RPAR, l.NEWLINE, l.EOF)
			endPos := p.current.End
			if p.current.Type == l.RPAR {
				endPos = p.current.Start
				p.advance()
			}

			p.tree.Nodes[callID].End = endPos
			return callID
		}
	}
	endPos := p.current.Start
	p.advance()

	p.tree.Nodes[callID].End = endPos
	return callID
}

func (p *Parser) parseCallArg(seenKeyword *bool, seenKwStar *bool) a.NodeID {
	if p.current.Type == l.STAR {
		start := p.current.Start
		p.advance()
		value := p.parseExpression(LOWEST)
		if value == a.NoNode {
			p.errorCurrent("expected expression after '*' in call argument")
			return a.NoNode
		}
		arg := p.tree.NewNode(a.NodeStarArg, start, p.tree.Nodes[value].End)
		p.tree.AddChild(arg, value)
		return arg
	}

	if p.current.Type == l.DOUBLESTAR {
		start := p.current.Start
		p.advance()
		value := p.parseExpression(LOWEST)
		if value == a.NoNode {
			p.errorCurrent("expected expression after '**' in call argument")
			return a.NoNode
		}
		arg := p.tree.NewNode(a.NodeKwStarArg, start, p.tree.Nodes[value].End)
		p.tree.AddChild(arg, value)
		*seenKwStar = true
		return arg
	}

	if p.current.Type == l.NAME && p.peek.Type == l.EQUAL {
		keyword := p.tree.NewNameNode(p.current.Start, p.current.End, p.current.Literal)
		start := p.current.Start
		p.advanceBy(2)

		value := p.parseExpression(LOWEST)
		if value == a.NoNode {
			p.errorCurrent("expected expression after '=' in keyword argument")
			return a.NoNode
		}

		arg := p.tree.NewNode(a.NodeKeywordArg, start, p.tree.Nodes[value].End)
		p.tree.AddChild(arg, keyword)
		p.tree.AddChild(arg, value)
		*seenKeyword = true
		return arg
	}

	arg := p.parseExpression(LOWEST)
	if arg == a.NoNode {
		return a.NoNode
	}
	if *seenKwStar {
		p.error(a.Range{Start: p.tree.Nodes[arg].Start, End: p.tree.Nodes[arg].End}, "positional argument follows **kwargs")
		return arg
	}
	if *seenKeyword {
		p.error(a.Range{Start: p.tree.Nodes[arg].Start, End: p.tree.Nodes[arg].End}, "positional argument follows keyword argument")
	}
	return arg
}
