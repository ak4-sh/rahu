package parser

import (
	"fmt"

	"rahu/lexer"
)

func (p *Parser) parseExpression(minBP int) Expression {
	left := p.parsePrimary()

	for {
		bp := infixBindingPower(p.current.Type)
		if bp <= minBP {
			break
		}

		opTok := p.current

		if isCompareOp(opTok.Type) {
			startPos := left.Position().Start
			ops := []CompareOp{}
			rights := []Expression{}

			for isCompareOp(p.current.Type) {
				op := tokenTypeToCompareOp(p.current.Type)
				p.advance()

				right := p.parseExpression(COMPARE + 1)
				ops = append(ops, op)
				rights = append(rights, right)
			}
			lastRight := rights[len(rights)-1]
			endPos := lastRight.Position().End

			left = &Compare{
				Left:  left,
				Ops:   ops,
				Right: rights,
				Pos:   Range{Start: startPos, End: endPos},
			}
			continue
		}

		if opTok.Type == lexer.AND || opTok.Type == lexer.OR {
			p.advance()
			right := p.parseExpression(bp)

			if boolOp, ok := left.(*BooleanOp); ok {
				if (opTok.Type == lexer.AND && boolOp.Operator == And) || (opTok.Type == lexer.OR && boolOp.Operator == Or) {
					boolOp.Values = append(boolOp.Values, right)
					continue
				}
			}

			op := And
			if opTok.Type == lexer.OR {
				op = Or
			}
			left = &BooleanOp{
				Operator: op,
				Values:   []Expression{left, right},
				Pos: Range{
					Start: left.Position().Start,
					End:   right.Position().End,
				},
			}
			continue
		}
		p.advance()

		right := p.parseExpression(bp)

		left = &BinOp{
			Left:  left,
			Op:    p.tokenTypeToOperator(opTok.Type),
			Right: right,
			Pos: Range{
				Start: left.Position().Start,
				End:   right.Position().End,
			},
		}
	}

	return left
}

func (p *Parser) parsePrimary() Expression {
	switch p.current.Type {
	case lexer.NUMBER:
		n := &Number{
			Value: p.current.Literal,
			Pos: Range{
				Start: Position{Line: p.current.Line, Col: p.current.Col},
				End:   Position{Line: p.current.Line, Col: p.current.EndCol},
			},
		}
		p.advance()
		return n
	case lexer.TRUE:
		ret := &Boolean{
			Value: true,
			Pos: Range{
				Start: Position{Line: p.current.Line, Col: p.current.Col},
				End:   Position{Line: p.current.Line, Col: p.current.EndCol},
			},
		}
		p.advance()
		return ret

	case lexer.FALSE:
		ret := &Boolean{
			Value: false,
			Pos: Range{
				Start: Position{Line: p.current.Line, Col: p.current.Col},
				End:   Position{Line: p.current.Line, Col: p.current.EndCol},
			},
		}
		p.advance()
		return ret
	case lexer.NAME:
		n := &Name{
			ID: p.current.Literal,
			Pos: Range{
				Start: Position{Line: p.current.Line, Col: p.current.Col},
				End:   Position{Line: p.current.Line, Col: p.current.EndCol},
			},
		}
		p.advance()

		if p.current.Type == lexer.LPAR {
			return p.parseCall(n)
		}
		return n
	case lexer.LPAR:
		p.advance()
		expr := p.parseExpression(LOWEST)
		if p.current.Type != lexer.RPAR {
			panic(`expected )`)
		}
		p.advance()
		return expr
	case lexer.LSQB:
		return p.parseList()
	case lexer.STRING:
		s := &String{
			Value: p.current.Literal,
			Pos: Range{
				Start: Position{Line: p.current.Line, Col: p.current.Col},
				End:   Position{Line: p.current.Line, Col: p.current.EndCol},
			},
		}
		p.advance()
		return s

	case lexer.MINUS:
		startPos := Position{Line: p.current.Line, Col: p.current.Col}
		p.advance()
		operand := p.parseExpression(PREFIX)
		endPos := operand.Position().End
		return &UnaryOp{
			Op:      USub,
			Operand: operand,
			Pos:     Range{Start: startPos, End: endPos},
		}

	case lexer.PLUS:
		startPos := Position{Line: p.current.Line, Col: p.current.Col}
		p.advance()
		operand := p.parseExpression(PREFIX)
		endPos := operand.Position().End
		return &UnaryOp{
			Op:      UAdd,
			Operand: operand,
			Pos:     Range{Start: startPos, End: endPos},
		}

	case lexer.NOT:
		startPos := Position{Line: p.current.Line, Col: p.current.Col}
		p.advance()
		expr := p.parseExpression(PREFIX)
		endPos := expr.Position().End
		return &UnaryOp{
			Op:      Not,
			Operand: expr,
			Pos:     Range{Start: startPos, End: endPos},
		}

	}
	panic(fmt.Sprintf("unexpected token %v\n", p.current))
}

func (p *Parser) parseList() Expression {
	startPos := Position{Line: p.current.Line, Col: p.current.Col}
	p.advance()
	elts := []Expression{}

	if p.current.Type != lexer.RSQB {
		elts = append(elts, p.parseExpression(LOWEST))

		for p.current.Type == lexer.COMMA {
			p.advance()

			if p.current.Type == lexer.RSQB {
				break
			}

			elts = append(elts, p.parseExpression(LOWEST))
		}
	}

	if p.current.Type != lexer.RSQB {
		panic(`expected ']' after list elements`)
	}

	endPos := Position{
		Line: p.current.Line,
		Col:  p.current.Col,
	}
	p.advance()
	return &List{Elts: elts, Pos: Range{Start: startPos, End: endPos}}
}

func (p *Parser) parseCall(funcExpr Expression) Expression {
	startPos := funcExpr.(*Name).Pos.Start
	p.advance()
	args := []Expression{}

	if p.current.Type != lexer.RPAR {
		args = append(args, p.parseExpression(LOWEST))

		for p.current.Type == lexer.COMMA {
			p.advance()
			args = append(args, p.parseExpression(LOWEST))
		}
	}

	if p.current.Type != lexer.RPAR {
		panic("expected ) after function arguments")
	}
	endPos := Position{Line: p.current.Line, Col: p.current.Col}
	p.advance()

	return &Call{
		Func: funcExpr,
		Args: args,
		Pos:  Range{Start: startPos, End: endPos},
	}
}
