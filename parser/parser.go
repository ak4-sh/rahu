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
	"rahu/lexer"
	"rahu/lsp"
)

type Error struct {
	Span Range
	Msg  string
}

func (p *Parser) Errors() []Error {
	return p.errors
}

type Parser struct {
	lexer   *lexer.Lexer
	current lexer.Token
	peek    lexer.Token

	errors []Error
}

func (p *Parser) error(span Range, msg string) {
	p.errors = append(p.errors, Error{Span: span, Msg: msg})
}

func (p *Parser) currentRange() Range {
	return Range{
		Start: Position{Line: p.current.Line, Col: p.current.Col},
		End:   Position{Line: p.current.Line, Col: p.current.EndCol},
	}
}

func (p *Parser) errorCurrent(msg string) {
	p.error(p.currentRange(), msg)
}

func (p *Parser) syncTo(types ...lexer.TokenType) {
	for p.current.Type != lexer.EOF {
		for _, t := range types {
			if p.current.Type == t {
				return
			}
		}
		p.advance()
	}
}

func (r Range) ToLSPRange() lsp.Range {
	return lsp.Range{
		Start: lsp.Position{
			Line:      r.Start.Line,
			Character: r.Start.Col - 1,
		},
		End: lsp.Position{
			Line:      r.End.Line - 1,
			Character: r.End.Col - 1,
		},
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

func (p *Parser) parseStatement() Statement {
	if p.current.Type == lexer.UNTERMINATED_STRING {
		p.errorCurrent("unterminated string literal")
		p.advance()
		return nil
	}

	if p.current.Type == lexer.NEWLINE {
		p.advance()
		return nil
	}

	if p.current.Type == lexer.NAME && p.peek.Type == lexer.EQUAL {
		return p.parseAssignment()
	}

	if p.current.Type == lexer.IF {
		return p.parseIf()
	}

	if canStartExpression(p.current.Type) {
		expr := p.parseExpression(LOWEST)
		if p.current.Type == lexer.NEWLINE {
			p.advance()
		} else if p.current.Type != lexer.EOF {
			p.error(expr.Position(), "expected newline after expression")
		}
		return &ExprStmt{Value: expr, Pos: expr.Position()}
	}

	if p.current.Type == lexer.DEF {
		funcdef := p.parseFunc()
		return funcdef
	}

	if p.current.Type == lexer.BREAK {
		pos := Range{
			Start: Position{Line: p.current.Line, Col: p.current.Col},
			End:   Position{Line: p.current.Line, Col: p.current.EndCol},
		}
		p.advance()
		if p.current.Type == lexer.NEWLINE {
			p.advance()
		}
		return &Break{Pos: pos}
	}

	if p.current.Type == lexer.CONTINUE {
		pos := Range{
			Start: Position{Line: p.current.Line, Col: p.current.Col},
			End:   Position{Line: p.current.Line, Col: p.current.EndCol},
		}
		p.advance()
		if p.current.Type == lexer.NEWLINE {
			p.advance()
		}
		return &Continue{Pos: pos}
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

	p.error(Range{
		Start: Position{Line: p.current.Line, Col: p.current.Col},
		End:   Position{Line: p.current.Line, Col: p.current.EndCol},
	},
		"unexpected token: "+p.current.String(),
	)

	p.advance()
	return nil
}
