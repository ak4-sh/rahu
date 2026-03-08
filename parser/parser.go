// Package parser implements a recursive descent parser for Python source code.
//
// The parser transforms a stream of tokens from the lexer into an Abstract Syntax Tree (AST)
// that represents the syntactic structure of Python programs. It handles Python's indentation-based
// syntax through INDENT and DEDENT tokens provided by the lexer.
//
// Features:
//   - Recursive descent parsing with operator precedence (Pratt parsing)
//   - Support for Python statements: assignments, function definitions, control flow (if/elif/else,
//     for, while), return, break, and continue
//   - Support for Python expressions: arithmetic operations, comparisons (with chaining),
//     boolean operations (and/or), unary operations, function calls, lists, and tuples
//   - Proper operator precedence handling for arithmetic, comparison, and boolean operators
//   - Position tracking for all AST nodes (line and column information)
//   - Advanced features: function default arguments, tuple unpacking in for loops, comparison chaining
//
// The parser produces an AST consisting of Statement and Expression nodes. Each node type
// implements the appropriate interface and includes position information for error reporting
// and language server features.
//
// Example usage:
//
//	input := `
//	def fibonacci(n):
//	    if n <= 1:
//	        return n
//	    return fibonacci(n-1) + fibonacci(n-2)
//	`
//	p := parser.New(input)
//	module := p.Parse()
//
// The resulting AST can be traversed for semantic analysis, type checking, or code generation.
package parser

import (
	"slices"

	"rahu/lexer"
	"rahu/parser/ast"
)

type Error struct {
	Span ast.Range
	Msg  string
}

func (p *Parser) Errors() []Error {
	return p.errors
}

type Parser struct {
	lexer      *lexer.Lexer
	current    lexer.Token
	peek       lexer.Token
	nextNodeID ast.NodeID

	errors []Error
}

func (p *Parser) newNodeID() ast.NodeID {
	p.nextNodeID++
	return p.nextNodeID
}

func (p *Parser) error(span ast.Range, msg string) {
	p.errors = append(p.errors, Error{Span: span, Msg: msg})
}

func (p *Parser) currentRange() ast.Range {
	return ast.Range{
		Start: int(p.current.Start),
		End:   int(p.current.End),
	}
}

func (p *Parser) errorCurrent(msg string) {
	p.error(p.currentRange(), msg)
}

func (p *Parser) syncTo(types ...lexer.TokenType) {
	for p.current.Type != lexer.EOF {
		if slices.Contains(types, p.current.Type) {
			return
		}
		p.advance()
	}
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

func (p *Parser) Parse() *ast.Module {
	statements := []ast.Statement{}
	for p.current.Type != lexer.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			statements = append(statements, stmt)
		}
	}
	return &ast.Module{Body: statements}
}

func canStartExpression(t lexer.TokenType) bool {
	switch t {
	case lexer.NAME, lexer.NUMBER, lexer.STRING, lexer.LPAR, lexer.LSQB, lexer.MINUS, lexer.PLUS, lexer.NOT, lexer.TRUE, lexer.FALSE, lexer.NONE:
		return true
	case lexer.UNTERMINATED_STRING:
		return false
	default:
		return false
	}
}

func (p *Parser) parseAssignmentFromFirst(start int, first ast.Expression) ast.Statement {
	targets := []ast.Expression{first}
	for p.current.Type == lexer.COMMA {
		p.advance()
		t := p.parseExpression(LOWEST)
		if t == nil {
			p.errorCurrent("expected assignment target")
			break
		}
		targets = append(targets, t)
	}

	if p.current.Type != lexer.EQUAL {
		p.error(ast.Range{Start: start, End: int(p.current.Start)}, "expected '=' in assignment")
		p.syncTo(lexer.NEWLINE, lexer.EOF)
		return &ast.Assign{Targets: targets, Value: nil, Pos: ast.Range{Start: start, End: int(p.current.Start)}}
	}
	p.advance()
	value := p.parseExpression(LOWEST)
	end := p.current.Start
	if p.current.Type == lexer.NEWLINE {
		p.advance()
	}
	return &ast.Assign{Targets: targets, Value: value, Pos: ast.Range{Start: start, End: int(end)}}
}

