package server

import (
	"strings"
	"testing"

	"rahu/lsp"
)

func TestHoverWorksInsideFStringExpression(t *testing.T) {
	code := "name = 'x'\nf\"hello {name}\"\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	hov := mustHoverAt(t, s, uri, 1, 9)
	content, ok := hov.Contents.(lsp.MarkupContent)
	if !ok {
		t.Fatalf("expected markup content, got %T", hov.Contents)
	}
	if !strings.Contains(content.Value, "variable(name") {
		t.Fatalf("expected hover on f-string name, got %q", content.Value)
	}
}
