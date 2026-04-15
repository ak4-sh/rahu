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
	case l.ASSERT:
		return p.parseAssert()
	case l.DEL:
		return p.parseDel()
	case l.GLOBAL:
		return p.parseGlobal()
	case l.NONLOCAL:
		return p.parseNonlocal()

	case l.NAME, l.NUMBER, l.STRING, l.FSTRING, l.LPAR, l.LSQB, l.LBRACE, l.MINUS, l.PLUS, l.NOT, l.TRUE, l.FALSE, l.NONE, l.YIELD:
		return p.dispatchExprParse()

	case l.DEF:
		return p.parseFunc()

	case l.IMPORT:
		return p.parseImport()

	case l.FROM:
		return p.parseFromImport()

	case l.BREAK:
		start := p.current.Start
		end := p.current.End
		p.advance()
		if p.current.Type == l.NEWLINE {
			p.advance()
		}
		return p.tree.NewNode(a.NodeBreak, start, end)

	case l.PASS:
		start := p.current.Start
		end := p.current.End
		p.advance()
		if p.current.Type == l.NEWLINE {
			p.advance()
		}
		return p.tree.NewNode(a.NodePass, start, end)

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
	case l.RAISE:
		return p.parseRaise()

	case l.FOR:
		return p.parseFor()
	case l.WHILE:
		return p.parseWhile()

	case l.CLASS:
		return p.parseClass()
	case l.TRY:
		return p.parseTry()
	case l.WITH:
		return p.parseWith()
	case l.AT:
		return p.parseDecoratedDef()
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

	if p.current.Type == l.COLON {
		switch p.tree.Nodes[expr].Kind {
		case a.NodeName:
			return p.parseAnnotatedAssignment(start, expr)
		}
	}

	if p.current.Type == l.EQUAL || p.current.Type == l.COMMA {
		switch p.tree.Nodes[expr].Kind {
		case a.NodeName, a.NodeAttribute, a.NodeTuple, a.NodeList, a.NodeSubScript:
			return p.parseAssignmentFromFirst(start, expr)
		}
	}

	if isAugAssignOp(p.current.Type) {
		switch p.tree.Nodes[expr].Kind {
		case a.NodeName, a.NodeAttribute, a.NodeSubScript:
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

func (p *Parser) parseAnnotatedAssignment(start uint32, target a.NodeID) a.NodeID {
	p.advance()

	annotation := p.parseExpression(LOWEST)
	if annotation == a.NoNode {
		p.errorCurrent("expected type annotation after ':'")
		annotation = p.tree.NewNode(a.NodeErrExp, p.current.Start, p.current.Start)
	}

	value := a.NoNode
	end := p.tree.Nodes[annotation].End
	if p.current.Type == l.EQUAL {
		p.advance()
		value = p.parseExpression(LOWEST)
		if value == a.NoNode {
			p.errorCurrent("expected expression after '='")
			value = p.tree.NewNode(a.NodeErrExp, p.current.Start, p.current.Start)
		}
		end = p.tree.Nodes[value].End
	}

	if p.current.Type == l.NEWLINE {
		end = p.current.Start
		p.advance()
	} else if p.current.Type != l.EOF {
		p.error(a.Range{Start: p.current.Start, End: p.current.End}, "expected newline after annotated assignment")
		p.syncTo(l.NEWLINE, l.EOF)
		if p.current.Type == l.NEWLINE {
			end = p.current.Start
			p.advance()
		}
	}

	ret := p.tree.NewNode(a.NodeAnnAssign, start, end)
	p.tree.AddChild(ret, target)
	p.tree.AddChild(ret, annotation)
	if value != a.NoNode {
		p.tree.AddChild(ret, value)
	}
	return ret
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
	lastTarget := first
	targetCount := 1
	for p.current.Type == l.COMMA {
		p.advance()
		t := p.parseExpression(LOWEST)
		if t == a.NoNode {
			p.errorCurrent("expected assignment target")
			break
		}
		p.tree.Nodes[lastTarget].NextSibling = t
		lastTarget = t
		targetCount++
	}

	if p.current.Type != l.EQUAL {
		p.error(a.Range{Start: start, End: p.current.Start}, "expected '=' in assignment")
		p.syncTo(l.EOF, l.NEWLINE)
		val := p.tree.NewNode(a.NodeErrExp, p.current.Start, p.current.Start)
		ret := p.tree.NewNode(a.NodeAssign, start, p.current.Start)
		p.tree.AddChild(ret, val)
		for i, child := 0, first; i < targetCount; i++ {
			next := p.tree.Nodes[child].NextSibling
			p.tree.AddChild(ret, child)
			child = next
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

	// Handle chained assignment: a = b = c (where b = c happens first, then a = result)
	// The "value" we just parsed is actually another target if followed by '='
	for p.current.Type == l.EQUAL {
		// Add the first value as another assignment target
		p.tree.Nodes[lastTarget].NextSibling = value
		lastTarget = value
		targetCount++

		// Parse the actual value after the second '='
		p.advance()
		value = p.parseExpression(LOWEST)
		end = p.current.Start
		if value == a.NoNode {
			p.errorCurrent("expected expression after '='")
			value = p.tree.NewNode(a.NodeErrExp, p.current.Start, p.current.Start)
		}
	}

	// Handle tuple unpacking: a, b = 1, 2
	if value != a.NoNode && p.current.Type == l.COMMA {
		tuple := p.tree.NewNode(a.NodeTuple, p.tree.Nodes[value].Start, p.tree.Nodes[value].End)
		p.tree.AddChild(tuple, value)
		for p.current.Type == l.COMMA {
			p.advance()
			if p.current.Type == l.NEWLINE || p.current.Type == l.EOF {
				break
			}
			elt := p.parseExpression(LOWEST)
			if elt == a.NoNode {
				p.errorCurrent("expected expression after ',' in assignment value")
				break
			}
			p.tree.AddChild(tuple, elt)
			p.tree.Nodes[tuple].End = p.tree.Nodes[elt].End
		}
		value = tuple
		end = p.tree.Nodes[value].End
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
	for i, child := 0, first; i < targetCount; i++ {
		next := p.tree.Nodes[child].NextSibling
		p.tree.AddChild(ret, child)
		child = next
	}
	return ret
}
