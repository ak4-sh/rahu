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
	seenVarArg := false
	seenKwArg := false

	if p.current.Type != l.RPAR {
		for {
			param, isVarArg, isKwArg := p.parseParameter()
			if param == a.NoNode {
				p.errorCurrent("expected parameter name")
				p.syncTo(l.COMMA, l.RPAR, l.EOF)
				if p.current.Type == l.COMMA {
					p.advance()
					continue
				}
				break
			}

			if seenKwArg {
				p.error(a.Range{Start: p.tree.Nodes[param].Start, End: p.tree.Nodes[param].End}, "parameter after **kwargs is not allowed")
			}
			if isVarArg {
				if seenVarArg {
					p.error(a.Range{Start: p.tree.Nodes[param].Start, End: p.tree.Nodes[param].End}, "duplicate *args parameter")
				}
				seenVarArg = true
				seenDefault = false
			}
			if isKwArg {
				if seenKwArg {
					p.error(a.Range{Start: p.tree.Nodes[param].Start, End: p.tree.Nodes[param].End}, "duplicate **kwargs parameter")
				}
				seenKwArg = true
				seenDefault = false
			}
			if !isVarArg && !isKwArg {
				_, _, defaultExpr := p.tree.ParamParts(param)
				if defaultExpr != a.NoNode {
					seenDefault = true
				} else if seenDefault {
					p.errorCurrent("non-default argument follows default argument")
					p.syncTo(l.COMMA, l.RPAR, l.EOF)
					if p.current.Type == l.COMMA {
						p.advance()
						continue
					}
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

func (p *Parser) parseParameter() (param a.NodeID, isVarArg bool, isKwArg bool) {
	flags := uint32(0)
	start := p.current.Start
	if p.current.Type == l.STAR {
		isVarArg = true
		flags |= a.ParamFlagIsVarArg
		start = p.current.Start
		p.advance()
		if p.current.Type != l.NAME {
			p.errorCurrent("expected parameter name after '*'")
			return a.NoNode, false, false
		}
	} else if p.current.Type == l.DOUBLESTAR {
		isKwArg = true
		flags |= a.ParamFlagIsKwArg
		start = p.current.Start
		p.advance()
		if p.current.Type != l.NAME {
			p.errorCurrent("expected parameter name after '**'")
			return a.NoNode, false, false
		}
	} else if p.current.Type != l.NAME {
		return a.NoNode, false, false
	}

	paramName := p.tree.NewNameNode(p.current.Start, p.current.End, p.current.Literal)
	param = p.tree.NewNode(a.NodeParam, start, p.current.End)
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
		flags |= a.ParamFlagHasAnnotation
		p.tree.Nodes[param].End = p.tree.Nodes[annotation].End
	}

	if p.current.Type == l.EQUAL {
		if isVarArg {
			p.errorCurrent("*args cannot have a default value")
		} else if isKwArg {
			p.errorCurrent("**kwargs cannot have a default value")
		}
		p.advance()

		defaultExpr := p.parseExpression(LOWEST)
		if defaultExpr == a.NoNode {
			p.errorCurrent("expected expression after '='")
			defaultExpr = p.tree.NewNode(a.NodeErrExp, p.current.Start, p.current.Start)
		}

		if !isVarArg && !isKwArg {
			p.tree.AddChild(param, defaultExpr)
			flags |= a.ParamFlagHasDefault
		}
		p.tree.Nodes[param].End = p.tree.Nodes[defaultExpr].End
	}

	p.tree.Nodes[param].Data = flags
	return param, isVarArg, isKwArg
}

func (p *Parser) parseDecoratedDef() a.NodeID {
	decorators := make([]a.NodeID, 0, 2)
	startPos := p.current.Start

	for p.current.Type == l.AT {
		decoratorStart := p.current.Start
		p.advance()

		expr := p.parseExpression(LOWEST)
		if expr == a.NoNode {
			p.errorCurrent("expected decorator expression after '@'")
			return p.tree.NewNode(a.NodeErrStmt, startPos, p.current.End)
		}

		decorator := p.tree.NewNode(a.NodeDecorator, decoratorStart, p.tree.Nodes[expr].End)
		p.tree.AddChild(decorator, expr)
		decorators = append(decorators, decorator)

		if p.current.Type != l.NEWLINE {
			p.errorCurrent("expected newline after decorator")
			p.syncTo(l.NEWLINE, l.EOF)
			if p.current.Type != l.NEWLINE {
				return p.tree.NewNode(a.NodeErrStmt, startPos, p.current.End)
			}
		}
		p.advance()
	}

	var def a.NodeID
	switch p.current.Type {
	case l.DEF:
		def = p.parseFunc()
	case l.CLASS:
		def = p.parseClass()
	default:
		p.errorCurrent("expected function or class definition after decorator")
		return p.tree.NewNode(a.NodeErrStmt, startPos, p.current.End)
	}

	p.tree.PrependChildren(def, decorators...)
	p.tree.Nodes[def].Start = startPos
	return def
}
