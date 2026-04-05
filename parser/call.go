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
	callID := p.tree.NewNode(a.NodeCall, startPos, startPos)
	p.tree.AddChild(callID, funcExpr)

	p.advance()

	if p.current.Type != l.RPAR {
		first := p.parseExpression(LOWEST)
		if first == a.NoNode {
			p.syncTo(l.RPAR, l.NEWLINE, l.EOF)
			end := p.current.End
			if p.current.Type == l.RPAR {
				end = p.current.Start
				p.advance()
			}
			p.tree.Nodes[callID].End = end
			return callID
		}

		p.tree.AddChild(callID, first)
		p.tree.Nodes[callID].End = p.tree.Nodes[first].End

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
				p.tree.Nodes[callID].End = end
				return callID
			}
			p.tree.AddChild(callID, arg)
			p.tree.Nodes[callID].End = p.tree.Nodes[arg].End
		}
		if p.current.Type != l.RPAR {
			p.errorCurrent("expected ')' after function arguments")
			p.syncTo(l.RPAR, l.NEWLINE, l.EOF)
			endPos := p.current.End
			if p.current.Type == l.RPAR {
				endPos = p.current.Start
				p.advance()
			}

			p.tree.Nodes[callID].End = endPos
			return callID
		}
	}
	endPos := p.current.Start
	p.advance()

	p.tree.Nodes[callID].End = endPos
	return callID
}
