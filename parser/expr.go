package parser

import (
	"fmt"

	l "rahu/lexer"
	a "rahu/parser/ast"
)

func (p *Parser) parseExpression(minBP int) a.NodeID {
	left := p.parsePrimary()
	if left == a.NoNode {
		return a.NoNode
	}

	left = p.postfixParseLoop(left)

	for {
		bp := infixBindingPower(p.current.Type)
		if bp <= minBP {
			break
		}

		opTok := p.current

		if isCompareOp(opTok.Type) {
			startPos := p.tree.Nodes[left].Start
			rights := make([]a.NodeID, 0)

			for isCompareOp(p.current.Type) {
				op := tokenTypeToCompareOp(p.current.Type)
				p.advance()

				right := p.parseExpression(COMPARE + 1)
				if right == a.NoNode {
					p.errorCurrent("expected expression after comparison operator")
					return left
				}
				rightCompareOp := p.tree.NewNode(a.NodeCompareOp, p.tree.Nodes[right].Start, p.tree.Nodes[right].End)
				p.tree.Nodes[rightCompareOp].Data = uint32(op)
				p.tree.AddChild(rightCompareOp, right)
				rights = append(rights, rightCompareOp)
			}
			lastRight := rights[len(rights)-1]
			endPos := p.tree.Nodes[lastRight].End

			leftID := p.tree.NewNode(
				a.NodeCompare,
				startPos,
				endPos,
			)
			p.tree.AddChild(leftID, left)
			for _, child := range rights {
				p.tree.AddChild(leftID, child)
			}

			left = leftID
			continue
		}

		if opTok.Type == l.AND || opTok.Type == l.OR {
			p.advance()
			right := p.parseExpression(bp)
			if right == a.NoNode {
				p.errorCurrent("expected expression after boolean operator")
				return left
			}

			if p.tree.Nodes[left].Kind == a.NodeBooleanOp {
				if opTok.Type == l.AND && a.BooleanOperator(p.tree.Nodes[left].Data) == a.And || opTok.Type == l.OR && a.BooleanOperator(p.tree.Nodes[left].Data) == a.Or {
					p.tree.AddChild(left, right)
					continue
				}
			}

			op := a.And
			if opTok.Type == l.OR {
				op = a.Or
			}

			booleanID := p.tree.NewNode(a.NodeBooleanOp, p.tree.Nodes[left].Start, p.tree.Nodes[right].End)
			p.tree.Nodes[booleanID].Data = uint32(op)

			p.tree.AddChild(booleanID, left)
			p.tree.AddChild(booleanID, right)
			left = booleanID
			continue
		}

		p.advance()

		var right a.NodeID

		if opTok.Type == l.DOUBLESTAR {
			right = p.parseExpression(bp - 1)
		} else {
			right = p.parseExpression(bp)
		}

		if right == a.NoNode {
			p.errorCurrent("expected expression after operator")
			return left
		}

		binOpID := p.tree.NewNode(a.NodeBinOp, p.tree.Nodes[left].Start, p.tree.Nodes[right].End)

		p.tree.Nodes[binOpID].Data = uint32(p.tokenTypeToOperator(opTok.Type))
		p.tree.AddChild(binOpID, left)
		p.tree.AddChild(binOpID, right)
		left = binOpID
	}
	return left
}

func (p *Parser) postfixParseLoop(left a.NodeID) a.NodeID {
	for {
		switch p.current.Type {
		case l.LPAR:
			left = p.parseCall(left)

		case l.DOT:
			left = p.parseAttribute(left)
		default:
			return left
		}
	}
}

