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
	left = p.parseAdjacentStringLiterals(left)

	left = p.postfixParseLoop(left)

	for {
		bp := infixBindingPower(p.current.Type)
		if isCompareOp(p.current.Type, p.peek.Type) {
			bp = COMPARE
		}
		if bp <= minBP {
			break
		}

		opTok := p.current

		if isCompareOp(opTok.Type, p.peek.Type) {
			startPos := p.tree.Nodes[left].Start
			leftID := p.tree.NewNode(a.NodeCompare, startPos, p.tree.Nodes[left].End)
			p.tree.AddChild(leftID, left)

			for isCompareOp(p.current.Type, p.peek.Type) {
				op := tokenTypesToCompareOp(p.current.Type, p.peek.Type)
				p.advanceBy(compareOpTokenWidth(p.current.Type, p.peek.Type))

				right := p.parseExpression(COMPARE + 1)
				if right == a.NoNode {
					p.errorCurrent("expected expression after comparison operator")
					return left
				}
				rightCompareOp := p.tree.NewNode(a.NodeCompareOp, p.tree.Nodes[right].Start, p.tree.Nodes[right].End)
				p.tree.Nodes[rightCompareOp].Data = uint32(op)
				p.tree.AddChild(rightCompareOp, right)
				p.tree.AddChild(leftID, rightCompareOp)
				p.tree.Nodes[leftID].End = p.tree.Nodes[rightCompareOp].End
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

		// Handle conditional expression: X if C else Y
		if opTok.Type == l.IF {
			p.advance() // consume 'if'
			condition := p.parseExpression(bp)
			if condition == a.NoNode {
				p.errorCurrent("expected condition after 'if' in conditional expression")
				return left
			}

			if p.current.Type != l.ELSE {
				p.errorCurrent("expected 'else' after condition in conditional expression")
				return left
			}
			p.advance() // consume 'else'

			falseExpr := p.parseExpression(bp)
			if falseExpr == a.NoNode {
				p.errorCurrent("expected expression after 'else' in conditional expression")
				return left
			}

			condID := p.tree.NewNode(a.NodeConditional, p.tree.Nodes[left].Start, p.tree.Nodes[falseExpr].End)
			p.tree.AddChild(condID, condition) // condition
			p.tree.AddChild(condID, left)      // true expression
			p.tree.AddChild(condID, falseExpr) // false expression
			left = condID
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

func (p *Parser) parseAdjacentStringLiterals(left a.NodeID) a.NodeID {
	for left != a.NoNode && (p.tree.Node(left).Kind == a.NodeString || p.tree.Node(left).Kind == a.NodeFString) {
		if p.current.Type != l.STRING && p.current.Type != l.FSTRING {
			return left
		}
		right := p.parsePrimary()
		if right == a.NoNode {
			return left
		}
		left = p.mergeStringLiterals(left, right)
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

		case l.LSQB:
			left = p.parseSubscript(left)
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
		ret := p.tree.NewNameNode(p.current.Start, p.current.End, p.current.Literal)
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

		// Check for generator expression: (expr for target in iter)
		if p.current.Type == l.FOR {
			return p.parseGeneratorExp(startPos, first)
		}

		if p.current.Type != l.COMMA {
			if p.current.Type != l.RPAR {
				p.errorCurrent("expected ')' after expression")
				return p.tree.NewNode(a.NodeErrExp, startPos, p.current.End)
			}

			p.advance()
			return first
		}

		ret := p.tree.NewNode(a.NodeTuple, startPos, p.tree.Nodes[first].End)
		p.tree.AddChild(ret, first)
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
			p.tree.AddChild(ret, elt)
			p.tree.Nodes[ret].End = p.tree.Nodes[elt].End
		}

		if p.current.Type != l.RPAR {
			p.errorCurrent("expected ')' after tuple")
			return p.tree.NewNode(a.NodeErrExp, startPos, p.current.End)
		}

		endPos := p.current.Start
		p.advance()
		p.tree.Nodes[ret].End = endPos
		return ret

	case l.LSQB:
		return p.parseList()

	case l.LBRACE:
		return p.parseDict()

	case l.STRING:
		idx := uint32(len(p.tree.Strings))
		p.tree.Strings = append(p.tree.Strings, p.current.Literal)
		ret := p.tree.NewNode(a.NodeString, p.current.Start, p.current.End)
		p.tree.Nodes[ret].Data = idx
		p.advance()
		return ret

	case l.BSTRING:
		idx := uint32(len(p.tree.Bytes))
		p.tree.Bytes = append(p.tree.Bytes, p.current.Literal)
		ret := p.tree.NewNode(a.NodeBytes, p.current.Start, p.current.End)
		p.tree.Nodes[ret].Data = idx
		p.advance()
		return ret

	case l.FSTRING:
		return p.parseFString()

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

	case l.YIELD:
		startPos := p.current.Start
		p.advance()
		ret := p.tree.NewNode(a.NodeYield, startPos, startPos)
		if p.current.Type == l.FROM {
			p.tree.Nodes[ret].Data = 1
			p.advance()
		}
		if p.current.Type == l.NEWLINE || p.current.Type == l.EOF || p.current.Type == l.COMMA || p.current.Type == l.RPAR || p.current.Type == l.RSQB || p.current.Type == l.COLON {
			return ret
		}
		expr := p.parseExpression(LOWEST)
		if expr == a.NoNode {
			if p.tree.Nodes[ret].Data == 1 {
				p.errorCurrent("expected expression after 'yield from'")
			} else {
				p.errorCurrent("expected expression after 'yield'")
			}
			return ret
		}
		p.tree.AddChild(ret, expr)
		p.tree.Nodes[ret].End = p.tree.Nodes[expr].End
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
	ret := p.tree.NewNode(a.NodeList, startPos, startPos)

	if p.current.Type != l.RSQB {
		first := p.parseExpression(LOWEST)
		if first != a.NoNode {
			if p.current.Type == l.FOR {
				return p.parseListComp(startPos, first)
			}
			p.tree.AddChild(ret, first)
			p.tree.Nodes[ret].End = p.tree.Nodes[first].End
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
				p.tree.AddChild(ret, elt)
				p.tree.Nodes[ret].End = p.tree.Nodes[elt].End
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

		p.tree.Nodes[ret].End = endPos
		return ret
	}

	endPos := p.current.Start
	p.advance()
	p.tree.Nodes[ret].End = endPos
	return ret
}

func (p *Parser) parseListComp(startPos uint32, expr a.NodeID) a.NodeID {
	ret := p.tree.NewNode(a.NodeListComp, startPos, p.tree.Nodes[expr].End)
	p.tree.AddChild(ret, expr)
	for p.current.Type == l.FOR {
		clause := p.parseComprehensionClause()
		if clause == a.NoNode {
			return p.tree.NewNode(a.NodeErrExp, startPos, p.current.End)
		}
		p.tree.AddChild(ret, clause)
		p.tree.Nodes[ret].End = p.tree.Nodes[clause].End
	}
	if p.current.Type != l.RSQB {
		p.errorCurrent("expected ']' after list comprehension")
		return p.tree.NewNode(a.NodeErrExp, startPos, p.current.End)
	}
	endPos := p.current.Start
	p.advance()
	p.tree.Nodes[ret].End = endPos
	return ret
}

func (p *Parser) parseGeneratorExp(startPos uint32, expr a.NodeID) a.NodeID {
	ret := p.tree.NewNode(a.NodeGeneratorExp, startPos, p.tree.Nodes[expr].End)
	p.tree.AddChild(ret, expr)
	for p.current.Type == l.FOR {
		clause := p.parseComprehensionClause()
		if clause == a.NoNode {
			return p.tree.NewNode(a.NodeErrExp, startPos, p.current.End)
		}
		p.tree.AddChild(ret, clause)
		p.tree.Nodes[ret].End = p.tree.Nodes[clause].End
	}
	if p.current.Type != l.RPAR {
		p.errorCurrent("expected ')' after generator expression")
		return p.tree.NewNode(a.NodeErrExp, startPos, p.current.End)
	}
	endPos := p.current.Start
	p.advance()
	p.tree.Nodes[ret].End = endPos
	return ret
}

func (p *Parser) parseComprehensionClause() a.NodeID {
	startPos := p.current.Start
	p.advance()
	ret := p.tree.NewNode(a.NodeComprehension, startPos, startPos)
	target := p.parseForTarget()
	if target == a.NoNode {
		p.errorCurrent("invalid expression for comprehension target")
		return a.NoNode
	}
	p.tree.AddChild(ret, target)
	if p.current.Type != l.IN {
		p.errorCurrent("expected 'in' after comprehension target")
		return ret
	}
	p.advance()
	// Use IF+1 as min precedence to prevent conditional expressions from consuming the IF
	// (comprehension filters use bare IF, not conditional IF-ELSE)
	iter := p.parseExpression(IF + 1)
	if iter == a.NoNode {
		p.errorCurrent("invalid expression for comprehension iterator")
		return ret
	}
	p.tree.AddChild(ret, iter)
	p.tree.Nodes[ret].End = p.tree.Nodes[iter].End
	for p.current.Type == l.IF {
		p.advance()
		// Use IF+1 to prevent nested if from being parsed as conditional expression
		filter := p.parseExpression(IF + 1)
		if filter == a.NoNode {
			p.errorCurrent("invalid expression for comprehension filter")
			return ret
		}
		p.tree.AddChild(ret, filter)
		p.tree.Nodes[ret].End = p.tree.Nodes[filter].End
	}
	return ret
}
