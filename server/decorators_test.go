package server

import (
	"strings"
	"testing"

	"rahu/lsp"
)

func TestHoverWorksOnDecoratorName(t *testing.T) {
	code := "dec = print\n@dec\ndef f():\n    pass\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	hov := mustHoverAt(t, s, uri, 1, 1)
	content, ok := hov.Contents.(lsp.MarkupContent)
	if !ok {
		t.Fatalf("expected markup content, got %T", hov.Contents)
	}
	if !strings.Contains(content.Value, "variable(dec") {
		t.Fatalf("expected hover on decorator name, got %q", content.Value)
	}
}
