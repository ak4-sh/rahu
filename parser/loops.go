package parser

import (
	"rahu/lexer"
	a "rahu/parser/ast"
)

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
