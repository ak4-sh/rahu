package parser

import (
	"fmt"

	"rahu/lexer"
)

func (p *Parser) parseExpression(minBP int) Expression {
	left := p.parsePrimary()
	if left == nil {
		return nil
	}

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
				if right == nil {
					p.errorCurrent("expected expression after comparison operator")
					return left
				}
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
			if right == nil {
				p.errorCurrent("expected expression after boolean operator")
				return left
			}

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
		var right Expression

		if opTok.Type == lexer.DOUBLESTAR {
			right = p.parseExpression(bp - 1)
		} else {
			right = p.parseExpression(bp)
		}

		if right == nil {
			p.errorCurrent("expected expression after operator")
			return left
		}

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
	case lexer.UNTERMINATED_STRING:
		p.errorCurrent("unterminated string literal")
		p.advance()
		return nil

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
		// TODO: need to add handling for tuples here
		startPos := Position{Line: p.current.Line, Col: p.current.Col}
		p.advance()
		expr := p.parseExpression(LOWEST)
		if p.current.Type != lexer.RPAR {
			p.errorCurrent("expected ')' after expression")
			p.syncTo(lexer.RPAR, lexer.NEWLINE, lexer.EOF)
			if p.current.Type == lexer.RPAR {
				p.advance()
			}
			if expr == nil {
				return &Name{ID: "", Pos: Range{Start: startPos, End: p.currentRange().End}}
			}
			return expr
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
		if operand == nil {
			p.errorCurrent("expected expression after '-' ")
			return &Name{ID: "", Pos: Range{Start: startPos, End: p.currentRange().End}}
		}
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
		if operand == nil {
			p.errorCurrent("expected expression after '+'")
			return &Name{ID: "", Pos: Range{Start: startPos, End: p.currentRange().End}}
		}
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
		if expr == nil {
			p.errorCurrent("expected expression after 'not'")
			return &Name{ID: "", Pos: Range{Start: startPos, End: p.currentRange().End}}
		}
		endPos := expr.Position().End
		return &UnaryOp{
			Op:      Not,
			Operand: expr,
			Pos:     Range{Start: startPos, End: endPos},
		}

	case lexer.NONE:
		startPos := Position{Line: p.current.Line, Col: p.current.Col}
		endPos := Position{Line: p.current.Line, Col: p.current.EndCol}
		p.advance()
		return &Name{
			ID:  "None",
			Pos: Range{Start: startPos, End: endPos},
		}

	}
	p.errorCurrent(fmt.Sprintf("unexpected token %v", p.current))
	p.advance()
	return nil
}

func (p *Parser) parseList() Expression {
	startPos := Position{Line: p.current.Line, Col: p.current.Col}
	p.advance()
	elts := []Expression{}

	if p.current.Type != lexer.RSQB {
		first := p.parseExpression(LOWEST)
		if first != nil {
			elts = append(elts, first)
		} else {
			p.errorCurrent("expected expression in list")
		}

		for p.current.Type == lexer.COMMA {
			p.advance()

			if p.current.Type == lexer.RSQB {
				break
			}

			elt := p.parseExpression(LOWEST)
			if elt != nil {
				elts = append(elts, elt)
			} else {
				p.errorCurrent("expected expression after ',' in list")
				break
			}
		}
	}

	if p.current.Type != lexer.RSQB {
		p.errorCurrent("expected ']' after list elements")
		p.syncTo(lexer.RSQB, lexer.NEWLINE, lexer.EOF)
		if p.current.Type == lexer.RSQB {
			p.advance()
		}
		return &List{Elts: elts, Pos: Range{Start: startPos, End: p.currentRange().End}}
	}

	endPos := Position{
		Line: p.current.Line,
		Col:  p.current.Col,
	}
	p.advance()
	return &List{Elts: elts, Pos: Range{Start: startPos, End: endPos}}
}

func (p *Parser) parseCall(funcExpr Expression) Expression {
	var startPos Position
	if name, ok := funcExpr.(*Name); ok {
		startPos = name.Pos.Start
	} else {
		startPos = funcExpr.Position().Start
	}

	p.advance()
	args := []Expression{}

	if p.current.Type != lexer.RPAR {
		first := p.parseExpression(LOWEST)
		if first == nil {
			p.syncTo(lexer.RPAR, lexer.NEWLINE, lexer.EOF)
			if p.current.Type == lexer.RPAR {
				p.advance()
			}
			return &Call{
				Func: funcExpr,
				Args: args,
				Pos: Range{
					Start: startPos,
					End:   p.currentRange().End,
				},
			}
		}
		args = append(args, first)

		for p.current.Type == lexer.COMMA {
			p.advance() // consume ','

			// trailing comma: foo(a, b,)
			if p.current.Type == lexer.RPAR {
				break
			}

			arg := p.parseExpression(LOWEST)
			if arg == nil {
				p.syncTo(lexer.RPAR, lexer.NEWLINE, lexer.EOF)
				if p.current.Type == lexer.RPAR {
					p.advance()
				}
				return &Call{
					Func: funcExpr,
					Args: args,
					Pos: Range{
						Start: startPos,
						End:   p.currentRange().End,
					},
				}
			}
			args = append(args, arg)
		}
	}

	if p.current.Type != lexer.RPAR {
		p.errorCurrent("expected ')' after function arguments")
		p.syncTo(lexer.RPAR, lexer.NEWLINE, lexer.EOF)
		if p.current.Type == lexer.RPAR {
			p.advance()
		}
		endPos := p.currentRange().End
		return &Call{
			Func: funcExpr,
			Args: args,
			Pos:  Range{Start: startPos, End: endPos},
		}
	}
	endPos := Position{Line: p.current.Line, Col: p.current.Col}
	p.advance()

	return &Call{
		Func: funcExpr,
		Args: args,
		Pos:  Range{Start: startPos, End: endPos},
	}
}
