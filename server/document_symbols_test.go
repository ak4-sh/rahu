package server

import (
	"testing"

	"rahu/lsp"
)

func TestDocumentSymbolTopLevelOutline(t *testing.T) {
	code := "class Foo:\n    def bar(self):\n        pass\n\ndef baz():\n    pass\n\nx = 1\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	syms, err := s.DocumentSymbol(&lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
	if err != nil {
		t.Fatalf("unexpected documentSymbol error: %v", err)
	}
	if len(syms) != 3 {
		t.Fatalf("unexpected symbol count: got %d want 3", len(syms))
	}
	if syms[0].Name != "Foo" || syms[0].Kind != lsp.SymbolKindClass {
		t.Fatalf("unexpected first symbol: %+v", syms[0])
	}
	if syms[1].Name != "baz" || syms[1].Kind != lsp.SymbolKindFunction {
		t.Fatalf("unexpected second symbol: %+v", syms[1])
	}
	if syms[2].Name != "x" || syms[2].Kind != lsp.SymbolKindVariable {
		t.Fatalf("unexpected third symbol: %+v", syms[2])
	}
}

func TestDocumentSymbolClassMethodsNested(t *testing.T) {
	code := "class Foo:\n    def a(self):\n        pass\n\n    def b(self):\n        pass\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	syms, err := s.DocumentSymbol(&lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
	if err != nil {
		t.Fatalf("unexpected documentSymbol error: %v", err)
	}
	if len(syms) != 1 {
		t.Fatalf("unexpected symbol count: %+v", syms)
	}
	if len(syms[0].Children) != 2 {
		t.Fatalf("unexpected child count: %+v", syms[0].Children)
	}
	if syms[0].Children[0].Kind != lsp.SymbolKindMethod || syms[0].Children[1].Kind != lsp.SymbolKindMethod {
		t.Fatalf("expected method children, got %+v", syms[0].Children)
	}
}

func TestDocumentSymbolSelectionAndFullRange(t *testing.T) {
	code := "def foo():\n    x = 1\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	syms, err := s.DocumentSymbol(&lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
	if err != nil {
		t.Fatalf("unexpected documentSymbol error: %v", err)
	}
	if len(syms) != 1 {
		t.Fatalf("unexpected symbol count: %+v", syms)
	}
	if syms[0].SelectionRange.Start.Line != 0 || syms[0].SelectionRange.Start.Character != 4 {
		t.Fatalf("unexpected selection range: %+v", syms[0].SelectionRange)
	}
	if syms[0].Range.End.Line < 1 {
		t.Fatalf("unexpected full range: %+v", syms[0].Range)
	}
}

func TestDocumentSymbolSkipsLocalsAndSubscriptTargets(t *testing.T) {
	code := "def foo():\n    x = 1\n\na = [1]\na[0] = 2\nobj.x = 3\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	syms, err := s.DocumentSymbol(&lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
	if err != nil {
		t.Fatalf("unexpected documentSymbol error: %v", err)
	}
	if len(syms) != 2 {
		t.Fatalf("unexpected symbol count: %+v", syms)
	}
	if syms[0].Name != "foo" || syms[1].Name != "a" {
		t.Fatalf("unexpected symbols: %+v", syms)
	}
}

func TestDocumentSymbolTupleDestructuring(t *testing.T) {
	code := "a, [b, c] = value\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	syms, err := s.DocumentSymbol(&lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
	if err != nil {
		t.Fatalf("unexpected documentSymbol error: %v", err)
	}
	if len(syms) != 3 {
		t.Fatalf("unexpected symbol count: %+v", syms)
	}
	if syms[0].Name != "a" || syms[1].Name != "b" || syms[2].Name != "c" {
		t.Fatalf("unexpected destructured symbols: %+v", syms)
	}
}
