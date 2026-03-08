package parser

import (
	"strings"

	"rahu/lexer"
	a "rahu/parser/ast"
)

func (p *Parser) parseIf() a.Statement {
	startPos := p.current.Start

	p.advance()
	ifExpr := &a.If{}
	testCond := p.parseExpression(LOWEST)

	ifExpr.Test = testCond

	if p.current.Type != lexer.COLON {
		p.errorCurrent("expected ':' after if condition")
		p.syncTo(lexer.COLON, lexer.NEWLINE, lexer.EOF)
		if p.current.Type == lexer.COLON {
			p.advance()
		}
		return ifExpr
	}

	p.advance() // skip colon

	if p.current.Type != lexer.NEWLINE {
		p.errorCurrent("expected newline after ':'")
		p.syncTo(lexer.NEWLINE, lexer.EOF)
		if p.current.Type == lexer.NEWLINE {
			p.advance()
		}
		return ifExpr
	}
	p.advance() // moving ahead of newline

	if p.current.Type != lexer.INDENT {
		p.errorCurrent("expected indentation block after if condition")
		p.syncTo(lexer.INDENT, lexer.DEDENT, lexer.EOF)
		if p.current.Type != lexer.INDENT {
			return ifExpr
		}
	}
	p.advance() // moving ahead of indent

	body := []a.Statement{}

	for p.current.Type != lexer.DEDENT && p.current.Type != lexer.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			body = append(body, stmt)
		}
	}

	ifExpr.Body = body

	endPos := p.current.Start

	if p.current.Type == lexer.DEDENT {
		p.advance() // skip past dedent
	}

	orelse := []a.Statement{}
	switch p.current.Type {
	case lexer.ELIF:
		elifStmt := p.parseIf()
		orelse = append(orelse, elifStmt)

		if ifNode, ok := elifStmt.(*a.If); ok {
			endPos = uint32(ifNode.Pos.End)
		}
	case lexer.ELSE:
		p.advance()

		if p.current.Type != lexer.COLON {
			p.errorCurrent("expected ':' after else")
			p.syncTo(lexer.COLON, lexer.NEWLINE, lexer.EOF)
			if p.current.Type != lexer.COLON {
				return ifExpr
			}
		}

		p.advance() // advancing past 'else' block

		if p.current.Type != lexer.NEWLINE {
			p.errorCurrent("expected newline after 'else:'")
			p.syncTo(lexer.NEWLINE, lexer.EOF)
			if p.current.Type != lexer.NEWLINE {
				return ifExpr
			}
		}

		p.advance() // move past newline

		if p.current.Type != lexer.INDENT {
			p.errorCurrent("expected indent block after 'else:'")
			p.syncTo(lexer.INDENT, lexer.DEDENT, lexer.EOF)
			if p.current.Type != lexer.INDENT {
				return ifExpr
			}
		}

		p.advance() // move past indent

		for p.current.Type != lexer.DEDENT && p.current.Type != lexer.EOF {
			stmt := p.parseStatement()
			if stmt != nil {
				orelse = append(orelse, stmt)
			}
		}

		endPos = p.current.End

		if p.current.Type == lexer.DEDENT {
			p.advance() // advance past dedent
		}
	}

	ifExpr.Orelse = orelse
	ifExpr.Pos = a.Range{
		Start: int(startPos),
		End:   int(endPos),
	}

	return ifExpr
}

func (p *Parser) parseReturn() a.Statement {
	startPos := p.current.Start
	p.advance()
	if p.current.Type == lexer.NEWLINE || p.current.Type == lexer.EOF {
		endPos := p.current.Start
		p.advance()
		return &a.Return{Value: nil, Pos: a.Range{Start: int(startPos), End: int(endPos)}}
	}

	value := p.parseExpression(LOWEST)
	endPos := p.current.Start

	if p.current.Type == lexer.NEWLINE {
		p.advance()
	}

	return &a.Return{Value: value, Pos: a.Range{Start: int(startPos), End: int(endPos)}}
}

