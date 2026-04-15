package server

import (
	"path/filepath"
	"testing"

	"rahu/lsp"
)

type decodedSemanticToken struct {
	line      int
	start     int
	length    int
	tokenType string
	modifiers uint32
}

func TestSemanticTokensClassFunctionAndKeywords(t *testing.T) {
	code := "class Foo:\n    def bar(self, x: int):\n        return x\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	tokens, err := s.SemanticTokensFull(&lsp.SemanticTokensParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
	if err != nil {
		t.Fatalf("unexpected semantic tokens error: %v", err)
	}
	decoded := decodeSemanticTokens(tokens)
	assertSemanticToken(t, decoded, 0, 0, 5, "keyword")
	assertSemanticToken(t, decoded, 0, 6, 3, "class")
	assertSemanticToken(t, decoded, 1, 4, 3, "keyword")
	assertSemanticToken(t, decoded, 1, 8, 3, "method")
	assertSemanticToken(t, decoded, 1, 12, 4, "parameter")
	assertSemanticToken(t, decoded, 1, 18, 1, "parameter")
	assertSemanticToken(t, decoded, 1, 21, 3, "type")
	assertSemanticToken(t, decoded, 2, 8, 6, "keyword")
	assertSemanticToken(t, decoded, 2, 15, 1, "parameter")
}

func TestSemanticTokensPropertyAndVariable(t *testing.T) {
	code := "class Foo:\n    def __init__(self):\n        self.value = 1\n\nfoo = Foo()\nfoo.value\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	tokens, err := s.SemanticTokensFull(&lsp.SemanticTokensParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
	if err != nil {
		t.Fatalf("unexpected semantic tokens error: %v", err)
	}
	decoded := decodeSemanticTokens(tokens)
	assertSemanticToken(t, decoded, 2, 13, 5, "property")
	assertSemanticToken(t, decoded, 4, 0, 3, "variable")
	assertSemanticToken(t, decoded, 5, 4, 5, "property")
}

func TestSemanticTokensImportedResolvedSymbolUsesResolvedKind(t *testing.T) {
	root := t.TempDir()
	modPath := filepath.Join(root, "mod.py")
	mainPath := filepath.Join(root, "main.py")
	modCode := "def foo(x: int):\n    return x\n"
	mainCode := "from mod import foo\nfoo(1)\n"
	writeWorkspaceFile(t, modPath, modCode)
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	tokens, err := s.SemanticTokensFull(&lsp.SemanticTokensParams{TextDocument: lsp.TextDocumentIdentifier{URI: mainURI}})
	if err != nil {
		t.Fatalf("unexpected semantic tokens error: %v", err)
	}
	decoded := decodeSemanticTokens(tokens)
	assertSemanticToken(t, decoded, 0, 0, 4, "keyword")
	assertSemanticToken(t, decoded, 0, 5, 3, "module")
	assertSemanticToken(t, decoded, 0, 9, 6, "keyword")
	assertSemanticToken(t, decoded, 0, 16, 3, "function")
	assertSemanticToken(t, decoded, 1, 0, 3, "function")
}

func TestSemanticTokensIncludePassKeyword(t *testing.T) {
	code := "if flag:\n    pass\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	tokens, err := s.SemanticTokensFull(&lsp.SemanticTokensParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
	if err != nil {
		t.Fatalf("unexpected semantic tokens error: %v", err)
	}
	decoded := decodeSemanticTokens(tokens)
	assertSemanticToken(t, decoded, 1, 4, 4, "keyword")
}

func TestSemanticTokensIncludeLiteralsAndOperators(t *testing.T) {
	code := "answer = 42\nmsg = \"hi\"\nformatted = f\"{answer}\"\nresult = left and right or not flag\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	tokens, err := s.SemanticTokensFull(&lsp.SemanticTokensParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
	if err != nil {
		t.Fatalf("unexpected semantic tokens error: %v", err)
	}
	decoded := decodeSemanticTokens(tokens)
	assertSemanticToken(t, decoded, 0, 7, 1, "operator")
	assertSemanticToken(t, decoded, 0, 9, 2, "number")
	assertSemanticToken(t, decoded, 1, 4, 1, "operator")
	assertSemanticToken(t, decoded, 1, 6, 4, "string")
	assertSemanticToken(t, decoded, 2, 10, 1, "operator")
	assertSemanticToken(t, decoded, 2, 12, 11, "string")
	assertSemanticToken(t, decoded, 3, 7, 1, "operator")
	assertSemanticToken(t, decoded, 3, 14, 3, "operator")
	assertSemanticToken(t, decoded, 3, 24, 2, "operator")
	assertSemanticToken(t, decoded, 3, 27, 3, "operator")
}

