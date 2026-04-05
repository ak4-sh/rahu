package server

import (
	"strings"
	"testing"

	"rahu/lsp"
)

func TestHoverWorksInsideTryExceptFinally(t *testing.T) {
	code := "value = 1\ntry:\n    value\nexcept ValueError as err:\n    err\nfinally:\n    value\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	hov := mustHoverAt(t, s, uri, 6, 4)
	content, ok := hov.Contents.(lsp.MarkupContent)
	if !ok {
		t.Fatalf("expected markup content, got %T", hov.Contents)
	}
	if !strings.Contains(content.Value, "variable(value: int") {
		t.Fatalf("expected hover inside finally block, got %q", content.Value)
	}
}

func TestSemanticTokensIncludeTryExceptFinallyKeywords(t *testing.T) {
	code := "try:\n    risky\nexcept ValueError as err:\n    err\nfinally:\n    cleanup\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	tokens, err := s.SemanticTokensFull(&lsp.SemanticTokensParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
	if err != nil {
		t.Fatalf("unexpected semantic tokens error: %v", err)
	}
	decoded := decodeSemanticTokens(tokens)
	assertSemanticToken(t, decoded, 0, 0, 3, "keyword")
	assertSemanticToken(t, decoded, 2, 0, 6, "keyword")
	assertSemanticToken(t, decoded, 4, 0, 7, "keyword")
}
