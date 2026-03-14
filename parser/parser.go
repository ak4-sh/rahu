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

func (p *Parser) parseFor() a.NodeID {
	panic("unimplemented")
}

func (p *Parser) parseClass() a.NodeID {
	panic("unimplemented")
}

func (p *Parser) parseWhile() a.NodeID {
	panic("unimplemented")
}

func (p *Parser) parseReturn() a.NodeID {
	panic("unimplemented")
}

func (p *Parser) parseIf() a.NodeID {
	panic("unimplemented")
}

func (p *Parser) dispatchExprParse() a.NodeID {
	var start uint32
	expr := p.parseExpression(LOWEST)
	if expr == a.NoNode {
		p.errorCurrent("expected expression")
		return expr
	}

	if p.current.Type == l.EQUAL || p.current.Type == l.COMMA {
		switch p.tree.Nodes[expr].Kind {
		case a.NodeName, a.NodeAttribute, a.NodeTuple, a.NodeList:
			return p.parseAssignmentFromFirst(start, expr)
		}
	}

	if p.current.Type == l.PLUSEQUAL || p.current.Type == l.MINEQUAL || p.current.Type == l.SLASHEQUAL || p.current.Type == l.STAREQUAL || p.current.Type == l.DOUBLESLASHEQUAL || p.current.Type == l.DOUBLESTAREQUAL || p.current.Type == l.AMPEREQUAL || p.current.Type == l.LEFTSHIFTEQUAL || p.current.Type == l.RIGHTSHIFTEQUAL {
		switch p.tree.Nodes[expr].Kind {
		case a.NodeName, a.NodeAttribute:
			return p.parseAugAssignFromFirst(start, expr)
		}
	}

	if p.current.Type == l.NEWLINE {
		p.advance()
	} else if p.current.Type != l.EOF {
		p.error(a.Range{
			Start: p.tree.Nodes[expr].Start,
			End:   p.tree.Nodes[expr].End,
		},
			"expected newline after expression",
		)
	}
	exprNode := p.tree.Nodes[expr]
	exprID := p.tree.NewNode(a.NodeExprStmt, exprNode.Start, exprNode.End)
	p.tree.AddChild(exprID, expr)
	return exprID
}

func (p *Parser) parseAugAssignFromFirst(start uint32, expr a.NodeID) a.NodeID {
	panic("unimplemented")
}

func (p *Parser) parseAssignmentFromFirst(start uint32, expr a.NodeID) a.NodeID {
	panic("unimplemented")
}
