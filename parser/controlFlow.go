package parser

import (
	"strings"

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

	bodyBlock := p.tree.NewNode(a.NodeBlock, p.current.Start, p.current.Start)
	for p.current.Type != l.DEDENT && p.current.Type != l.EOF {
		stmt := p.parseStatement()
		if stmt != a.NoNode {
			p.tree.AddChild(bodyBlock, stmt)
			if p.tree.Nodes[bodyBlock].FirstChild == stmt {
				p.tree.Nodes[bodyBlock].Start = p.tree.Nodes[stmt].Start
			}
			p.tree.Nodes[bodyBlock].End = p.tree.Nodes[stmt].End
		}
	}
	p.tree.AddChild(ret, bodyBlock)

	endPos := p.current.Start
	if p.current.Type == l.DEDENT {
		p.advance()
	}

	if p.current.Type == l.ELSE {
		elseBlock := a.NoNode
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
		elseBlock = p.tree.NewNode(a.NodeBlock, p.current.Start, p.current.Start)

		for p.current.Type != l.DEDENT && p.current.Type != l.EOF {
			stmt := p.parseStatement()
			if stmt != a.NoNode {
				p.tree.AddChild(elseBlock, stmt)
				if p.tree.Nodes[elseBlock].FirstChild == stmt {
					p.tree.Nodes[elseBlock].Start = p.tree.Nodes[stmt].Start
				}
				p.tree.Nodes[elseBlock].End = p.tree.Nodes[stmt].End
			}
		}
		if p.tree.Nodes[elseBlock].End == p.tree.Nodes[elseBlock].Start {
			p.tree.Nodes[elseBlock].End = p.current.Start
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

	first := p.tree.NewNameNode(p.current.Start, p.current.End, p.current.Literal)
	p.advance()

	if p.current.Type == l.COMMA {
		tuple := p.tree.NewNode(a.NodeTuple, p.tree.Nodes[first].Start, p.tree.Nodes[first].End)
		p.tree.AddChild(tuple, first)
		for p.current.Type == l.COMMA {
			p.advance()
			if p.current.Type != l.NAME {
				p.errorCurrent("expected variable name")
				return tuple
			}

			newTarget := p.tree.NewNameNode(p.current.Start, p.current.End, p.current.Literal)
			p.tree.AddChild(tuple, newTarget)
			p.tree.Nodes[tuple].End = p.tree.Nodes[newTarget].End
			p.advance()
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

	for p.current.Type != l.DEDENT && p.current.Type != l.EOF {
		stmt := p.parseStatement()
		if stmt != a.NoNode {
			p.tree.AddChild(body, stmt)
			if p.tree.Nodes[body].FirstChild == stmt {
				p.tree.Nodes[body].Start = p.tree.Nodes[stmt].Start
			}
			p.tree.Nodes[body].End = uint32(p.tree.Nodes[stmt].End)
		}
	}
	endPos := p.current.Start

	if p.current.Type == l.DEDENT {
		p.advance()
	}

	ret := p.tree.NewNode(a.NodeWhile, startPos, endPos)
	p.tree.AddChild(ret, testExpr)
	p.tree.AddChild(ret, body)

	return ret
}

func (p *Parser) parseClassBases() a.NodeID {
	start := p.current.Start // at '('
	bases := p.tree.NewNode(a.NodeBaseList, start, start)
	p.advance() // consume '('

	for p.current.Type != l.RPAR && p.current.Type != l.EOF {
		expr := p.parseExpression(LOWEST)
		if expr == a.NoNode {
			p.errorCurrent("expected expression in class base list")
			p.syncTo(l.COMMA, l.RPAR, l.COLON, l.EOF)
		} else {
			p.tree.AddChild(bases, expr)
		}

		if p.current.Type == l.COMMA {
			p.advance()
			continue
		}
		break
	}

	if p.current.Type != l.RPAR {
		p.errorCurrent("expected ')' after class base list")
	} else {
		p.tree.Nodes[bases].End = p.current.End
		p.advance()
	}

	return bases
}

func (p *Parser) parseClass() a.NodeID {
	startPos := p.current.Start
	p.advance()

	if p.current.Type != l.NAME {
		p.errorCurrent("expected classname after `class`")
		p.syncTo(l.NEWLINE, l.COLON, l.EOF)
		name := p.tree.NewNameNode(startPos, p.current.End, "<incomplete>")
		class := p.tree.NewNode(a.NodeClassDef, startPos, p.current.End)
		p.tree.AddChild(class, name)
		return class
	}

	className := p.tree.NewNameNode(p.current.Start, p.current.End, p.current.Literal)

	p.advance()

	if p.current.Type != l.COLON && (p.current.Type != l.LPAR) {
		p.errorCurrent("expected `(` or `:` after class name")
		p.syncTo(l.NEWLINE, l.COLON, l.EOF)
		ret := p.tree.NewNode(a.NodeClassDef, startPos, p.current.End)
		p.tree.AddChild(ret, className)
		return ret
	}

	bases := a.NoNode

	if p.current.Type == l.LPAR {
		bases = p.parseClassBases()
	}

	if p.current.Type != l.COLON {
		p.errorCurrent("expected `:` after class header")
		p.syncTo(l.COLON, l.EOF, l.RPAR)
		if p.current.Type != l.COLON {
			def := p.tree.NewNode(a.NodeClassDef, startPos, p.current.End)
			p.tree.AddChild(def, className)
			if bases != a.NoNode {
				p.tree.AddChild(def, bases)
			}
			return def
		}
	}
	p.advance()

	if p.current.Type != l.NEWLINE {
		p.errorCurrent("expected newline after `:`")
		p.syncTo(l.EOF, l.NEWLINE)
		if p.current.Type != l.NEWLINE {
			def := p.tree.NewNode(a.NodeClassDef, startPos, p.current.End)
			p.tree.AddChild(def, className)
			if bases != a.NoNode {
				p.tree.AddChild(def, bases)
			}
			return def
		}
	}
	p.advance()
	for p.current.Type == l.NEWLINE {
		p.advance()
	}

	if p.current.Type != l.INDENT {
		p.errorCurrent("expected indent after class definition")
		p.syncTo(l.INDENT, l.DEDENT, l.EOF)
		if p.current.Type != l.INDENT {
			def := p.tree.NewNode(a.NodeClassDef, startPos, p.current.End)
			p.tree.AddChild(def, className)
			if bases != a.NoNode {
				p.tree.AddChild(def, bases)
			}
			return def
		}
	}
	p.advance()

	bodyStmts := p.tree.NewNode(a.NodeBlock, p.current.Start, p.current.Start)
	docString := ""
	atStart := true

	for p.current.Type != l.EOF && p.current.Type != l.DEDENT {
		if p.current.Type == l.NEWLINE {
			p.advance()
			continue
		}
		if atStart && p.current.Type == l.STRING {
			docString = strings.TrimSpace(p.current.Literal)
			p.advance()
			atStart = false
			continue
		}

		atStart = false
		stmt := p.parseStatement()
		if stmt != a.NoNode {
			p.tree.AddChild(bodyStmts, stmt)
			p.tree.Nodes[bodyStmts].End = p.tree.Nodes[stmt].End

		}
	}

	endPos := p.current.End
	if p.current.Type == l.DEDENT {
		endPos = p.current.End
		p.advance()
	}

	if p.tree.Nodes[bodyStmts].End == p.tree.Nodes[bodyStmts].Start {
		p.tree.Nodes[bodyStmts].End = endPos
	}

	def := p.tree.NewNode(a.NodeClassDef, startPos, endPos)
	p.tree.AddChild(def, className)
	if bases != a.NoNode {
		p.tree.AddChild(def, bases)
	}
	if docString != "" {
		idx := uint32(len(p.tree.Strings))
		p.tree.Strings = append(p.tree.Strings, docString)
		p.tree.Nodes[def].Data = idx
	}

	p.tree.AddChild(def, bodyStmts)
	return def
}

func (p *Parser) parseReturn() a.NodeID {
	startPos := p.current.Start
	p.advance()
	if p.current.Type == l.NEWLINE || p.current.Type == l.EOF {
		endPos := p.current.Start
		if p.current.Type == l.NEWLINE {
			p.advance()
		}
		return p.tree.NewNode(a.NodeReturn, startPos, endPos)
	}

	value := p.parseExpression(LOWEST)
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
				p.errorCurrent("expected expression after ',' in return value")
				break
			}
			p.tree.AddChild(tuple, elt)
			p.tree.Nodes[tuple].End = p.tree.Nodes[elt].End
		}
		value = tuple
	}
	endPos := p.current.Start
	if p.current.Type == l.NEWLINE {
		p.advance()
	}

	ret := p.tree.NewNode(a.NodeReturn, startPos, endPos)
	if value != a.NoNode {
		p.tree.AddChild(ret, value)
	}
	return ret
}

func (p *Parser) parseIf() a.NodeID {
	startPos := p.current.Start
	p.advance()

	testCond := p.parseExpression(LOWEST)
	if testCond == a.NoNode {
		p.errorCurrent("invalid test condition for if")
		return a.NoNode
	}

	if p.current.Type != l.COLON {
		p.errorCurrent("expected `:` after if condition")
		p.syncTo(l.COLON, l.NEWLINE, l.EOF)
		if p.current.Type == l.COLON {
			p.advance()
		}

		ret := p.tree.NewNode(a.NodeIf, startPos, p.current.End)
		p.tree.AddChild(ret, testCond)
		return ret
	}

	p.advance()

	if p.current.Type != l.NEWLINE {
		p.errorCurrent("expected newline after `:`")
		p.syncTo(l.NEWLINE, l.EOF)
		if p.current.Type == l.NEWLINE {
			p.advance()
		} else {
			ret := p.tree.NewNode(a.NodeIf, startPos, p.current.End)
			p.tree.AddChild(ret, testCond)
			return ret
		}
	} else {
		p.advance()
	}

	if p.current.Type != l.INDENT {
		p.errorCurrent("expected indentation block after if condition")
		p.syncTo(l.INDENT, l.DEDENT, l.EOF)
		if p.current.Type != l.INDENT {
			ret := p.tree.NewNode(a.NodeIf, startPos, p.current.End)
			p.tree.AddChild(ret, testCond)
			return ret
		}
	}

	p.advance()

	body := p.tree.NewNode(a.NodeBlock, p.current.Start, p.current.Start)

	for p.current.Type != l.DEDENT && p.current.Type != l.EOF {
		if p.current.Type == l.NEWLINE {
			p.advance()
			continue
		}
		stmt := p.parseStatement()
		if stmt != a.NoNode {
			p.tree.AddChild(body, stmt)
			if p.tree.Nodes[body].FirstChild == stmt {
				p.tree.Nodes[body].Start = p.tree.Nodes[stmt].Start
			}
			p.tree.Nodes[body].End = p.tree.Nodes[stmt].End
		}
	}
	if p.tree.Nodes[body].End == p.tree.Nodes[body].Start {
		body = a.NoNode
	}

	endPos := p.current.Start

	if p.current.Type == l.DEDENT {
		endPos = p.current.Start
		p.advance()
	}

	orElseBlock := a.NoNode

	switch p.current.Type {
	case l.ELIF:
		elifStmt := p.parseIf()
		if elifStmt != a.NoNode {
			endPos = p.tree.Nodes[elifStmt].End
		} else {
			elifStmt = p.tree.NewNode(a.NodeErrStmt, p.current.Start, p.current.End)
		}

		orElseBlock = p.tree.NewNode(a.NodeBlock, p.tree.Nodes[elifStmt].Start, p.tree.Nodes[elifStmt].End)
		p.tree.AddChild(orElseBlock, elifStmt)
	case l.ELSE:
		p.advance()
		if p.current.Type != l.COLON {
			p.errorCurrent("expected `:` after else")
			p.syncTo(l.COLON, l.NEWLINE, l.EOF)
			if p.current.Type != l.COLON {
				ret := p.tree.NewNode(a.NodeIf, startPos, endPos)
				p.tree.AddChild(ret, testCond)
				p.tree.AddChild(ret, body)
				return ret
			}
		}

		p.advance()

		if p.current.Type != l.NEWLINE {
			p.errorCurrent("expected newline after `else:`")
			p.syncTo(l.NEWLINE, l.EOF)
			if p.current.Type == l.NEWLINE {
				p.advance()
			} else {
				ret := p.tree.NewNode(a.NodeIf, startPos, endPos)
				p.tree.AddChild(ret, testCond)
				if body != a.NoNode {
					p.tree.AddChild(ret, body)
				}
				return ret
			}
		} else {
			p.advance()
		}

		if p.current.Type != l.INDENT {
			p.errorCurrent("expected indent block after `else:`")
			p.syncTo(l.INDENT, l.DEDENT, l.EOF)
			if p.current.Type != l.INDENT {
				ret := p.tree.NewNode(a.NodeIf, startPos, endPos)
				p.tree.AddChild(ret, testCond)
				if body != a.NoNode {
					p.tree.AddChild(ret, body)
				}
				return ret
			}
		}

		p.advance()
		orElseBlock = p.tree.NewNode(a.NodeBlock, p.current.Start, p.current.Start)

		for p.current.Type != l.DEDENT && p.current.Type != l.EOF {
			if p.current.Type == l.NEWLINE {
				p.advance()
				continue
			}

			stmt := p.parseStatement()
			if stmt != a.NoNode {
				p.tree.AddChild(orElseBlock, stmt)
				if p.tree.Nodes[orElseBlock].FirstChild == stmt {
					p.tree.Nodes[orElseBlock].Start = p.tree.Nodes[stmt].Start
				}
				p.tree.Nodes[orElseBlock].End = p.tree.Nodes[stmt].End
				endPos = p.tree.Nodes[stmt].End
			}
		}
		if orElseBlock != a.NoNode && p.tree.Nodes[orElseBlock].End == p.tree.Nodes[orElseBlock].Start {
			p.tree.Nodes[orElseBlock].End = endPos
		}

		if p.current.Type == l.DEDENT {
			p.advance()
		}
	}

	ret := p.tree.NewNode(a.NodeIf, startPos, endPos)
	p.tree.AddChild(ret, testCond)
	if body != a.NoNode {
		p.tree.AddChild(ret, body)
	}
	if orElseBlock != a.NoNode {
		p.tree.AddChild(ret, orElseBlock)
	}

	return ret
}
