package server

import (
	"strings"
	"testing"

	"rahu/lsp"
	"rahu/source"
)

func TestSignatureHelpFunctionFirstArg(t *testing.T) {
	code := "def foo(x: int, y: str = \"a\") -> int:\n    return 1\n\nfoo(\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	help, err := s.SignatureHelp(signatureHelpParams(uri, code, 3, 4))
	if err != nil {
		t.Fatalf("unexpected signatureHelp error: %v", err)
	}
	if help.ActiveParameter != 0 {
		t.Fatalf("unexpected active parameter: %+v", help)
	}
	if len(help.Signatures) != 1 {
		t.Fatalf("unexpected signatures: %+v", help)
	}
	label := help.Signatures[0].Label
	if !strings.Contains(label, "foo(x: int, y: str = \"a\")") {
		t.Fatalf("unexpected signature label: %q", label)
	}
	if !strings.Contains(label, "-> int") {
		t.Fatalf("expected return type in label: %q", label)
	}
}

func TestSignatureHelpFunctionSecondArg(t *testing.T) {
	code := "def foo(x: int, y: str = \"a\"):\n    pass\n\nfoo(1, \n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	help, err := s.SignatureHelp(signatureHelpParams(uri, code, 3, 7))
	if err != nil {
		t.Fatalf("unexpected signatureHelp error: %v", err)
	}
	if help.ActiveParameter != 1 {
		t.Fatalf("unexpected active parameter: %+v", help)
	}
}

func TestSignatureHelpMethodCall(t *testing.T) {
	code := "class Foo:\n    def method(self, x: int, y = 1):\n        pass\n\nfoo = Foo()\nfoo.method(1, \n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	help, err := s.SignatureHelp(signatureHelpParams(uri, code, 5, 14))
	if err != nil {
		t.Fatalf("unexpected signatureHelp error: %v", err)
	}
	if help.ActiveParameter != 1 {
		t.Fatalf("unexpected active parameter: %+v", help)
	}
	label := help.Signatures[0].Label
	if !strings.HasPrefix(label, "method(self") {
		t.Fatalf("unexpected method signature label: %q", label)
	}
	if !strings.Contains(label, "x: int") {
		t.Fatalf("expected typed method parameter: %q", label)
	}
	if !strings.Contains(label, "y = 1") {
		t.Fatalf("expected default value in method signature: %q", label)
	}
}

func TestSignatureHelpKeywordArgMatchesByName(t *testing.T) {
	code := "def foo(x: int, y: str = \"a\"):\n    pass\n\nfoo(x=1, y=\"b\")\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	help, err := s.SignatureHelp(signatureHelpParams(uri, code, 3, 11))
	if err != nil {
		t.Fatalf("unexpected signatureHelp error: %v", err)
	}
	if help.ActiveParameter != 1 {
		t.Fatalf("unexpected active parameter for keyword arg: %+v", help)
	}
}

func TestSignatureHelpNestedCallPrefersInnermost(t *testing.T) {
	code := "def inner(x: int, y: int):\n    pass\n\ndef outer(a, b):\n    pass\n\nouter(inner(1, ), 2)\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	help, err := s.SignatureHelp(signatureHelpParams(uri, code, 6, 15))
	if err != nil {
		t.Fatalf("unexpected signatureHelp error: %v", err)
	}
	if help.ActiveParameter != 1 {
		t.Fatalf("unexpected active parameter: %+v", help)
	}
	if !strings.HasPrefix(help.Signatures[0].Label, "inner(") {
		t.Fatalf("expected innermost call signature, got %q", help.Signatures[0].Label)
	}
}

func TestSignatureHelpOutsideCallFails(t *testing.T) {
	code := "x = 1\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	_, err := s.SignatureHelp(signatureHelpParams(uri, code, 0, 0))
	if err == nil {
		t.Fatal("expected signatureHelp outside a call to fail")
	}
}

func signatureHelpParams(uri lsp.DocumentURI, code string, line, char int) *lsp.SignatureHelpParams {
	li := source.NewLineIndex(code)
	offset := li.PositionToOffset(line, char)
	rLine, rChar := li.OffsetToPosition(offset)
	return &lsp.SignatureHelpParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Position:     lsp.Position{Line: rLine, Character: rChar},
	}
}
