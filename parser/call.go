package parser

import (
	l "rahu/lexer"
	a "rahu/parser/ast"
)

func (p *Parser) parseCall(funcExpr a.NodeID) a.NodeID {
	if funcExpr == a.NoNode {
		return funcExpr
	}
	startPos := p.tree.Nodes[funcExpr].Start

	p.advance()

	args := make([]a.NodeID, 0)
	if p.current.Type != l.RPAR {
		first := p.parseExpression(LOWEST)
		if first == a.NoNode {
			p.syncTo(l.RPAR, l.NEWLINE, l.EOF)
			end := p.current.End
			if p.current.Type == l.RPAR {
				end = p.current.Start
				p.advance()
			}
			callID := p.tree.NewNode(a.NodeCall, startPos, end)
			p.tree.AddChild(callID, funcExpr)
			return callID
		}

		args = append(args, first)

		for p.current.Type == l.COMMA {
			p.advance()

			if p.current.Type == l.RPAR {
				break
			}
			arg := p.parseExpression(LOWEST)

			if arg == a.NoNode {
				p.syncTo(l.RPAR, l.NEWLINE, l.EOF)
				end := p.current.End
				if p.current.Type == l.RPAR {
					end = p.current.Start
					p.advance()
				}
				callID := p.tree.NewNode(a.NodeCall, startPos, end)
				p.tree.AddChild(callID, funcExpr)
				for _, child := range args {
					p.tree.AddChild(callID, child)
				}
				return callID
			}
			args = append(args, arg)
		}
		if p.current.Type != l.RPAR {
			p.errorCurrent("expected ')' after function arguments")
			p.syncTo(l.RPAR, l.NEWLINE, l.EOF)
			endPos := p.current.End
			if p.current.Type == l.RPAR {
				endPos = p.current.Start
				p.advance()
			}

			callID := p.tree.NewNode(a.NodeCall, startPos, endPos)
			p.tree.AddChild(callID, funcExpr)
			for _, child := range args {
				p.tree.AddChild(callID, child)
			}
			return callID
		}
	}
	endPos := p.current.Start
	p.advance()

	callID := p.tree.NewNode(a.NodeCall, startPos, endPos)
	p.tree.AddChild(callID, funcExpr)
	for _, child := range args {
		p.tree.AddChild(callID, child)
	}
	return callID
}
