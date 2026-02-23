package parser

import (
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
	if p.current.Type == lexer.ELIF {
		elifStmt := p.parseIf()
		orelse = append(orelse, elifStmt)

		if ifNode, ok := elifStmt.(*a.If); ok {
			endPos = ifNode.Pos.End
		}
	} else if p.current.Type == lexer.ELSE {
		p.advance()

		endPos = p.current.End
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
		Start: startPos,
		End:   endPos,
	}

	return ifExpr
}

func (p *Parser) parseFor() a.Statement {
	startPos := p.current.Start
	forStmt := &a.For{Pos: a.Range{Start: startPos}}
	p.advance()
	forStmt.Target = p.parseForTarget()

	if p.current.Type != lexer.IN {
		p.errorCurrent("expected 'in' after loop variable")
		p.syncTo(lexer.IN, lexer.COLON, lexer.NEWLINE, lexer.EOF)
		if p.current.Type != lexer.IN {
			return forStmt
		}
	}

	p.advance()

	forStmt.Iter = p.parseExpression(LOWEST)

	if p.current.Type != lexer.COLON {
		p.errorCurrent("expected ':' after for clause")
		p.syncTo(lexer.COLON, lexer.NEWLINE, lexer.EOF)
		if p.current.Type != lexer.COLON {
			return forStmt
		}
	}
	p.advance()

	if p.current.Type != lexer.NEWLINE {
		p.errorCurrent("expected newline after ':'")
		p.syncTo(lexer.NEWLINE, lexer.EOF)
		if p.current.Type != lexer.NEWLINE {
			return forStmt
		}
	}

	p.advance()

	if p.current.Type != lexer.INDENT {
		p.errorCurrent("expected indent after for statement")
		p.syncTo(lexer.INDENT, lexer.DEDENT, lexer.EOF)
		if p.current.Type != lexer.INDENT {
			return forStmt
		}
	}

	p.advance()

	body := []a.Statement{}

	for p.current.Type != lexer.DEDENT && p.current.Type != lexer.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			body = append(body, stmt)
		}
	}

	forStmt.Body = body

	endPos := p.current.Start

	if p.current.Type == lexer.DEDENT {
		p.advance()
	}

	orelse := []a.Statement{}
	if p.current.Type == lexer.ELSE {
		p.advance()

		endPos = p.current.Start
		if p.current.Type != lexer.COLON {
			p.errorCurrent("expected ':' after else")
			p.syncTo(lexer.COLON, lexer.NEWLINE, lexer.EOF)
			if p.current.Type != lexer.COLON {
				return forStmt
			}
		}

		p.advance()

		if p.current.Type != lexer.NEWLINE {
			p.errorCurrent("expected newline after 'else:'")
			p.syncTo(lexer.NEWLINE, lexer.EOF)
			if p.current.Type != lexer.NEWLINE {
				return forStmt
			}
		}

		p.advance()

		if p.current.Type != lexer.INDENT {
			p.errorCurrent("expected indent block after 'else:'")
			p.syncTo(lexer.INDENT, lexer.DEDENT, lexer.EOF)
			if p.current.Type != lexer.INDENT {
				return forStmt
			}
		}

		p.advance()

		for p.current.Type != lexer.DEDENT && p.current.Type != lexer.EOF {
			stmt := p.parseStatement()
			if stmt != nil {
				orelse = append(orelse, stmt)
			}
		}

		endPos = p.current.Start

		if p.current.Type == lexer.DEDENT {
			p.advance()
		}
	}

	forStmt.Orelse = orelse
	forStmt.Pos.End = endPos
	return forStmt
}

func (p *Parser) parseReturn() a.Statement {
	startPos := p.current.Start
	p.advance()
	if p.current.Type == lexer.NEWLINE || p.current.Type == lexer.EOF {
		endPos := p.current.Start
		p.advance()
		return &a.Return{Value: nil, Pos: a.Range{Start: startPos, End: endPos}}
	}

	value := p.parseExpression(LOWEST)
	endPos := p.current.Start

	if p.current.Type == lexer.NEWLINE {
		p.advance()
	}

	return &a.Return{Value: value, Pos: a.Range{Start: startPos, End: endPos}}
}

func (p *Parser) parseAugAssign() a.Statement {
	start := p.current.Start
	target := p.parsePrimary()
	op := p.current.Type
	p.advance() // consume op

	value := p.parseExpression(LOWEST)

	end := p.current.Start

	if p.current.Type == lexer.NEWLINE {
		p.advance()
	}

	return &a.AugAssign{
		Target: target,
		Op:     op,
		Value:  value,
		Pos:    a.Range{Start: start, End: end},
	}
}

func (p *Parser) parseAssignment() a.Statement {
	start := p.current.Start
	targets := []a.Expression{}
	for {
		target := p.parsePrimary()
		targets = append(targets, target)
		if p.current.Type != lexer.COMMA {
			break
		}
		p.advance()
	}

	if p.current.Type != lexer.EQUAL {
		p.error(
			a.Range{
				Start: start,
				End:   p.current.Start,
			},
			"expected '=' in assignment",
		)
	}

	p.advance()

	// moved past '='

	value := p.parseExpression(LOWEST)

	assgnEnd := p.current.Start

	if p.current.Type == lexer.NEWLINE {
		p.advance()
	} else if p.current.Type != lexer.EOF {
		p.error(
			a.Range{
				Start: assgnEnd,
				End:   p.current.End,
			},
			"expected newline after assignment",
		)
	}

	return &a.Assign{
		Targets: targets,
		Value:   value,
		Pos:     a.Range{Start: start, End: assgnEnd},
	}
}