func (p *Parser) parseFunc() a.Statement {
	startPos := int(p.current.Start)
	p.advance()
	funcDef := &a.FunctionDef{
		DocString: "",
	}

	if p.current.Type != lexer.NAME {
		p.errorCurrent("expected function name after 'def'")
		p.syncTo(lexer.NEWLINE, lexer.COLON, lexer.EOF)
		funcDef.Name = &a.Name{
			Text: "<incomplete>",
			Pos:  a.Range{Start: startPos, End: int(p.current.End)},
			ID:   p.newNodeID(),
		}
		return funcDef
	}

	// funcDef.Name = p.current.Literal
	funcDef.Name = &a.Name{
		Text: p.current.Literal,
		Pos: a.Range{
			Start: int(p.current.Start),
			End:   int(p.current.End),
		},
		ID: p.newNodeID(),
	}
	funcDef.NamePos = a.Range{
		Start: int(p.current.Start),
		End:   int(p.current.End),
	}
	p.advance() // advance past func name

	if p.current.Type != lexer.LPAR {
		p.errorCurrent("expected '(' after function name")
		p.syncTo(lexer.LPAR, lexer.NEWLINE, lexer.EOF)
		if p.current.Type != lexer.LPAR {
			return funcDef
		}
	}

	p.advance() // advanced past left parantheses

	args := []a.FuncArg{}
	seenDefault := false

	if p.current.Type != lexer.RPAR {
		for {
			if p.current.Type != lexer.NAME {
				p.errorCurrent("expected parameter name")
				p.syncTo(lexer.COMMA, lexer.RPAR, lexer.EOF)
				if p.current.Type == lexer.COMMA {
					p.advance()
					continue
				}
				break
			}

			start := p.current.Start
			end := p.current.End

			arg := a.FuncArg{
				// Name: p.current.Literal,
				Name: &a.Name{
					Text: p.current.Literal,
					Pos:  a.Range{Start: int(start), End: int(end)},
					ID:   p.newNodeID(),
				},
				Pos: a.Range{Start: int(start), End: int(end)},
			}

			p.advance()

			if p.current.Type == lexer.EQUAL {
				seenDefault = true
				p.advance()
				arg.Default = p.parseExpression(LOWEST)
			} else if seenDefault {
				p.errorCurrent("non-default argument follows default argument")
				p.syncTo(lexer.COMMA, lexer.RPAR, lexer.EOF)
				if p.current.Type == lexer.COMMA {
					p.advance()
					continue
				}
			}

			args = append(args, arg)

			if p.current.Type == lexer.COMMA {
				p.advance()
				continue
			}
			break
		}
	}

	funcDef.Args = args

	if p.current.Type != lexer.RPAR {
		p.errorCurrent("expected ')' after params")
		p.syncTo(lexer.RPAR, lexer.NEWLINE, lexer.EOF)
		if p.current.Type != lexer.RPAR {
			return funcDef
		}
	}

	p.advance()

	if p.current.Type != lexer.COLON {
		p.errorCurrent("expected ':' after function signature")
		p.syncTo(lexer.COLON, lexer.NEWLINE, lexer.EOF)
		if p.current.Type != lexer.COLON {
			return funcDef
		}
	}
	p.advance()

	if p.current.Type != lexer.NEWLINE {
		p.errorCurrent("expected newline after ':'")
		p.syncTo(lexer.NEWLINE, lexer.EOF)
		if p.current.Type != lexer.NEWLINE {
			return funcDef
		}
	}

	p.advance()

	if p.current.Type != lexer.INDENT {
		p.errorCurrent("expected indentation after function signature")
		p.syncTo(lexer.INDENT, lexer.DEDENT, lexer.EOF)
		if p.current.Type != lexer.INDENT {
			return funcDef
		}
	}
	p.advance()

	funcBody := []a.Statement{}
	atStart := true

	for p.current.Type != lexer.DEDENT && p.current.Type != lexer.EOF {
		// starting from first line of func body
		if atStart && p.current.Type == lexer.STRING {
			funcDef.DocString = strings.TrimSpace(p.current.Literal)
			p.advance()
		}
		atStart = false
		stmt := p.parseStatement()
		if stmt != nil {
			funcBody = append(funcBody, stmt)
		}
	}

	if p.current.Type == lexer.DEDENT {
		p.advance() // skip past dedent
	}

	funcDef.Body = funcBody
	endPos := p.current.Start
	funcDef.Pos = a.Range{Start: startPos, End: int(endPos)}

	return funcDef
}

