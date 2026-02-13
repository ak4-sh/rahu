package parser

import (
	"rahu/lexer"
)

func (p *Parser) parseIf() Statement {
	startPos := Position{
		Line: p.current.Line,
		Col:  p.current.Col,
	}

	p.advance()
	ifExpr := &If{}
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

	body := []Statement{}

	for p.current.Type != lexer.DEDENT && p.current.Type != lexer.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			body = append(body, stmt)
		}
	}

	ifExpr.Body = body

	endPos := Position{
		Line: p.current.Line,
		Col:  p.current.Col,
	}

	if p.current.Type == lexer.DEDENT {
		p.advance() // skip past dedent
	}

	orelse := []Statement{}
	if p.current.Type == lexer.ELIF {
		elifStmt := p.parseIf()
		orelse = append(orelse, elifStmt)

		if ifNode, ok := elifStmt.(*If); ok {
			endPos = ifNode.Pos.End
		}
	} else if p.current.Type == lexer.ELSE {
		p.advance()

		endPos.Col = p.current.Col
		endPos.Line = p.current.Line
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

		endPos = Position{
			Line: p.current.Line,
			Col:  p.current.Col,
		}

		if p.current.Type == lexer.DEDENT {
			p.advance() // advance past dedent
		}
	}

	ifExpr.Orelse = orelse
	ifExpr.Pos = Range{
		Start: startPos,
		End:   endPos,
	}

	return ifExpr
}

func (p *Parser) parseFor() Statement {
	startPos := Position{Line: p.current.Line, Col: p.current.Col}
	forStmt := &For{Pos: Range{Start: startPos}}
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

	body := []Statement{}

	for p.current.Type != lexer.DEDENT && p.current.Type != lexer.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			body = append(body, stmt)
		}
	}

	forStmt.Body = body

	endPos := Position{Line: p.current.Line, Col: p.current.Col}

	if p.current.Type == lexer.DEDENT {
		p.advance()
	}

	orelse := []Statement{}
	if p.current.Type == lexer.ELSE {
		p.advance()

		endPos = Position{Line: p.current.Line, Col: p.current.Col}
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

		endPos = Position{Line: p.current.Line, Col: p.current.Col}

		if p.current.Type == lexer.DEDENT {
			p.advance()
		}
	}

	forStmt.Orelse = orelse
	forStmt.Pos.End = endPos
	return forStmt
}

func (p *Parser) parseReturn() Statement {
	startPos := Position{Line: p.current.Line, Col: p.current.Col}
	p.advance()
	if p.current.Type == lexer.NEWLINE || p.current.Type == lexer.EOF {
		endPos := Position{Line: p.current.Line, Col: p.current.Col}
		p.advance()
		return &Return{Value: nil, Pos: Range{Start: startPos, End: endPos}}
	}

	value := p.parseExpression(LOWEST)
	endPos := Position{Line: p.current.Line, Col: p.current.Col}

	if p.current.Type == lexer.NEWLINE {
		p.advance()
	}

	return &Return{Value: value, Pos: Range{Start: startPos, End: endPos}}
}

func (p *Parser) parseAssignment() Statement {
	start := Position{Line: p.current.Line, Col: p.current.Col}
	targets := []Expression{}
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
			Range{
				Start: start,
				End:   Position{Line: p.current.Line, Col: p.current.EndCol},
			},
			"expected '=' in assignment",
		)
	}

	p.advance()

	// moved past '='

	value := p.parseExpression(LOWEST)

	assgnEnd := Position{
		Line: p.current.Line,
		Col:  p.current.Col,
	}

	if p.current.Type == lexer.NEWLINE {
		p.advance()
	} else if p.current.Type != lexer.EOF {
		p.error(
			Range{
				Start: assgnEnd,
				End:   Position{Line: p.current.Line, Col: p.current.EndCol},
			},
			"expected newline after assignment",
		)
	}

	return &Assign{
		Targets: targets,
		Value:   value,
		Pos:     Range{Start: start, End: assgnEnd},
	}
}

func (p *Parser) parseWhile() Statement {
	startPos := Position{Line: p.current.Line, Col: p.current.Col}
	p.advance()
	whileStmt := &WhileLoop{}

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

	body := []Statement{}
	for p.current.Type != lexer.DEDENT && p.current.Type != lexer.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			body = append(body, stmt)
		}
	}
	whileStmt.Body = body

	endPos := Position{Line: p.current.Line, Col: p.current.Col}
	if p.current.Type == lexer.DEDENT {
		p.advance()
	}
	whileStmt.Pos = Range{Start: startPos, End: endPos}

	return whileStmt
}

