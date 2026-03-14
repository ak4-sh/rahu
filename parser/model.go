// parser builds a compact arena-allocated AST. Nodes are stored in a slice
// and referenced by NodeID. Parent/child relationships are represented using
// FirstChild and NextSibling links. The parser is responsible for constructing
// nodes in a consistent child order for each NodeKind.
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
	lexer   *lexer.Lexer
	current lexer.Token
	peek    lexer.Token
	tree    *ast.AST

	errors   []Error
	inputLen int
}

func (p *Parser) error(span ast.Range, msg string) {
	p.errors = append(p.errors, Error{Span: span, Msg: msg})
}

func (p *Parser) currentRange() ast.Range {
	return ast.Range{
		Start: p.current.Start,
		End:   p.current.End,
	}
}

func (p *Parser) errorCurrent(msg string) {
	span := ast.Range{Start: p.current.Start, End: p.current.End}
	p.errors = append(p.errors, Error{Span: span, Msg: msg})
}

func (p *Parser) syncTo(types ...lexer.TokenType) {
	for p.current.Type != lexer.EOF {
		if slices.Contains(types, p.current.Type) {
			return
		}
		p.current = p.peek
		p.peek = p.lexer.NextToken()
	}
}

func New(input string) *Parser {
	l := lexer.New(input)
	p := &Parser{
		lexer:    l,
		inputLen: len(input),
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
		p.current = p.peek
		p.peek = p.lexer.NextToken()
	}
}

func (p *Parser) Parse() *ast.AST {
	tree := ast.New(p.inputLen)
	module := tree.NewNode(ast.NodeModule, 0, 0)
	tree.Root = module
	p.tree = tree
	for p.current.Type != lexer.EOF {
		if stmt := p.parseStatement(); stmt != ast.NoNode {
			tree.AddChild(module, stmt)
		}
	}

	tree.Nodes[module].End = uint32(p.inputLen)
	return tree
}