func (p *Parser) parseClass() a.Statement {
	// advance past `class`
	def := a.ClassDef{
		DocString: "",
	}
	startPos := p.current.Start
	p.advance()

	if p.current.Type != lexer.NAME {
		p.errorCurrent("expected classname after `class`")
		p.syncTo(lexer.NEWLINE, lexer.COLON, lexer.EOF)
		def.Pos = a.Range{
			Start: int(startPos),
			End:   int(p.current.End),
		}
		def.Name = &a.Name{
			Text: "<incomplete>",
			Pos: a.Range{
				Start: int(startPos),
				End:   int(p.current.End),
			},
			ID: p.newNodeID(),
		}
		return &def
	}

	// def.Name = p.current.Literal
	def.Name = &a.Name{
		Text: p.current.Literal,
		Pos:  a.Range{Start: int(p.current.Start), End: int(p.current.End)},
		ID:   p.newNodeID(),
	}

	p.advance()

	if p.current.Type != lexer.COLON && (p.current.Type != lexer.LPAR) {
		p.errorCurrent("expected `(` or `:` after class name")
		p.syncTo(lexer.NEWLINE, lexer.COLON, lexer.EOF)
		def.Pos = a.Range{
			Start: int(startPos),
			End:   int(p.current.End),
		}
		return &def
	}

	if p.current.Type == lexer.LPAR {
		p.advance() // advance past `(`
	Loop:
		for p.current.Type != lexer.RPAR && p.current.Type != lexer.EOF {
			switch p.current.Type {
			case lexer.NAME:
				name := a.Name{
					Text: p.current.Literal,
					Pos: a.Range{
						Start: int(p.current.Start),
						End:   int(p.current.End),
					},
					ID: p.newNodeID(),
				}

				def.Bases = append(def.Bases, &name)
				p.advance()
			case lexer.COMMA:
				p.advance()
			default:
				p.errorCurrent("unexpected token in class base list")
				p.syncTo(lexer.EOF, lexer.RPAR, lexer.COLON)
				break Loop
			}
		}

		if p.current.Type == lexer.RPAR {
			p.advance()
		}
	}

	if p.current.Type != lexer.COLON {
		p.errorCurrent("expected `:` after class header")
		p.syncTo(lexer.COLON, lexer.EOF, lexer.RPAR)
		if p.current.Type != lexer.COLON {
			def.Pos = a.Range{
				Start: int(startPos),
				End:   int(p.current.End),
			}
			return &def
		}
	}

	p.advance() // advance past colon

	if p.current.Type != lexer.NEWLINE {
		p.errorCurrent("expected newline after `:`")
		p.syncTo(lexer.EOF, lexer.NEWLINE)
		if p.current.Type != lexer.NEWLINE {
			def.Pos = a.Range{
				Start: int(startPos),
				End:   int(p.current.End),
			}
			return &def
		}
	}
	p.advance() // advance past newline

	if p.current.Type != lexer.INDENT {
		p.errorCurrent("expected indent after class definition")
		p.syncTo(lexer.INDENT, lexer.DEDENT, lexer.EOF)
		if p.current.Type != lexer.INDENT {
			def.Pos = a.Range{
				Start: int(startPos),
				End:   int(p.current.End),
			}

			return &def
		}
	}

	p.advance() // move past indent

	body := []a.Statement{}
	atStart := true
	for p.current.Type != lexer.EOF && p.current.Type != lexer.DEDENT {
		if atStart && p.current.Type == lexer.STRING {
			def.DocString = strings.TrimSpace(p.current.Literal)
			p.advance()
		}
		atStart = false
		stmt := p.parseStatement()
		if stmt != nil {
			body = append(body, stmt)
		}
	}

	if p.current.Type == lexer.DEDENT {
		p.advance()
	}

	def.Body = body
	endPos := p.current.End
	def.Pos = a.Range{Start: int(startPos), End: int(endPos)}
	return &def
}
