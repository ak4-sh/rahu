package server

import (
	"strings"
	"testing"

	"rahu/lsp"
)

func TestHoverWorksInsideListComprehension(t *testing.T) {
	code := "xs = [1]\nvalues = [x for x in xs if x]\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	hov := mustHoverAt(t, s, uri, 1, 10)
	content, ok := hov.Contents.(lsp.MarkupContent)
	if !ok {
		t.Fatalf("expected markup content, got %T", hov.Contents)
	}
	if !strings.Contains(content.Value, "variable(x: int") {
		t.Fatalf("expected hover inside list comprehension, got %q", content.Value)
	}
}