func (p *Parser) isAugAssign() bool {
	switch p.peek.Type {
	case lexer.PLUSEQUAL, lexer.MINEQUAL, lexer.SLASHEQUAL, lexer.STAREQUAL, lexer.DOUBLESLASHEQUAL, lexer.DOUBLESTAREQUAL, lexer.AMPEREQUAL, lexer.NOTEQUAL, lexer.LEFTSHIFTEQUAL, lexer.RIGHTSHIFTEQUAL:
		return true
	default:
		return false
	}
}

func (p *Parser) parseAugAssignFromFirst(start int, target ast.Expression) ast.Statement {
	op := p.current.Type
	p.advance()
	value := p.parseExpression(LOWEST)
	end := p.current.Start
	if p.current.Type == lexer.NEWLINE {
		p.advance()
	}
	return &ast.AugAssign{Target: target, Op: op, Value: value, Pos: ast.Range{Start: start, End: int(end)}}
}

func (p *Parser) parseStatement() ast.Statement {
	if p.current.Type == lexer.UNTERMINATED_STRING {
		p.errorCurrent("unterminated string literal")
		p.advance()
		return nil
	}

	if p.current.Type == lexer.NEWLINE {
		p.advance()
		return nil
	}

	if p.current.Type == lexer.IF {
		return p.parseIf()
	}

	if canStartExpression(p.current.Type) {
		start := p.current.Start
		expr := p.parseExpression(LOWEST)
		if expr == nil {
			p.errorCurrent("expected expression")
			return nil
		}

		if p.current.Type == lexer.EQUAL || p.current.Type == lexer.COMMA {
			switch expr.(type) {
			case *ast.Name, *ast.Attribute, *ast.Tuple, *ast.List:
				return p.parseAssignmentFromFirst(int(start), expr)
			}
		}

		if p.current.Type == lexer.PLUSEQUAL ||
			p.current.Type == lexer.MINEQUAL ||
			p.current.Type == lexer.SLASHEQUAL ||
			p.current.Type == lexer.STAREQUAL ||
			p.current.Type == lexer.DOUBLESLASHEQUAL ||
			p.current.Type == lexer.DOUBLESTAREQUAL ||
			p.current.Type == lexer.AMPEREQUAL ||
			p.current.Type == lexer.LEFTSHIFTEQUAL ||
			p.current.Type == lexer.RIGHTSHIFTEQUAL {
			switch expr.(type) {
			case *ast.Name, *ast.Attribute:
				return p.parseAugAssignFromFirst(int(start), expr)
			}
		}

		if p.current.Type == lexer.NEWLINE {
			p.advance()
		} else if p.current.Type != lexer.EOF {
			p.error(expr.Position(), "expected newline after expression")
		}
		return &ast.ExprStmt{Value: expr, Pos: expr.Position()}

	}

	if p.current.Type == lexer.DEF {
		funcdef := p.parseFunc()
		return funcdef
	}

	if p.current.Type == lexer.BREAK {
		pos := ast.Range{
			Start: int(p.current.Start),
			End:   int(p.current.End),
		}
		p.advance()
		if p.current.Type == lexer.NEWLINE {
			p.advance()
		}
		return &ast.Break{Pos: pos}
	}

	if p.current.Type == lexer.CONTINUE {
		pos := ast.Range{
			Start: int(p.current.Start),
			End:   int(p.current.End),
		}
		p.advance()
		if p.current.Type == lexer.NEWLINE {
			p.advance()
		}
		return &ast.Continue{Pos: pos}
	}

	if p.current.Type == lexer.RETURN {
		return p.parseReturn()
	}

	if p.current.Type == lexer.FOR {
		return p.parseFor()
	}

	if p.current.Type == lexer.WHILE {
		return p.parseWhile()
	}

	if p.current.Type == lexer.CLASS {
		return p.parseClass()
	}

	p.error(ast.Range{
		Start: int(p.current.Start),
		End:   int(p.current.End),
	},
		"unexpected token: "+p.current.String(),
	)

	p.advance()
	return nil
}
