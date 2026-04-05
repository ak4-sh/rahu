package server

import (
	"testing"

	"rahu/lsp"
	"rahu/source"
)

func TestDefinitionAttributeLookup(t *testing.T) {
	code := `class Foo:
    def __init__(self):
        self.value = 1

f = Foo()
f.value
`

	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	doc := s.Get(uri)
	s.analyze(doc)

	li := source.NewLineIndex(code)
	offset := li.PositionToOffset(5, 2)
	line, char := li.OffsetToPosition(offset)

	loc, err := s.Definition(&lsp.DefinitionParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Position: lsp.Position{
			Line:      line,
			Character: char,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if loc.Range.Start.Line != 2 || loc.Range.Start.Character != 13 {
		t.Fatalf("expected definition at 2:13, got %d:%d", loc.Range.Start.Line, loc.Range.Start.Character)
	}
}

func TestHoverAttributeLookup(t *testing.T) {
	code := `class Foo:
    def __init__(self):
        self.value = 1

f = Foo()
f.value
`

	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	doc := s.Get(uri)
	s.analyze(doc)

	li := source.NewLineIndex(code)
	offset := li.PositionToOffset(5, 2)
	line, char := li.OffsetToPosition(offset)

	hov, err := s.Hover(&lsp.HoverParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Position: lsp.Position{
			Line:      line,
			Character: char,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hov == nil {
		t.Fatal("expected hover")
	}
	if hov.Range == nil {
		t.Fatal("expected hover range")
	}
	if hov.Range.Start.Line != 2 || hov.Range.Start.Character != 13 {
		t.Fatalf("expected hover range at 2:13, got %d:%d", hov.Range.Start.Line, hov.Range.Start.Character)
	}
}
