package parser

import (
	"fmt"
	"rahu/lexer"
)

const (
	LOWEST = iota
	OR
	AND
	NOT
	COMPARE
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
	for p.current.Type == lexer.INDENT || p.current.Type == lexer.DEDENT {
		p.advance()
	}
	if p.current.Type == lexer.NAME && p.peek.Type == lexer.EQUAL {
		return p.parseAssignment()
	}

	if p.current.Type == lexer.IF {
		// parsing conditional
		p.advance()
		return p.parseIf()
	}

	if p.current.Type == lexer.NAME || p.current.Type == lexer.NUMBER ||
		p.current.Type == lexer.STRING {
		expr := p.parseExpression(LOWEST)
		if p.current.Type == lexer.NEWLINE {
			p.advance()
		}
		return &ExprStmt{Value: expr}
	}

	if p.current.Type == lexer.DEF {
		p.advance()
		funcdef := p.parseFunc()
		return funcdef
	}

	if p.current.Type == lexer.RETURN {
		p.advance()
		return p.parseReturn()
	}

	if p.current.Type == lexer.FOR {
		p.advance()
		return p.parseFor()
	}

	p.advance()
	return nil
}

func (p *Parser) parseForTarget() Expression {
	if p.current.Type != lexer.NAME {
		panic(`expected variable name`)
	}

	first := &Name{Id: p.current.Literal}
	p.advance()

	if p.current.Type == lexer.COMMA {
		targets := []Expression{first}

		for p.current.Type == lexer.COMMA {
			p.advance()

			if p.current.Type != lexer.NAME {
				panic(`expected variable name`)
			}
			targets = append(targets, &Name{Id: p.current.Literal})
			p.advance()
		}
		return &Tuple{Elts: targets}
	}
	return first
}

func (p *Parser) parseFor() Statement {
	forStmt := &For{}
	forStmt.Target = p.parseForTarget()

	if p.current.Type != lexer.IN {
		panic(`expected 'in' after loop variable`)
	}

	p.advance()

	forStmt.Iter = p.parseExpression(LOWEST)

	if p.current.Type != lexer.COLON {
		fmt.Printf("%vfailed for token v\n", p.current)
		panic(`expected ':' after for clause`)
	}
	p.advance()

	if p.current.Type != lexer.NEWLINE {
		panic(`expected newline after ':'`)
	}

	p.advance()

	if p.current.Type != lexer.INDENT {
		panic(`expected indent after for statement`)
	}

	p.advance()

	body := []Statement{}

	for p.current.Type != lexer.DEDENT && p.current.Type != lexer.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			body = append(body, stmt)
		}
	}

	forStmt.Body = body

	if p.current.Type == lexer.DEDENT {
		p.advance()
	}
	return forStmt
}

// return already consumed while receiving
func (p *Parser) parseReturn() Statement {
	if p.current.Type == lexer.NEWLINE {
		p.advance()
		return &Return{Value: nil}
	}

	value := p.parseExpression(LOWEST)

	if p.current.Type == lexer.NEWLINE {
		p.advance()
	}

	return &Return{Value: value}
}

func (p *Parser) parseFunc() Statement {
	funcDef := &FunctionDef{}

	if p.current.Type != lexer.NAME {
		fmt.Printf("%v\n", p.current)
		panic(`expected function name after "def"`)
	}

	funcDef.Name = p.current.Literal
	p.advance() // advance past func name

	if p.current.Type != lexer.LPAR {
		panic(`expected '(' after function name`)
	}

	p.advance() // advanced past left parantheses

	args := []string{}

	if p.current.Type != lexer.RPAR {
		if p.current.Type != lexer.NAME {
			panic("expected param name")
		}
		args = append(args, p.current.Literal)
		p.advance()
		for p.current.Type == lexer.COMMA {
			p.advance()

			if p.current.Type != lexer.NAME {
				panic("expected param name after comma")
			}
			args = append(args, p.current.Literal)
			p.advance()
		}
	}

	funcDef.Args = args

	if p.current.Type != lexer.RPAR {
		panic(`expected ')' after params`)
	}

	p.advance()

	if p.current.Type != lexer.COLON {
		panic(`expected ':' after function signature`)
	}
	p.advance()

	if p.current.Type != lexer.NEWLINE {
		panic(`expected newline after ':'`)
	}

	p.advance()

	if p.current.Type != lexer.INDENT {
		panic(`expected indentation after function signature`)
	}
	p.advance()

	funcBody := []Statement{}

	for p.current.Type != lexer.DEDENT && p.current.Type != lexer.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			funcBody = append(funcBody, stmt)
		}
	}

	if p.current.Type == lexer.DEDENT {
		p.advance() // skip past dedent
	}

	funcDef.Body = funcBody

	return funcDef
}

