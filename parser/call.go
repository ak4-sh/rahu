package parser

import (
	"rahu/lexer"
	a "rahu/parser/ast"
)

func (p *Parser) parseCall(funcExpr a.Expression) a.Expression {
	var startPos int
	if name, ok := funcExpr.(*a.Name); ok {
		startPos = name.Pos.Start
	} else {
		startPos = funcExpr.Position().Start
	}

	p.advance()
	args := []a.Expression{}

	if p.current.Type != lexer.RPAR {
		first := p.parseExpression(LOWEST)
		if first == nil {
			p.syncTo(lexer.RPAR, lexer.NEWLINE, lexer.EOF)
			if p.current.Type == lexer.RPAR {
				p.advance()
			}
			return &a.Call{
				Func: funcExpr,
				Args: args,
				Pos: a.Range{
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
				return &a.Call{
					Func: funcExpr,
					Args: args,
					Pos: a.Range{
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
		return &a.Call{
			Func: funcExpr,
			Args: args,
			Pos:  a.Range{Start: startPos, End: endPos},
		}
	}
	endPos := p.current.Start
	p.advance()

	return &a.Call{
		Func: funcExpr,
		Args: args,
		Pos:  a.Range{Start: startPos, End: endPos},
	}
}