func (p *Parser) parseWhile() a.Statement {
	startPos := p.current.Start
	p.advance()
	whileStmt := &a.WhileLoop{}

	whileStmt.Test = p.parseExpression(LOWEST)

	if p.current.Type != lexer.COLON {
		p.errorCurrent("expected ':' after while condition")
		p.syncTo(lexer.COLON, lexer.NEWLINE, lexer.EOF)
		if p.current.Type != lexer.COLON {
			return whileStmt
		}
	}
	p.advance()

	if p.current.Type != lexer.NEWLINE {
		p.errorCurrent("expected newline after ':'")
		p.syncTo(lexer.NEWLINE, lexer.EOF)
		if p.current.Type != lexer.NEWLINE {
			return whileStmt
		}
	}
	p.advance()

	if p.current.Type != lexer.INDENT {
		p.errorCurrent("expected indent after while:")
		p.syncTo(lexer.INDENT, lexer.DEDENT, lexer.EOF)
		if p.current.Type != lexer.INDENT {
			return whileStmt
		}
	}
	p.advance()

	body := []a.Statement{}
	for p.current.Type != lexer.DEDENT && p.current.Type != lexer.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			body = append(body, stmt)
		}
	}
	whileStmt.Body = body

	endPos := p.current.Start
	if p.current.Type == lexer.DEDENT {
		p.advance()
	}
	whileStmt.Pos = a.Range{Start: startPos, End: endPos}

	return whileStmt
}

func (p *Parser) parseFunc() a.Statement {
	startPos := p.current.Start
	p.advance()
	funcDef := &a.FunctionDef{}

	if p.current.Type != lexer.NAME {
		p.errorCurrent("expected function name after 'def'")
		p.syncTo(lexer.NEWLINE, lexer.COLON, lexer.EOF)
		return funcDef
	}

	// funcDef.Name = p.current.Literal
	funcDef.Name = &a.Name{
		ID: p.current.Literal,
		Pos: a.Range{
			Start: p.current.Start,
			End:   p.current.End,
		},
	}
	funcDef.NamePos = a.Range{
		Start: p.current.Start,
		End:   p.current.End,
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
					ID:  p.current.Literal,
					Pos: a.Range{Start: start, End: end},
				},
				Pos: a.Range{Start: start, End: end},
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

	for p.current.Type != lexer.DEDENT && p.current.Type != lexer.EOF {
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
	funcDef.Pos = a.Range{Start: startPos, End: endPos}

	return funcDef
}

func (p *Parser) parseForTarget() a.Expression {
	if p.current.Type != lexer.NAME {
		p.errorCurrent("expected variable name")
		return nil
	}

	first := &a.Name{
		ID: p.current.Literal,
		Pos: a.Range{
			Start: p.current.Start,
			End:   p.current.End,
		},
	}
	p.advance()

	if p.current.Type == lexer.COMMA {
		targets := []a.Expression{first}

		for p.current.Type == lexer.COMMA {
			p.advance()

			if p.current.Type != lexer.NAME {
				p.errorCurrent("expected variable name")
				return &a.Tuple{
					Elts: targets,
					Pos: a.Range{
						Start: first.Pos.Start,
						End:   targets[len(targets)-1].Position().End,
					},
				}
			}
			targets = append(
				targets, &a.Name{
					ID: p.current.Literal,
					Pos: a.Range{
						Start: p.current.Start,
						End:   p.current.End,
					},
				})
			p.advance()
		}
		lastName := targets[len(targets)-1]
		return &a.Tuple{
			Elts: targets,
			Pos: a.Range{
				Start: first.Pos.Start,
				End:   lastName.Position().End,
			},
		}
	}
	return first
}

func (p *Parser) parseClass() a.Statement {
	// advance past `class`
	def := a.ClassDef{}
	startPos := p.current.Start
	p.advance()

	if p.current.Type != lexer.NAME {
		p.errorCurrent("expected classname after `class`")
		p.syncTo(lexer.NEWLINE, lexer.COLON, lexer.EOF)
		def.Pos = a.Range{
			Start: startPos,
			End:   p.current.End,
		}
		return &def
	}

	// def.Name = p.current.Literal
	def.Name = &a.Name{
		ID:  p.current.Literal,
		Pos: a.Range{Start: startPos, End: p.current.End},
	}

	p.advance()

	if p.current.Type != lexer.COLON && (p.current.Type != lexer.LPAR) {
		p.errorCurrent("expected `(` or `:` after class name")
		p.syncTo(lexer.NEWLINE, lexer.COLON, lexer.EOF)
		def.Pos = a.Range{
			Start: startPos,
			End:   p.current.End,
		}
		return &def
	}

	if p.current.Type == lexer.LPAR {
		p.advance() // advance past `(`
		for p.current.Type != lexer.RPAR && p.current.Type != lexer.EOF {
			if p.current.Type == lexer.NAME {
				name := a.Name{
					ID: p.current.Literal,
					Pos: a.Range{
						Start: p.current.Start,
						End:   p.current.End,
					},
				}

				def.Bases = append(def.Bases, &name)
				p.advance()
			} else if p.current.Type == lexer.COMMA {
				p.advance()
			} else {
				p.errorCurrent("unexpected token in class base list")
				p.syncTo(lexer.EOF, lexer.RPAR, lexer.COLON)
				break
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
				Start: startPos,
				End:   p.current.End,
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
				Start: startPos,
				End:   p.current.End,
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
				Start: startPos,
				End:   p.current.End,
			}

			return &def
		}
	}

	p.advance() // move past indent

	body := []a.Statement{}
	for p.current.Type != lexer.EOF && p.current.Type != lexer.DEDENT {
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
	def.Pos = a.Range{Start: startPos, End: endPos}
	return &def
}