func (p *Parser) parseIf() Statement {

	ifExpr := &If{}
	testCond := p.parseExpression(LOWEST)

	ifExpr.Test = testCond

	if p.current.Type != lexer.COLON {
		fmt.Printf("panicked with curr type : %v\n", p.current)
		panic(`expected ':' after if condition %v`)
	}

	p.advance() // skip colon

	if p.current.Type != lexer.NEWLINE {
		panic(`expected newline after ":"`)
	}
	p.advance() // moving ahead of newline

	if p.current.Type != lexer.INDENT {
		panic("expected indentation block after if condtion")
	}
	p.advance() // moving ahead of indent

	body := []Statement{}

	for p.current.Type != lexer.DEDENT && p.current.Type != lexer.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			body = append(body, stmt)
		}
	}

	ifExpr.Body = body

	if p.current.Type == lexer.DEDENT {
		p.advance() // skip past dedent
	}

	orelse := []Statement{}
	if p.current.Type == lexer.ELIF {
		p.advance()
		elifStmt := p.parseIf()
		orelse = append(orelse, elifStmt)
	} else if p.current.Type == lexer.ELSE {
		p.advance()

		if p.current.Type != lexer.COLON {
			panic(`expected ':' after else`)
		}

		p.advance() // advancing past 'else' block

		if p.current.Type != lexer.NEWLINE {
			panic(`expected NEWLINE after 'else:'`)
		}

		p.advance() // move past newline

		if p.current.Type != lexer.INDENT {
			panic(`expected indent block after "else:"`)
		}

		p.advance() // move past indent

		for p.current.Type != lexer.DEDENT && p.current.Type != lexer.EOF {
			stmt := p.parseStatement()
			if stmt != nil {
				orelse = append(orelse, stmt)
			}
		}

		if p.current.Type == lexer.DEDENT {
			p.advance() // advance past dedent
		}
	}

	ifExpr.Orelse = orelse

	return ifExpr
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

func (p *Parser) parseExpression(minBP int) Expression {
	left := p.parsePrimary()

	if minBP <= COMPARE && isCompareOp(p.current.Type) {
		ops := []CompareOp{}
		rights := []Expression{}

		for isCompareOp(p.current.Type) {
			op := tokenTypeToCompareOp(p.current.Type)
			p.advance()

			right := p.parseExpression(COMPARE + 1)
			ops = append(ops, op)
			rights = append(rights, right)
		}

		return &Compare{
			Left:  left,
			Ops:   ops,
			Right: rights,
		}
	}

	for {
		bp := infixBindingPower(p.current.Type)
		if bp <= minBP {
			break
		}

		opTok := p.current
		p.advance()

		right := p.parseExpression(bp)

		left = &BinOp{
			Left:  left,
			Op:    p.tokenTypeToOperator(opTok.Type),
			Right: right,
		}
	}

	return left
}

func isCompareOp(t lexer.TokenType) bool {
	switch t {
	case lexer.EQEQUAL, lexer.NOTEQUAL, lexer.LESS, lexer.LESSEQUAL, lexer.GREATER, lexer.GREATEREQUAL:
		return true
	default:
		return false
	}
}

func infixBindingPower(t lexer.TokenType) int {
	switch t {
	case lexer.PLUS, lexer.MINUS:
		return SUM
	case lexer.STAR, lexer.SLASH, lexer.DOUBLESLASH:
		return PRODUCT
	case lexer.EQEQUAL, lexer.NOTEQUAL, lexer.LESS, lexer.LESSEQUAL, lexer.GREATER, lexer.GREATEREQUAL:
		return COMPARE
	default:
		return LOWEST
	}
}

func (p *Parser) parsePrimary() Expression {
	switch p.current.Type {
	case lexer.NUMBER:
		n := &Number{Value: p.current.Literal}
		p.advance()
		return n
	case lexer.NAME:
		n := &Name{Id: p.current.Literal}
		p.advance()

		if p.current.Type == lexer.LPAR {
			return p.parseCall(n)
		}
		return n
	case lexer.LPAR:
		p.advance()
		expr := p.parseExpression(LOWEST)
		if p.current.Type != lexer.RPAR {
			panic("expected )")
		}
		p.advance()
		return expr
	case lexer.STRING:
		s := &String{Value: p.current.Literal}
		p.advance()
		return s

	case lexer.MINUS:
		p.advance()
		operand := p.parseExpression(PREFIX)
		return &UnaryOp{
			Op:      USub,
			Operand: operand,
		}

	case lexer.PLUS:
		p.advance()
		operand := p.parseExpression(PREFIX)
		return &UnaryOp{
			Op:      UAdd,
			Operand: operand,
		}

	case lexer.NOT:
		p.advance()
		expr := p.parseExpression(PREFIX)
		return &UnaryOp{
			Op:      Not,
			Operand: expr,
		}

	}
	panic(fmt.Sprintf("unexpected token %v\n", p.current))
}

func (p *Parser) parseCall(func_expr Expression) Expression {
	p.advance()
	args := []Expression{}

	if p.current.Type != lexer.RPAR {
		args = append(args, p.parseExpression(LOWEST))

		for p.current.Type == lexer.COMMA {
			p.advance()
			args = append(args, p.parseExpression(LOWEST))
		}
	}

	if p.current.Type != lexer.RPAR {
		panic("expected ) after function arguments")
	}
	p.advance()

	return &Call{
		Func: func_expr,
		Args: args,
	}
}

func (p *Parser) isOperator(t lexer.TokenType) bool {
	return t == lexer.PLUS || t == lexer.MINUS || t == lexer.STAR || t == lexer.SLASH || t == lexer.DOUBLESLASH || t == lexer.PERCENT
}

func tokenTypeToCompareOp(t lexer.TokenType) CompareOp {
	switch t {
	case lexer.EQEQUAL:
		return Eq
	case lexer.NOTEQUAL:
		return NotEq
	case lexer.LESS:
		return Lt
	case lexer.LESSEQUAL:
		return LtE
	case lexer.GREATER:
		return Gt
	case lexer.GREATEREQUAL:
		return GtE
	default:
		panic("invalid compare operator")
	}
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
