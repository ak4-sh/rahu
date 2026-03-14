package parser

import (
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

			leftId := p.tree.NewNode(
				a.NodeCompare,
				startPos,
				endPos,
			)
			p.tree.AddChild(leftId, left)
			for _, child := range rights {
				p.tree.AddChild(leftId, child)
			}

			left = leftId
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

		binOpId := p.tree.NewNode(a.NodeBinOp, p.tree.Nodes[left].Start, p.tree.Nodes[right].End)

		p.tree.Nodes[binOpId].Data = uint32(p.tokenTypeToOperator(opTok.Type))
		p.tree.AddChild(binOpId, left)
		p.tree.AddChild(binOpId, right)
		left = binOpId
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

func (p *Parser) parseAttribute(left a.NodeID) a.NodeID {
	panic("unimplemented")
}

func (p *Parser) parsePrimary() a.NodeID {
	panic("unimplemented")
}
