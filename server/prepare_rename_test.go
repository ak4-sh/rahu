package server

import (
	"path/filepath"
	"testing"

	"rahu/lsp"
	"rahu/source"
)

func TestPrepareRenameLocalVariable(t *testing.T) {
	code := "x = 1\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	res, err := s.PrepareRename(prepareRenameParams(uri, code, 0, 0))
	if err != nil {
		t.Fatalf("unexpected prepareRename error: %v", err)
	}
	if res.Placeholder != "x" {
		t.Fatalf("unexpected placeholder: %+v", res)
	}
}

func TestPrepareRenameImportedAlias(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "mod.py"), "def foo():\n    pass\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from mod import foo as bar\nbar()\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	res, err := s.PrepareRename(prepareRenameParams(mainURI, mainCode, 1, 0))
	if err != nil {
		t.Fatalf("unexpected prepareRename error: %v", err)
	}
	if res.Placeholder != "bar" {
		t.Fatalf("unexpected placeholder: %+v", res)
	}
}

func TestPrepareRenameRejectsAttributeTarget(t *testing.T) {
	code := "class Foo:\n    def __init__(self):\n        self.value = 1\n\nf = Foo()\nf.value\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	_, err := s.PrepareRename(prepareRenameParams(uri, code, 5, 2))
	if err == nil {
		t.Fatal("expected prepareRename on attribute to fail")
	}
}

func TestPrepareRenameRejectsBuiltin(t *testing.T) {
	code := "print(x)\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	_, err := s.PrepareRename(prepareRenameParams(uri, code, 0, 0))
	if err == nil {
		t.Fatal("expected prepareRename on builtin to fail")
	}
}

func TestPrepareRenameRejectsBrokenImport(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from missing.mod import foo\nfoo\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	_, err := s.PrepareRename(prepareRenameParams(mainURI, mainCode, 1, 0))
	if err == nil {
		t.Fatal("expected prepareRename on broken import to fail")
	}
}

func TestPrepareRenameRejectsNonNamePosition(t *testing.T) {
	code := "x = 1\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	_, err := s.PrepareRename(prepareRenameParams(uri, code, 0, 2))
	if err == nil {
		t.Fatal("expected prepareRename on punctuation to fail")
	}
}

func prepareRenameParams(uri lsp.DocumentURI, code string, line, char int) *lsp.PrepareRenameParams {
	li := source.NewLineIndex(code)
	offset := li.PositionToOffset(line, char)
	rLine, rChar := li.OffsetToPosition(offset)
	return &lsp.PrepareRenameParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Position:     lsp.Position{Line: rLine, Character: rChar},
	}
}