func (p *Parser) parsePrimary() a.NodeID {
	switch p.current.Type {
	case l.UNTERMINATED_STRING:
		p.errorCurrent("unterminated string literal")
		p.advance()
		return a.NoNode

	case l.NUMBER:
		n := p.tree.NewNode(a.NodeNumber, p.current.Start, p.current.End)
		idx := uint32(len(p.tree.Numbers))
		p.tree.Numbers = append(p.tree.Numbers, p.current.Literal)
		p.tree.Nodes[n].Data = idx
		p.advance()
		return n

	case l.TRUE:
		ret := p.tree.NewNode(a.NodeBoolean, p.current.Start, p.current.End)
		p.tree.Nodes[ret].Data = uint32(a.TRUE)
		p.advance()
		return ret

	case l.FALSE:
		ret := p.tree.NewNode(a.NodeBoolean, p.current.Start, p.current.End)
		p.tree.Nodes[ret].Data = uint32(a.FALSE)
		p.advance()
		return ret

	case l.NAME:
		ret := p.tree.NewNode(a.NodeName, p.current.Start, p.current.End)
		idx := uint32(len(p.tree.Names))
		p.tree.Names = append(p.tree.Names, p.current.Literal)
		p.tree.Nodes[ret].Data = idx
		p.advance()
		return ret

	case l.LPAR:
		startPos := p.current.Start
		p.advance()

		// empty tuple
		if p.current.Type == l.RPAR {
			endPos := p.current.Start
			p.advance()
			return p.tree.NewNode(a.NodeTuple, startPos, endPos)
		}

		first := p.parseExpression(LOWEST)
		if first == a.NoNode {
			return p.tree.NewNode(a.NodeErrExp, startPos, p.current.End)
		}

		if p.current.Type != l.COMMA {
			if p.current.Type != l.RPAR {
				p.errorCurrent("expected ')' after expression")
				return p.tree.NewNode(a.NodeErrExp, startPos, p.current.End)
			}

			p.advance()
			return first
		}

		elts := []a.NodeID{first}
		for p.current.Type == l.COMMA {
			p.advance()
			if p.current.Type == l.RPAR {
				break
			}

			elt := p.parseExpression(LOWEST)
			if elt == a.NoNode {
				p.errorCurrent("expected expression after ','")
				return p.tree.NewNode(a.NodeErrExp, startPos, p.current.End)
			}
			elts = append(elts, elt)
		}

		if p.current.Type != l.RPAR {
			p.errorCurrent("expected ')' after tuple")
			return p.tree.NewNode(a.NodeErrExp, startPos, p.current.End)
		}

		endPos := p.current.Start
		p.advance()
		ret := p.tree.NewNode(a.NodeTuple, startPos, endPos)
		for _, child := range elts {
			p.tree.AddChild(ret, child)
		}
		return ret

	case l.LSQB:
		return p.parseList()

	case l.STRING:
		idx := uint32(len(p.tree.Strings))
		p.tree.Strings = append(p.tree.Strings, p.current.Literal)
		ret := p.tree.NewNode(a.NodeString, p.current.Start, p.current.End)
		p.tree.Nodes[ret].Data = idx
		p.advance()
		return ret

	case l.MINUS:
		startPos := p.current.Start
		p.advance()
		operand := p.parseExpression(PREFIX)
		if operand == a.NoNode {
			p.errorCurrent("expected expression after '-'")
			return p.tree.NewNode(a.NodeErrExp, startPos, p.current.End)
		}
		endPos := p.tree.Nodes[operand].End
		ret := p.tree.NewNode(a.NodeUnaryOp, startPos, endPos)
		p.tree.Nodes[ret].Data = uint32(a.USub)
		p.tree.AddChild(ret, operand)
		return ret

	case l.PLUS:
		startPos := p.current.Start
		p.advance()
		operand := p.parseExpression(PREFIX)
		if operand == a.NoNode {
			p.errorCurrent("expected expression after '+'")
			return p.tree.NewNode(a.NodeErrExp, startPos, p.current.End)
		}

		endPos := p.tree.Nodes[operand].End
		ret := p.tree.NewNode(a.NodeUnaryOp, startPos, endPos)
		p.tree.Nodes[ret].Data = uint32(a.UAdd)
		p.tree.AddChild(ret, operand)
		return ret

	case l.NOT:
		startPos := p.current.Start
		p.advance()
		expr := p.parseExpression(PREFIX)
		if expr == a.NoNode {
			p.errorCurrent("expected expression after 'not'")
			return p.tree.NewNode(a.NodeErrExp, startPos, p.current.End)
		}

		endpos := p.tree.Nodes[expr].End
		ret := p.tree.NewNode(a.NodeUnaryOp, startPos, endpos)
		p.tree.Nodes[ret].Data = uint32(a.Not)
		p.tree.AddChild(ret, expr)
		return ret

	case l.NONE:
		startPos := p.current.Start
		endPos := p.current.End
		p.advance()
		ret := p.tree.NewNode(a.NodeNone, startPos, endPos)
		return ret
	}
	p.errorCurrent(fmt.Sprintf("unexpected token %v", p.current))
	p.advance()
	return a.NoNode
}

func (p *Parser) parseList() a.NodeID {
	startPos := p.current.Start
	p.advance()
	elts := []a.NodeID{}

	if p.current.Type != l.RSQB {
		first := p.parseExpression(LOWEST)
		if first != a.NoNode {
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
			if elt != a.NoNode {
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
		endPos := p.current.Start

		if p.current.Type == l.RSQB {
			endPos = p.current.Start
			p.advance()
		}

		ret := p.tree.NewNode(a.NodeList, startPos, endPos)
		for _, child := range elts {
			p.tree.AddChild(ret, child)
		}
		return ret
	}

	endPos := p.current.Start
	p.advance()
	ret := p.tree.NewNode(a.NodeList, startPos, endPos)
	for _, child := range elts {
		p.tree.AddChild(ret, child)
	}
	return ret
}
