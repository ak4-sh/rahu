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

	// Check for generator expression without parentheses: func(x for x in items)
	// This is valid when the generator is the only argument
	if p.current.Type == l.FOR {
		// Convert the expression into a generator expression
		startPos := p.tree.Nodes[arg].Start
		genExp := p.parseGeneratorExpFromExpr(startPos, arg)
		return genExp
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

// parseGeneratorExpFromExpr parses a generator expression starting from an already-parsed expression.
// Used for bare generator expressions in function calls (e.g., sum(x for x in items)).
func (p *Parser) parseGeneratorExpFromExpr(startPos uint32, expr a.NodeID) a.NodeID {
	ret := p.tree.NewNode(a.NodeGeneratorExp, startPos, p.tree.Nodes[expr].End)
	p.tree.AddChild(ret, expr)
	for p.current.Type == l.FOR {
		clause := p.parseComprehensionClause()
		if clause == a.NoNode {
			return p.tree.NewNode(a.NodeErrExp, startPos, p.current.End)
		}
		p.tree.AddChild(ret, clause)
		p.tree.Nodes[ret].End = p.tree.Nodes[clause].End
	}
	return ret
}
