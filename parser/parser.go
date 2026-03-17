package parser

import (
	l "rahu/lexer"
	a "rahu/parser/ast"
)

// function parseStatement walks a module and parses all statements.
// parseStatement acts as a dispatcher to smaller helper functions to
// parse other node types
func (p *Parser) parseStatement() a.NodeID {
	if p.current.Type == l.UNTERMINATED_STRING {
		p.error(a.Range{Start: p.current.Start, End: p.current.End}, "unterminated string literal")
		p.advance()
		return a.NoNode
	}

	// consume newlines
	for p.current.Type == l.NEWLINE {
		p.advance()
	}

	switch p.current.Type {
	case l.IF:
		return p.parseIf()

	case l.NAME, l.NUMBER, l.STRING, l.LPAR, l.LSQB, l.MINUS, l.PLUS, l.NOT, l.TRUE, l.FALSE, l.NONE:
		return p.dispatchExprParse()

	case l.DEF:
		return p.parseFunc()

	case l.BREAK:
		start := p.current.Start
		end := p.current.End
		p.advance()
		if p.current.Type == l.NEWLINE {
			p.advance()
		}
		return p.tree.NewNode(a.NodeBreak, start, end)

	case l.CONTINUE:
		start := p.current.Start
		end := p.current.End
		p.advance()
		if p.current.Type == l.NEWLINE {
			p.advance()
		}
		return p.tree.NewNode(a.NodeContinue, start, end)

	case l.RETURN:
		return p.parseReturn()

	case l.FOR:
		return p.parseFor()
	case l.WHILE:
		return p.parseWhile()

	case l.CLASS:
		return p.parseClass()
	default:
		p.error(a.Range{Start: p.current.Start, End: p.current.End}, "unexpected token: "+p.current.String())
		p.advance()
		return a.NoNode
	}
}

func (p *Parser) dispatchExprParse() a.NodeID {
	expr := p.parseExpression(LOWEST)
	if expr == a.NoNode {
		p.errorCurrent("expected expression")
		return expr
	}

	start := p.tree.Nodes[expr].Start

	if p.current.Type == l.EQUAL || p.current.Type == l.COMMA {
		switch p.tree.Nodes[expr].Kind {
		case a.NodeName, a.NodeAttribute, a.NodeTuple, a.NodeList:
			return p.parseAssignmentFromFirst(start, expr)
		}
	}

	if isAugAssignOp(p.current.Type) {
		switch p.tree.Nodes[expr].Kind {
		case a.NodeName, a.NodeAttribute:
			return p.parseAugAssignFromFirst(start, expr)
		}
	}

	if p.current.Type == l.NEWLINE {
		p.advance()
	} else if p.current.Type != l.EOF {
		p.error(a.Range{
			Start: p.current.Start,
			End:   p.current.End,
		}, "expected newline after expression")

		p.syncTo(l.NEWLINE, l.EOF)
		if p.current.Type == l.NEWLINE {
			p.advance()
		}
	}

	exprNode := p.tree.Nodes[expr]
	exprID := p.tree.NewNode(a.NodeExprStmt, exprNode.Start, exprNode.End)
	p.tree.AddChild(exprID, expr)
	return exprID
}

func isAugAssignOp(t l.TokenType) bool {
	return t == l.PLUSEQUAL ||
		t == l.MINEQUAL ||
		t == l.SLASHEQUAL ||
		t == l.STAREQUAL ||
		t == l.DOUBLESLASHEQUAL ||
		t == l.DOUBLESTAREQUAL ||
		t == l.AMPEREQUAL ||
		t == l.LEFTSHIFTEQUAL ||
		t == l.RIGHTSHIFTEQUAL ||
		t == l.ATEQUAL ||
		t == l.VBAREQUAL ||
		t == l.PERCENTEQUAL ||
		t == l.CIRCUMFLEXEQUAL
}

