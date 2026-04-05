package parser

import (
	"strings"

	l "rahu/lexer"
	a "rahu/parser/ast"
)

func (p *Parser) parseFunc() a.NodeID {
	startPos := p.current.Start
	p.advance()

	if p.current.Type != l.NAME {
		p.errorCurrent("expected function name after `def`")
		p.syncTo(l.NEWLINE, l.COLON, l.EOF)
		name := p.tree.NewNameNode(startPos, p.current.End, "<incomplete>")
		ret := p.tree.NewNode(a.NodeFunctionDef, startPos, p.current.End)
		p.tree.AddChild(ret, name)
		return ret
	}

	name := p.tree.NewNameNode(p.current.Start, p.current.End, p.current.Literal)
	p.advance()

	if p.current.Type != l.LPAR {
		p.errorCurrent("expected '(' after function name")
		p.syncTo(l.LPAR, l.NEWLINE, l.EOF)
		if p.current.Type != l.LPAR {
			ret := p.tree.NewNode(a.NodeFunctionDef, startPos, p.current.End)
			p.tree.AddChild(ret, name)
			return ret
		}
	}

	p.advance()

	args := a.NoNode
	seenDefault := false

	if p.current.Type != l.RPAR {
		for {
			if p.current.Type != l.NAME {
				p.errorCurrent("expected parameter name")
				p.syncTo(l.COMMA, l.RPAR, l.EOF)
				if p.current.Type == l.COMMA {
					p.advance()
					continue
				}
				break
			}

			paramName := p.tree.NewNameNode(p.current.Start, p.current.End, p.current.Literal)
			param := p.tree.NewNode(a.NodeParam, p.current.Start, p.current.End)
			p.tree.AddChild(param, paramName)
			p.advance()

			if p.current.Type == l.COLON {
				p.advance()

				annotation := p.parseExpression(LOWEST)
				if annotation == a.NoNode {
					p.errorCurrent("expected type annotation after ':'")
					annotation = p.tree.NewNode(a.NodeErrExp, p.current.Start, p.current.Start)
				}

				p.tree.AddChild(param, annotation)
				p.tree.Nodes[param].Data |= 1
				p.tree.Nodes[param].End = p.tree.Nodes[annotation].End
			}

			if p.current.Type == l.EQUAL {
				seenDefault = true
				p.advance()

				defaultExpr := p.parseExpression(LOWEST)
				if defaultExpr == a.NoNode {
					p.errorCurrent("expected expression after '='")
					defaultExpr = p.tree.NewNode(a.NodeErrExp, p.current.Start, p.current.Start)
				}

				p.tree.AddChild(param, defaultExpr)
				p.tree.Nodes[param].Data |= 2
				p.tree.Nodes[param].End = p.tree.Nodes[defaultExpr].End
			} else if seenDefault {
				p.errorCurrent("non-default argument follows default argument")
				p.syncTo(l.COMMA, l.RPAR, l.EOF)
				if p.current.Type == l.COMMA {
					p.advance()
					continue
				}
			}

			if args == a.NoNode {
				args = p.tree.NewNode(a.NodeArgs, p.tree.Nodes[param].Start, p.tree.Nodes[param].End)
			}
			p.tree.AddChild(args, param)
			p.tree.Nodes[args].End = p.tree.Nodes[param].End

			if p.current.Type == l.COMMA {
				p.advance()
				continue
			}

			break
		}
	}

	if p.current.Type != l.RPAR {
		p.errorCurrent("expected ')' after params")
		p.syncTo(l.RPAR, l.NEWLINE, l.EOF)
		if p.current.Type != l.RPAR {
			ret := p.tree.NewNode(a.NodeFunctionDef, startPos, p.current.End)
			p.tree.AddChild(ret, name)
			if args != a.NoNode {
				p.tree.AddChild(ret, args)
			}
			return ret
		}
	}
	p.advance()

	returnAnnotation := a.NoNode
	if p.current.Type == l.RARROW {
		p.advance()

		returnAnnotation = p.parseExpression(LOWEST)
		if returnAnnotation == a.NoNode {
			p.errorCurrent("expected return type after '->'")
			returnAnnotation = p.tree.NewNode(a.NodeErrExp, p.current.Start, p.current.Start)
		}
	}

	if p.current.Type != l.COLON {
		p.errorCurrent("expected ':' after function signature")
		p.syncTo(l.COLON, l.NEWLINE, l.EOF)
		if p.current.Type != l.COLON {
			ret := p.tree.NewNode(a.NodeFunctionDef, startPos, p.current.End)
			p.tree.AddChild(ret, name)
			if args != a.NoNode {
				p.tree.AddChild(ret, args)
			}
			return ret
		}
	}
	p.advance()

	if p.current.Type != l.NEWLINE {
		p.errorCurrent("expected newline after ':'")
		p.syncTo(l.NEWLINE, l.EOF)
		if p.current.Type != l.NEWLINE {
			ret := p.tree.NewNode(a.NodeFunctionDef, startPos, p.current.End)
			p.tree.AddChild(ret, name)
			if args != a.NoNode {
				p.tree.AddChild(ret, args)
			}
			return ret
		}
	}
	p.advance()
	for p.current.Type == l.NEWLINE {
		p.advance()
	}

	if p.current.Type != l.INDENT {
		p.errorCurrent("expected indentation after function signature")
		p.syncTo(l.INDENT, l.DEDENT, l.EOF)
		if p.current.Type != l.INDENT {
			ret := p.tree.NewNode(a.NodeFunctionDef, startPos, p.current.End)
			p.tree.AddChild(ret, name)
			if args != a.NoNode {
				p.tree.AddChild(ret, args)
			}
			return ret
		}
	}
	p.advance()

	body := p.tree.NewNode(a.NodeBlock, p.current.Start, p.current.Start)
	docString := ""
	atStart := true

	for p.current.Type != l.DEDENT && p.current.Type != l.EOF {
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
			p.tree.AddChild(body, stmt)
			if p.tree.Nodes[body].FirstChild == stmt {
				p.tree.Nodes[body].Start = p.tree.Nodes[stmt].Start
			}
			p.tree.Nodes[body].End = p.tree.Nodes[stmt].End
		}
	}

	endPos := p.current.End
	if p.current.Type == l.DEDENT {
		endPos = p.current.End
		p.advance()
	}

	if p.tree.Nodes[body].End == p.tree.Nodes[body].Start {
		p.tree.Nodes[body].End = endPos
	}

	ret := p.tree.NewNode(a.NodeFunctionDef, startPos, endPos)
	p.tree.AddChild(ret, name)
	if args != a.NoNode {
		p.tree.AddChild(ret, args)
	}
	if returnAnnotation != a.NoNode {
		p.tree.AddChild(ret, returnAnnotation)
	}
	if docString != "" {
		idx := uint32(len(p.tree.Strings))
		p.tree.Strings = append(p.tree.Strings, docString)
		p.tree.Nodes[ret].Data = idx
	}
	p.tree.AddChild(ret, body)

	return ret
}
