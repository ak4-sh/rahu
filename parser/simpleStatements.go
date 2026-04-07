package parser

import (
	l "rahu/lexer"
	a "rahu/parser/ast"
)

func (p *Parser) finishSimpleStatementWithMessage(start uint32, end uint32, msg string) uint32 {
	if p.current.Type == l.NEWLINE {
		end = p.current.Start
		p.advance()
		return end
	}
	if p.current.Type != l.EOF {
		p.errorCurrent(msg)
		p.syncTo(l.NEWLINE, l.EOF)
		if p.current.Type == l.NEWLINE {
			end = p.current.Start
			p.advance()
		}
	}
	return end
}

func (p *Parser) parseAssert() a.NodeID {
	start := p.current.Start
	p.advance()
	ret := p.tree.NewNode(a.NodeAssert, start, start)

	test := p.parseExpression(LOWEST)
	if test == a.NoNode {
		p.errorCurrent("expected expression after 'assert'")
		p.tree.Nodes[ret].End = p.finishSimpleStatementWithMessage(start, p.current.Start, "expected newline after assert statement")
		return ret
	}
	p.tree.AddChild(ret, test)
	end := p.tree.Nodes[test].End

	if p.current.Type == l.COMMA {
		p.advance()
		msgExpr := p.parseExpression(LOWEST)
		if msgExpr == a.NoNode {
			p.errorCurrent("expected assertion message after ','")
		} else {
			p.tree.AddChild(ret, msgExpr)
			end = p.tree.Nodes[msgExpr].End
		}
	}

	p.tree.Nodes[ret].End = p.finishSimpleStatementWithMessage(start, end, "expected newline after assert statement")
	return ret
}

func isDeleteTarget(tree *a.AST, id a.NodeID) bool {
	if tree == nil || id == a.NoNode {
		return false
	}
	switch tree.Node(id).Kind {
	case a.NodeName, a.NodeAttribute, a.NodeSubScript:
		return true
	case a.NodeTuple, a.NodeList:
		for child := tree.Node(id).FirstChild; child != a.NoNode; child = tree.Node(child).NextSibling {
			if !isDeleteTarget(tree, child) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func (p *Parser) parseDel() a.NodeID {
	start := p.current.Start
	p.advance()
	ret := p.tree.NewNode(a.NodeDel, start, start)

	for {
		target := p.parseExpression(LOWEST)
		if target == a.NoNode {
			if p.tree.Nodes[ret].FirstChild == a.NoNode {
				p.errorCurrent("expected delete target after 'del'")
			} else {
				p.errorCurrent("expected delete target after ','")
			}
			break
		}
		if !isDeleteTarget(p.tree, target) {
			p.error(p.tree.RangeOf(target), "invalid delete target")
		}
		p.tree.AddChild(ret, target)
		p.tree.Nodes[ret].End = p.tree.Nodes[target].End
		if p.current.Type != l.COMMA {
			break
		}
		p.advance()
	}

	p.tree.Nodes[ret].End = p.finishSimpleStatementWithMessage(start, p.tree.Nodes[ret].End, "expected newline after del statement")
	return ret
}

func (p *Parser) parseNameListStatement(kind a.NodeKind, keyword, missingMsg string) a.NodeID {
	start := p.current.Start
	p.advance()
	ret := p.tree.NewNode(kind, start, start)

	for {
		if p.current.Type != l.NAME {
			if p.tree.Nodes[ret].FirstChild == a.NoNode {
				p.errorCurrent(missingMsg)
			} else {
				p.errorCurrent("expected name after ','")
			}
			break
		}
		name := p.tree.NewNameNode(p.current.Start, p.current.End, p.current.Literal)
		p.tree.AddChild(ret, name)
		p.tree.Nodes[ret].End = p.current.End
		p.advance()
		if p.current.Type != l.COMMA {
			break
		}
		p.advance()
	}

	p.tree.Nodes[ret].End = p.finishSimpleStatementWithMessage(start, p.tree.Nodes[ret].End, "expected newline after "+keyword+" statement")
	return ret
}

func (p *Parser) parseGlobal() a.NodeID {
	return p.parseNameListStatement(a.NodeGlobal, "global", "expected name after 'global'")
}

func (p *Parser) parseNonlocal() a.NodeID {
	return p.parseNameListStatement(a.NodeNonlocal, "nonlocal", "expected name after 'nonlocal'")
}