func (p *Parser) parseAugAssignFromFirst(start uint32, expr a.NodeID) a.NodeID {
	op := p.current.Type
	opTok := p.current
	p.advance()
	value := p.parseExpression(LOWEST)
	if value == a.NoNode {
		p.errorCurrent("expected expression after augmented assign operator")
		value = p.tree.NewNode(a.NodeErrExp, p.current.Start, p.current.Start)
	}
	end := p.current.Start
	if p.current.Type == l.NEWLINE {
		p.advance()
	} else if p.current.Type != l.EOF {
		p.error(a.Range{Start: p.current.Start, End: p.current.End}, "expected newline after augmented assignment")
		p.syncTo(l.NEWLINE, l.EOF)
		if p.current.Type == l.NEWLINE {
			p.advance()
		}

	}
	ret := p.tree.NewNode(a.NodeAugAssign, start, end)
	p.tree.AddChild(ret, expr)
	p.tree.AddChild(ret, value)
	switch op {
	case l.PLUSEQUAL:
		p.tree.Nodes[ret].Data = uint32(a.AugAdd)

	case l.MINEQUAL:
		p.tree.Nodes[ret].Data = uint32(a.AugSub)

	case l.STAREQUAL:
		p.tree.Nodes[ret].Data = uint32(a.AugMul)

	case l.SLASHEQUAL:
		p.tree.Nodes[ret].Data = uint32(a.AugDiv)

	case l.DOUBLESLASHEQUAL:
		p.tree.Nodes[ret].Data = uint32(a.AugFloorDiv)

	case l.DOUBLESTAREQUAL:
		p.tree.Nodes[ret].Data = uint32(a.AugPow)

	case l.AMPEREQUAL:
		p.tree.Nodes[ret].Data = uint32(a.AugAnd)

	case l.LEFTSHIFTEQUAL:
		p.tree.Nodes[ret].Data = uint32(a.AugLShift)

	case l.RIGHTSHIFTEQUAL:
		p.tree.Nodes[ret].Data = uint32(a.AugRShift)

	case l.ATEQUAL:
		p.tree.Nodes[ret].Data = uint32(a.AugMatMul)

	case l.VBAREQUAL:
		p.tree.Nodes[ret].Data = uint32(a.AugOr)

	case l.PERCENTEQUAL:
		p.tree.Nodes[ret].Data = uint32(a.AugMod)

	case l.CIRCUMFLEXEQUAL:
		p.tree.Nodes[ret].Data = uint32(a.AugXor)
	default:
		p.error(a.Range{Start: opTok.Start, End: opTok.End}, "invalid augmented assignment operator")
		p.tree.Nodes[ret].Data = uint32(a.AugInvalid)

	}
	return ret
}

func (p *Parser) parseAssignmentFromFirst(start uint32, first a.NodeID) a.NodeID {
	targets := []a.NodeID{first}
	for p.current.Type == l.COMMA {
		p.advance()
		t := p.parseExpression(LOWEST)
		if t == a.NoNode {
			p.errorCurrent("expected assignment target")
			break
		}
		targets = append(targets, t)
	}

	if p.current.Type != l.EQUAL {
		p.error(a.Range{Start: start, End: p.current.Start}, "expected '=' in assignment")
		p.syncTo(l.EOF, l.NEWLINE)
		val := p.tree.NewNode(a.NodeErrExp, p.current.Start, p.current.Start)
		ret := p.tree.NewNode(a.NodeAssign, start, p.current.Start)
		p.tree.AddChild(ret, val)
		for _, child := range targets {
			p.tree.AddChild(ret, child)
		}

		if p.current.Type == l.NEWLINE {
			p.advance()
		}
		return ret
	}
	p.advance()

	value := p.parseExpression(LOWEST)
	end := p.current.Start
	if value == a.NoNode {
		p.errorCurrent("expected expression after '='")
		value = p.tree.NewNode(a.NodeErrExp, p.current.Start, p.current.Start)
	}

	if p.current.Type == l.NEWLINE {
		p.advance()
	} else if p.current.Type != l.EOF {
		p.error(a.Range{Start: p.current.Start, End: p.current.End}, "expected newline after assignment")
		p.syncTo(l.NEWLINE, l.EOF)
		if p.current.Type == l.NEWLINE {
			p.advance()
		}
	}
	ret := p.tree.NewNode(a.NodeAssign, start, end)

	p.tree.AddChild(ret, value)
	for _, child := range targets {
		p.tree.AddChild(ret, child)
	}
	return ret
}