func (p *Parser) parseFunc() Statement {
	startPos := Position{Line: p.current.Line, Col: p.current.Col}
	p.advance()
	funcDef := &FunctionDef{}

	if p.current.Type != lexer.NAME {
		p.errorCurrent("expected function name after 'def'")
		p.syncTo(lexer.NEWLINE, lexer.COLON, lexer.EOF)
		return funcDef
	}

	funcDef.Name = p.current.Literal
	funcDef.NamePos = Range{
		Start: Position{Line: p.current.Line, Col: p.current.Col},
		End:   Position{Line: p.current.Line, Col: p.current.EndCol},
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

	args := []FuncArg{}
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

			start := Position{Line: p.current.Line, Col: p.current.Col}
			end := Position{Line: p.current.Line, Col: p.current.EndCol}

			arg := FuncArg{
				Name: p.current.Literal,
				Pos:  Range{Start: start, End: end},
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

	funcBody := []Statement{}

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
	endPos := Position{Line: p.current.Line, Col: p.current.Col}
	funcDef.Pos = Range{Start: startPos, End: endPos}

	return funcDef
}

func (p *Parser) parseForTarget() Expression {
	if p.current.Type != lexer.NAME {
		p.errorCurrent("expected variable name")
		return nil
	}

	first := &Name{
		ID: p.current.Literal,
		Pos: Range{
			Start: Position{Line: p.current.Line, Col: p.current.Col},
			End:   Position{Line: p.current.Line, Col: p.current.EndCol},
		},
	}
	p.advance()

	if p.current.Type == lexer.COMMA {
		targets := []Expression{first}

		for p.current.Type == lexer.COMMA {
			p.advance()

			if p.current.Type != lexer.NAME {
				p.errorCurrent("expected variable name")
				return &Tuple{
					Elts: targets,
					Pos: Range{
						Start: first.Pos.Start,
						End:   targets[len(targets)-1].Position().End,
					},
				}
			}
			targets = append(
				targets, &Name{
					ID: p.current.Literal,
					Pos: Range{
						Start: Position{Line: p.current.Line, Col: p.current.Col},
						End:   Position{Line: p.current.Line, Col: p.current.EndCol},
					},
				})
			p.advance()
		}
		lastName := targets[len(targets)-1]
		return &Tuple{
			Elts: targets,
			Pos: Range{
				Start: first.Pos.Start,
				End:   lastName.Position().End,
			},
		}
	}
	return first
}

func (p *Parser) parseClass() Statement {
	// advance past `class`
	def := ClassDef{}
	startPos := Position{Line: p.current.Line, Col: p.current.Col}
	p.advance()

	if p.current.Type != lexer.NAME {
		p.errorCurrent("expected classname after `class`")
		p.syncTo(lexer.NEWLINE, lexer.COLON, lexer.EOF)
		def.Pos = Range{
			Start: startPos,
			End:   Position{Col: p.current.Col, Line: p.current.Line},
		}
		return &def
	}

	def.Name = p.current.Literal

	p.advance()

	if p.current.Type != lexer.COLON && (p.current.Type != lexer.LPAR) {
		p.errorCurrent("expected `(` or `:` after class name")
		p.syncTo(lexer.NEWLINE, lexer.COLON, lexer.EOF)
		def.Pos = Range{
			Start: startPos,
			End:   Position{Col: p.current.Col, Line: p.current.Line},
		}
		return &def
	}

	if p.current.Type == lexer.LPAR {
		p.advance() // advance past `(`
		for p.current.Type != lexer.RPAR && p.current.Type != lexer.EOF {
			if p.current.Type == lexer.NAME {
				name := Name{
					ID: p.current.Literal,
					Pos: Range{
						Start: Position{Line: p.current.Line, Col: p.current.Col},
						End:   Position{Line: p.current.Line, Col: p.current.EndCol},
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
			def.Pos = Range{
				Start: startPos,
				End: Position{
					Line: p.current.Line,
					Col:  p.current.Col,
				},
			}
			return &def
		}
	}

	p.advance() // advance past colon

	if p.current.Type != lexer.NEWLINE {
		p.errorCurrent("expected newline after `:`")
		p.syncTo(lexer.EOF, lexer.NEWLINE)
		if p.current.Type != lexer.NEWLINE {
			def.Pos = Range{
				Start: startPos,
				End:   Position{Line: p.current.Line, Col: p.current.Col},
			}
			return &def
		}
	}
	p.advance() // advance past newline

	if p.current.Type != lexer.INDENT {
		p.errorCurrent("expected indent after class definition")
		p.syncTo(lexer.INDENT, lexer.DEDENT, lexer.EOF)
		if p.current.Type != lexer.INDENT {
			def.Pos = Range{
				Start: startPos,
				End:   Position{Line: p.current.Line, Col: p.current.Col},
			}

			return &def
		}
	}

	p.advance() // move past indent

	body := []Statement{}
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
	endPos := Position{Line: p.current.Line, Col: p.current.Col}
	def.Pos = Range{Start: startPos, End: endPos}
	return &def
}
