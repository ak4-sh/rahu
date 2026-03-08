package parser

import (
	"fmt"

	l "rahu/lexer"
	a "rahu/parser/ast"
)

func (p *Parser) parseExpression(minBP int) a.Expression {
	left := p.parsePrimary()
	if left == nil {
		return nil
	}

	left = p.postfixParseLoop(left)

	for {
		bp := infixBindingPower(p.current.Type)
		if bp <= minBP {
			break
		}

		opTok := p.current

		if isCompareOp(opTok.Type) {
			startPos := left.Position().Start
			ops := []a.CompareOp{}
			rights := []a.Expression{}

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

			left = &a.Compare{
				Left:  left,
				Ops:   ops,
				Right: rights,
				Pos:   a.Range{Start: int(startPos), End: int(endPos)},
			}
			continue
		}

		if opTok.Type == l.AND || opTok.Type == l.OR {
			p.advance()
			right := p.parseExpression(bp)
			if right == nil {
				p.errorCurrent("expected expression after boolean operator")
				return left
			}

			if boolOp, ok := left.(*a.BooleanOp); ok {
				if (opTok.Type == l.AND && boolOp.Operator == a.And) || (opTok.Type == l.OR && boolOp.Operator == a.Or) {
					boolOp.Values = append(boolOp.Values, right)
					continue
				}
			}

			op := a.And
			if opTok.Type == l.OR {
				op = a.Or
			}
			left = &a.BooleanOp{
				Operator: op,
				Values:   []a.Expression{left, right},
				Pos: a.Range{
					Start: left.Position().Start,
					End:   right.Position().End,
				},
			}
			continue
		}
		p.advance()
		var right a.Expression

		if opTok.Type == l.DOUBLESTAR {
			right = p.parseExpression(bp - 1)
		} else {
			right = p.parseExpression(bp)
		}

		if right == nil {
			p.errorCurrent("expected expression after operator")
			return left
		}

		left = &a.BinOp{
			Left:  left,
			Op:    p.tokenTypeToOperator(opTok.Type),
			Right: right,
			Pos: a.Range{
				Start: left.Position().Start,
				End:   right.Position().End,
			},
		}
	}

	return left
}

func (p *Parser) parsePrimary() a.Expression {
	switch p.current.Type {
	case l.UNTERMINATED_STRING:
		p.errorCurrent("unterminated string literal")
		p.advance()
		return nil

	case l.NUMBER:
		n := &a.Number{
			Value: p.current.Literal,
			Pos: a.Range{
				Start: int(p.current.Start),
				End:   int(p.current.End),
			},
		}
		p.advance()
		return n
	case l.TRUE:
		ret := &a.Boolean{
			Value: true,
			Pos: a.Range{
				Start: int(p.current.Start),
				End:   int(p.current.End),
			},
		}
		p.advance()
		return ret

	case l.FALSE:
		ret := &a.Boolean{
			Value: false,
			Pos: a.Range{
				Start: int(p.current.Start),
				End:   int(p.current.End),
			},
		}
		p.advance()
		return ret
	case l.NAME:
		n := &a.Name{
			Text: p.current.Literal,
			Pos: a.Range{
				Start: int(p.current.Start),
				End:   int(p.current.End),
			},
			ID: p.newNodeID(),
		}
		p.advance()
		return n

	case l.LPAR:
		// TODO: need to add handling for tuples here
		startPos := p.current.Start
		p.advance()

		// empty tuple
		if p.current.Type == l.RPAR {
			endPos := p.current.Start
			p.advance()
			return &a.Tuple{Elts: []a.Expression{}, Pos: a.Range{Start: int(startPos), End: int(endPos)}}
		}

		first := p.parseExpression(LOWEST)
		if first == nil {
			return &a.Name{Text: "", Pos: a.Range{Start: int(startPos), End: p.currentRange().End}, ID: p.newNodeID()}
		}

		if p.current.Type != l.COMMA {
			p.advance() // expecting a RPAR now
			return first
		}

		elts := []a.Expression{first}
		for p.current.Type == l.COMMA {
			p.advance()

			if p.current.Type == l.RPAR {
				break
			}

			elt := p.parseExpression(LOWEST)
			if elt != nil {
				elts = append(elts, elt)
			}
		}

		endPos := p.current.Start
		p.advance()
		return &a.Tuple{Elts: elts, Pos: a.Range{Start: int(startPos), End: int(endPos)}}

	case l.LSQB:
		return p.parseList()
	case l.STRING:
		s := &a.String{
			Value: p.current.Literal,
			Pos: a.Range{
				Start: int(p.current.Start),
				End:   int(p.current.End),
			},
		}
		p.advance()
		return s

	case l.MINUS:
		startPos := p.current.Start
		p.advance()
		operand := p.parseExpression(PREFIX)
		if operand == nil {
			p.errorCurrent("expected expression after '-' ")
			return &a.Name{Text: "", Pos: a.Range{Start: int(startPos), End: p.currentRange().End}, ID: p.newNodeID()}
		}
		endPos := operand.Position().End
		return &a.UnaryOp{
			Op:      a.USub,
			Operand: operand,
			Pos:     a.Range{Start: int(startPos), End: int(endPos)},
		}

	case l.PLUS:
		startPos := p.current.Start
		p.advance()
		operand := p.parseExpression(PREFIX)
		if operand == nil {
			p.errorCurrent("expected expression after '+'")
			return &a.Name{Text: "", Pos: a.Range{Start: int(startPos), End: p.currentRange().End}, ID: p.newNodeID()}
		}
		endPos := operand.Position().End
		return &a.UnaryOp{
			Op:      a.UAdd,
			Operand: operand,
			Pos:     a.Range{Start: int(startPos), End: int(endPos)},
		}

	case l.NOT:
		startPos := p.current.Start
		p.advance()
		expr := p.parseExpression(PREFIX)
		if expr == nil {
			p.errorCurrent("expected expression after 'not'")
			return &a.Name{Text: "", Pos: a.Range{Start: int(startPos), End: p.currentRange().End}, ID: p.newNodeID()}
		}
		endPos := expr.Position().End
		return &a.UnaryOp{
			Op:      a.Not,
			Operand: expr,
			Pos:     a.Range{Start: int(startPos), End: int(endPos)},
		}

	case l.NONE:
		startPos := p.current.Start
		endPos := p.current.End
		p.advance()
		return &a.Name{
			Text: "None",
			Pos:  a.Range{Start: int(startPos), End: int(endPos)},
			ID:   p.newNodeID(),
		}

	}
	p.errorCurrent(fmt.Sprintf("unexpected token %v", p.current))
	p.advance()
	return nil
}

func (p *Parser) parseList() a.Expression {
	startPos := p.current.Start
	p.advance()
	elts := []a.Expression{}

	if p.current.Type != l.RSQB {
		first := p.parseExpression(LOWEST)
		if first != nil {
			elts = append(elts, first)
		} else {
			p.errorCurrent("expected expression in list")
		}

		for p.current.Type == l.COMMA {
			p.advance()

			if p.current.Type == l.RSQB {
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

	if p.current.Type != l.RSQB {
		p.errorCurrent("expected ']' after list elements")
		p.syncTo(l.RSQB, l.NEWLINE, l.EOF)
		if p.current.Type == l.RSQB {
			p.advance()
		}
		return &a.List{
			Elts: elts,
			Pos: a.Range{
				Start: int(startPos),
				End:   p.currentRange().End,
			},
		}
	}

	endPos := p.current.Start
	p.advance()
	return &a.List{Elts: elts, Pos: a.Range{Start: int(startPos), End: int(endPos)}}
}
