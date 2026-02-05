package parser

import (
	"fmt"

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
		fmt.Printf("panicked with curr type : %v and testCond: %v\n", p.current, testCond)
		panic(`expected ':' after if condition`)
	}

	p.advance() // skip colon

	if p.current.Type != lexer.NEWLINE {
		panic(`expected newline after ":"`)
	}
	p.advance() // moving ahead of newline

	if p.current.Type != lexer.INDENT {
		panic("expected indentation block after if condtion")
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
			panic(`expected ':' after else`)
		}

		p.advance() // advancing past 'else' block

		if p.current.Type != lexer.NEWLINE {
			panic(`expected NEWLINE after 'else:'`)
		}

		p.advance() // move past newline

		if p.current.Type != lexer.INDENT {
			panic(`expected indent block after "else:"`)
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
		panic(`expected 'in' after loop variable`)
	}

	p.advance()

	forStmt.Iter = p.parseExpression(LOWEST)

	if p.current.Type != lexer.COLON {
		fmt.Printf("%vfailed for token v\n", p.current)
		panic(`expected ':' after for clause`)
	}
	p.advance()

	if p.current.Type != lexer.NEWLINE {
		panic(`expected newline after ':'`)
	}

	p.advance()

	if p.current.Type != lexer.INDENT {
		panic(`expected indent after for statement`)
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

	if p.current.Type == lexer.DEDENT {
		p.advance()
	}
	forStmt.Pos.End = Position{Line: p.current.Line, Col: p.current.Col}
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
	assgnStart := Position{Line: p.current.Line, Col: p.current.Col}
	targetStart := Position{Line: p.current.Line, Col: p.current.Col}

	target := &Name{ID: p.current.Literal}

	p.advance()

	target.Pos = Range{
		Start: targetStart,
		End:   Position{Line: p.current.Line, Col: p.current.Col},
	}

	if p.current.Type != lexer.EQUAL {
		p.error(
			Range{
				Start: targetStart,
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
		Targets: []Expression{target},
		Value:   value,
		Pos:     Range{Start: assgnStart, End: assgnEnd},
	}
}

func (p *Parser) parseWhile() Statement {
	startPos := Position{Line: p.current.Line, Col: p.current.Col}
	p.advance()
	whileStmt := &WhileLoop{}

	whileStmt.Test = p.parseExpression(LOWEST)

	if p.current.Type != lexer.COLON {
		panic(`expected ':' after while condition`)
	}
	p.advance()

	if p.current.Type != lexer.NEWLINE {
		panic(`expected newline after ':'`)
	}
	p.advance()

	if p.current.Type != lexer.INDENT {
		panic(`expected indent after while:`)
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
		fmt.Printf("%v\n", p.current)
		panic(`expected function name after "def"`)
	}

	funcDef.Name = p.current.Literal
	p.advance() // advance past func name

	if p.current.Type != lexer.LPAR {
		panic(`expected '(' after function name`)
	}

	p.advance() // advanced past left parantheses

	args := []FuncArg{}
	seenDefault := false

	if p.current.Type != lexer.RPAR {
		for {
			if p.current.Type != lexer.NAME {
				panic("expected param name")
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
				panic("non-default argumentn follows default argument")
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
		panic(`expected ')' after params`)
	}

	p.advance()

	if p.current.Type != lexer.COLON {
		panic(`expected ':' after function signature`)
	}
	p.advance()

	if p.current.Type != lexer.NEWLINE {
		panic(`expected newline after ':'`)
	}

	p.advance()

	if p.current.Type != lexer.INDENT {
		panic(`expected indentation after function signature`)
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
		panic(`expected variable name`)
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
				panic(`expected variable name`)
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
