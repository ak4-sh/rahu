package server

import (
	"path/filepath"
	"testing"

	"rahu/lsp"
	"rahu/source"
)

func TestReferencesLocalVariable(t *testing.T) {
	code := "x = 1\ny = x\nx\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	refs, err := s.References(referenceParams(uri, code, 1, 4, true))
	if err != nil {
		t.Fatalf("unexpected references error: %v", err)
	}
	if len(refs) != 3 {
		t.Fatalf("unexpected reference count: %+v", refs)
	}
}

func TestReferencesExcludeDeclaration(t *testing.T) {
	code := "x = 1\ny = x\nx\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	refs, err := s.References(referenceParams(uri, code, 1, 4, false))
	if err != nil {
		t.Fatalf("unexpected references error: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("unexpected reference count without declaration: %+v", refs)
	}
}

func TestReferencesCrossFileImportedFunction(t *testing.T) {
	root := t.TempDir()
	modPath := filepath.Join(root, "mod.py")
	mainPath := filepath.Join(root, "main.py")
	modCode := "def foo():\n    pass\n"
	mainCode := "from mod import foo\nfoo()\n"
	writeWorkspaceFile(t, modPath, modCode)
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	refs, err := s.References(referenceParams(mainURI, mainCode, 1, 0, true))
	if err != nil {
		t.Fatalf("unexpected references error: %v", err)
	}
	if len(refs) != 3 {
		t.Fatalf("unexpected cross-file reference count: %+v", refs)
	}
	assertReferenceURI(t, refs, pathToURI(modPath))
	assertReferenceURI(t, refs, mainURI)
}

func TestReferencesAliasImportIncludesDefinition(t *testing.T) {
	root := t.TempDir()
	modPath := filepath.Join(root, "mod.py")
	mainPath := filepath.Join(root, "main.py")
	modCode := "def foo():\n    pass\n"
	mainCode := "from mod import foo as bar\nbar()\n"
	writeWorkspaceFile(t, modPath, modCode)
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	refs, err := s.References(referenceParams(mainURI, mainCode, 1, 0, true))
	if err != nil {
		t.Fatalf("unexpected references error: %v", err)
	}
	if len(refs) != 3 {
		t.Fatalf("unexpected alias reference count: %+v", refs)
	}
	assertReferenceURI(t, refs, pathToURI(modPath))
	assertReferenceURI(t, refs, mainURI)
}

func TestReferencesIncludeIndexedNonOpenFiles(t *testing.T) {
	root := t.TempDir()
	modPath := filepath.Join(root, "mod.py")
	mainPath := filepath.Join(root, "main.py")
	modCode := "def foo():\n    pass\n"
	mainCode := "from mod import foo\nfoo()\n"
	writeWorkspaceFile(t, modPath, modCode)
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	refs, err := s.References(referenceParams(mainURI, mainCode, 1, 0, true))
	if err != nil {
		t.Fatalf("unexpected references error: %v", err)
	}
	if len(refs) != 3 {
		t.Fatalf("expected indexed non-open file references, got %+v", refs)
	}
}

func TestReferencesBrokenImportReturnsNone(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from missing.mod import foo\nfoo\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	refs, err := s.References(referenceParams(mainURI, mainCode, 1, 0, true))
	if err != nil {
		t.Fatalf("unexpected references error: %v", err)
	}
	if len(refs) != 0 {
		t.Fatalf("expected no references for unresolved import, got %+v", refs)
	}
}

func TestReferencesDirectAttributeIncludeDeclaration(t *testing.T) {
	code := "class Foo:\n    def __init__(self):\n        self.value = 1\n\n    def read(self):\n        return self.value\n\nfoo = Foo()\nfoo.value\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	refs, err := s.References(referenceParams(uri, code, 5, 20, true))
	if err != nil {
		t.Fatalf("unexpected references error: %v", err)
	}
	if len(refs) != 3 {
		t.Fatalf("unexpected attribute reference count: %+v", refs)
	}
}

func TestReferencesDirectAttributeExcludeDeclaration(t *testing.T) {
	code := "class Foo:\n    def __init__(self):\n        self.value = 1\n\n    def read(self):\n        return self.value\n\nfoo = Foo()\nfoo.value\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	refs, err := s.References(referenceParams(uri, code, 5, 20, false))
	if err != nil {
		t.Fatalf("unexpected references error: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("unexpected attribute reference count without declaration: %+v", refs)
	}
}

func TestReferencesDirectAttributeDoNotMixClasses(t *testing.T) {
	code := "class Foo:\n    def __init__(self):\n        self.value = 1\n\n    def read(self):\n        return self.value\n\nclass Bar:\n    def __init__(self):\n        self.value = 2\n\n    def read(self):\n        return self.value\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	refs, err := s.References(referenceParams(uri, code, 5, 20, true))
	if err != nil {
		t.Fatalf("unexpected references error: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("unexpected mixed-class attribute references: %+v", refs)
	}
	for _, ref := range refs {
		if ref.Range.Start.Line >= 7 {
			t.Fatalf("expected Foo.value references only, got %+v", refs)
		}
	}
}

func referenceParams(uri lsp.DocumentURI, code string, line, char int, includeDecl bool) *lsp.ReferenceParams {
	li := source.NewLineIndex(code)
	offset := li.PositionToOffset(line, char)
	refLine, refChar := li.OffsetToPosition(offset)
	return &lsp.ReferenceParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Position:     lsp.Position{Line: refLine, Character: refChar},
		Context:      lsp.ReferenceContext{IncludeDeclaration: includeDecl},
	}
}

func assertReferenceURI(t *testing.T, refs []lsp.Location, want lsp.DocumentURI) {
	t.Helper()
	for _, ref := range refs {
		if ref.URI == want {
			return
		}
	}
	t.Fatalf("expected reference URI %q, got %+v", want, refs)
}
