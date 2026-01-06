package parser

import (
	"fmt"
	"rahu/lexer"
)

const (
	LOWEST = iota
	SUM
	PRODUCT
	PREFIX
)

type Parser struct {
	lexer   *lexer.Lexer
	current lexer.Token
	peek    lexer.Token
}

func New(input string) *Parser {
	l := lexer.New(input)
	p := &Parser{
		lexer: l,
	}

	p.current = l.NextToken()
	p.peek = l.NextToken()
	return p
}

func (p *Parser) advance() {
	p.current = p.peek
	p.peek = p.lexer.NextToken()
}

func (p *Parser) advanceBy(count int) {
	for range count {
		p.advance()
	}
}

func (p *Parser) Parse() *Module {
	statements := []Statement{}
	for p.current.Type != lexer.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			statements = append(statements, stmt)
		}
	}
	return &Module{Body: statements}
}

func (p *Parser) parseStatement() Statement {
	if p.current.Type == lexer.INDENT || p.current.Type == lexer.DEDENT{
		p.advance()
	}
	if p.current.Type == lexer.NAME && p.peek.Type == lexer.EQUAL {
		return p.parseAssignment()
	}

	p.advance()
	return nil
}

func (p *Parser) parseAssignment() Statement {
	target := &Name{Id: p.current.Literal}
	// moving past name and equal
	p.advanceBy(2)

	value := p.parseExpression(LOWEST)

	if p.current.Type == lexer.NEWLINE {
		p.advance()
	}

	return &Assign{
		Targets: []Expression{target},
		Value:   value,
	}
}

func (p *Parser) parseExpression(minBP int) Expression{
	left := p.parsePrimary()
	for {
		bp := infixBindingPower(p.current.Type)
		if bp <= minBP{
			break
		}

		opTok := p.current
		p.advance()

		right := p.parseExpression(bp)

		left = &BinOp{
			Left:  left,
			Op:  p.tokenTypeToOperator(opTok.Type),
			Right:  right,
		}
	}

	return left
}

func infixBindingPower(t lexer.TokenType) int{
	switch t{
	case lexer.PLUS, lexer.MINUS:
		return SUM
	case lexer.STAR, lexer.SLASH, lexer.DOUBLESLASH:
		return PRODUCT
	default:
		return LOWEST
	}
}

func (p *Parser) parsePrimary() Expression {
	switch p.current.Type{
	case lexer.NUMBER:
			n := &Number{Value:  p.current.Literal}
			p.advance()
			return n
		case lexer.NAME:
			n := &Name{Id: p.current.Literal}
			p.advance()
			return n
		case lexer.LPAR:
			p.advance()
			expr := p.parseExpression(LOWEST)
			if p.current.Type != lexer.RPAR{
				panic("expected )")
			}
			p.advance()
			return expr
	}
	panic(fmt.Sprintf("unexpected token %v\n", p.current))
}

func (p *Parser) isOperator(t lexer.TokenType) bool {
	return t == lexer.PLUS || t == lexer.MINUS || t == lexer.STAR || t == lexer.SLASH || t == lexer.DOUBLESLASH
}

func (p *Parser) tokenTypeToOperator(t lexer.TokenType) Operator {
	switch t {
	case lexer.PLUS:
		return Add
	case lexer.MINUS:
		return Sub
	case lexer.STAR:
		return Mult
	case lexer.SLASH:
		return Div
	case lexer.DOUBLESLASH:
		return FloorDiv
	case lexer.PERCENT:
		return Mod
	default:
		return Add
	}
}
