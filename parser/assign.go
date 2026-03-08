package parser

import (
	"rahu/lexer"
	a "rahu/parser/ast"
)

func (p *Parser) parseAugAssign() a.Statement {
	start := p.current.Start
	target := p.parsePrimary()
	op := p.current.Type
	p.advance() // consume op

	value := p.parseExpression(LOWEST)

	end := p.current.Start

	if p.current.Type == lexer.NEWLINE {
		p.advance()
	}

	return &a.AugAssign{
		Target: target,
		Op:     op,
		Value:  value,
		Pos:    a.Range{Start: int(start), End: int(end)},
	}
}

func (p *Parser) parseAssignment() a.Statement {
	start := p.current.Start
	targets := []a.Expression{}
	for {
		target := p.parseExpression(LOWEST)
		if target == nil {
			p.errorCurrent("expected assignment target")
			return &a.Assign{Targets: targets, Value: nil, Pos: a.Range{Start: int(start), End: int(p.current.Start)}}
		}
		switch target.(type) {
		case *a.Name, *a.Attribute, *a.Tuple, *a.List:
		default:
			p.error(target.Position(), "invalid assignment target")

		}
		targets = append(targets, target)
		if p.current.Type != lexer.COMMA {
			break
		}
		p.advance()
	}

	if p.current.Type != lexer.EQUAL {
		p.error(
			a.Range{
				Start: int(start),
				End:   int(p.current.Start),
			},
			"expected '=' in assignment",
		)
		p.syncTo(lexer.EQUAL, lexer.NEWLINE, lexer.EOF)
		if p.current.Type != lexer.EQUAL {
			return &a.Assign{Targets: targets, Value: nil, Pos: a.Range{Start: int(start), End: int(p.current.Start)}}
		}

	}

	p.advance()

	// moved past '='

	value := p.parseExpression(LOWEST)

	assgnEnd := p.current.Start

	if p.current.Type == lexer.NEWLINE {
		p.advance()
	} else if p.current.Type != lexer.EOF {
		p.error(
			a.Range{
				Start: int(assgnEnd),
				End:   int(p.current.End),
			},
			"expected newline after assignment",
		)
	}

	return &a.Assign{
		Targets: targets,
		Value:   value,
		Pos:     a.Range{Start: int(start), End: int(assgnEnd)},
	}
}