func TestSemanticTokensIncludeDecoratorKeywordVariants(t *testing.T) {
	code := "@decorator\ndef run():\n    global total\n    with ctx as value:\n        yield value\n    del total\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	tokens, err := s.SemanticTokensFull(&lsp.SemanticTokensParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
	if err != nil {
		t.Fatalf("unexpected semantic tokens error: %v", err)
	}
	decoded := decodeSemanticTokens(tokens)
	assertSemanticToken(t, decoded, 0, 1, 9, "decorator")
	assertSemanticToken(t, decoded, 2, 4, 6, "keyword")
	assertSemanticToken(t, decoded, 3, 4, 4, "keyword")
	assertSemanticToken(t, decoded, 3, 13, 2, "keyword")
	assertSemanticToken(t, decoded, 4, 8, 5, "keyword")
	assertSemanticToken(t, decoded, 5, 4, 3, "keyword")
}

func TestSemanticTokensEmptyDataIsNonNil(t *testing.T) {
	code := "x\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	tokens, err := s.SemanticTokensFull(&lsp.SemanticTokensParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
	if err != nil {
		t.Fatalf("unexpected semantic tokens error: %v", err)
	}
	if tokens == nil {
		t.Fatal("expected semantic tokens result")
	}
	if tokens.Data == nil {
		t.Fatal("expected non-nil semantic token data")
	}
	if len(tokens.Data) != 0 {
		t.Fatalf("expected empty semantic token data, got %v", tokens.Data)
	}
}

func TestSemanticTokensMissingDocumentReturnsEmptyData(t *testing.T) {
	s := New(nil)
	uri := lsp.DocumentURI("file:///missing.py")

	tokens, err := s.SemanticTokensFull(&lsp.SemanticTokensParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
	if err != nil {
		t.Fatalf("unexpected semantic tokens error: %v", err)
	}
	if tokens == nil {
		t.Fatal("expected semantic tokens result")
	}
	if tokens.Data == nil {
		t.Fatal("expected non-nil semantic token data")
	}
	if len(tokens.Data) != 0 {
		t.Fatalf("expected empty semantic token data, got %v", tokens.Data)
	}
}

func decodeSemanticTokens(tokens *lsp.SemanticTokens) []decodedSemanticToken {
	if tokens == nil || len(tokens.Data) == 0 {
		return nil
	}
	decoded := make([]decodedSemanticToken, 0, len(tokens.Data)/5)
	line := 0
	start := 0
	for i := 0; i < len(tokens.Data); i += 5 {
		deltaLine := int(tokens.Data[i])
		deltaStart := int(tokens.Data[i+1])
		length := int(tokens.Data[i+2])
		tokenType := int(tokens.Data[i+3])
		modifiers := tokens.Data[i+4]
		line += deltaLine
		if deltaLine == 0 {
			start += deltaStart
		} else {
			start = deltaStart
		}
		decoded = append(decoded, decodedSemanticToken{
			line:      line,
			start:     start,
			length:    length,
			tokenType: semanticTokenLegendTypes[tokenType],
			modifiers: modifiers,
		})
	}
	return decoded
}

func assertSemanticToken(t *testing.T, tokens []decodedSemanticToken, line, start, length int, tokenType string) {
	t.Helper()
	for _, token := range tokens {
		if token.line == line && token.start == start && token.length == length && token.tokenType == tokenType {
			return
		}
	}
	t.Fatalf("expected semantic token %s at %d:%d len %d, got %+v", tokenType, line, start, length, tokens)
}
