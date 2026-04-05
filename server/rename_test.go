package server

import (
	"path/filepath"
	"testing"

	"rahu/lsp"
	"rahu/source"
)

func TestRenameLocalVariable(t *testing.T) {
	code := "x = 1\ny = x\nx\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	edit, err := s.Rename(renameParams(uri, code, 1, 4, "value"))
	if err != nil {
		t.Fatalf("unexpected rename error: %v", err)
	}
	if len(edit.Changes[uri]) != 3 {
		t.Fatalf("unexpected local rename edits: %+v", edit.Changes)
	}
}

func TestRenameCrossFileExportedFunction(t *testing.T) {
	root := t.TempDir()
	modPath := filepath.Join(root, "mod.py")
	mainPath := filepath.Join(root, "main.py")
	modCode := "def foo():\n    pass\n"
	mainCode := "from mod import foo\nfoo()\n"
	writeWorkspaceFile(t, modPath, modCode)
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	modURI := pathToURI(modPath)
	s.Open(lsp.TextDocumentItem{URI: modURI, Text: modCode, Version: 1})
	s.analyze(s.Get(modURI))

	edit, err := s.Rename(renameParams(modURI, modCode, 0, 4, "renamed"))
	if err != nil {
		t.Fatalf("unexpected rename error: %v", err)
	}
	if len(edit.Changes[pathToURI(modPath)]) != 1 || len(edit.Changes[pathToURI(mainPath)]) != 2 {
		t.Fatalf("unexpected cross-file rename edits: %+v", edit.Changes)
	}
}

func TestRenameAliasImportDoesNotRenameSource(t *testing.T) {
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

	edit, err := s.Rename(renameParams(mainURI, mainCode, 1, 0, "baz"))
	if err != nil {
		t.Fatalf("unexpected rename error: %v", err)
	}
	if _, ok := edit.Changes[pathToURI(modPath)]; ok {
		t.Fatalf("did not expect source module rename edits for alias rename: %+v", edit.Changes)
	}
	if len(edit.Changes[mainURI]) != 2 {
		t.Fatalf("unexpected alias rename edits: %+v", edit.Changes)
	}
}

func TestRenameRejectsInvalidIdentifier(t *testing.T) {
	code := "x = 1\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	_, err := s.Rename(renameParams(uri, code, 0, 0, "123bad"))
	if err == nil {
		t.Fatal("expected invalid identifier rename to fail")
	}
}

func TestRenameBrokenImportFails(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from missing.mod import foo\nfoo\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	_, err := s.Rename(renameParams(mainURI, mainCode, 1, 0, "bar"))
	if err == nil {
		t.Fatal("expected rename on broken import to fail")
	}
}

func TestRenameDirectAttributeSameFile(t *testing.T) {
	code := "class Foo:\n    def __init__(self):\n        self.value = 1\n\n    def read(self):\n        return self.value\n\nfoo = Foo()\nfoo.value\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	edit, err := s.Rename(renameParams(uri, code, 5, 20, "renamed"))
	if err != nil {
		t.Fatalf("unexpected rename error: %v", err)
	}
	if len(edit.Changes[uri]) != 3 {
		t.Fatalf("unexpected direct attribute rename edits: %+v", edit.Changes)
	}
	for _, change := range edit.Changes[uri] {
		if change.NewText != "renamed" {
			t.Fatalf("unexpected new text: %+v", change)
		}
		if change.Range.End.Character-change.Range.Start.Character != len("value") {
			t.Fatalf("expected attribute-only edit range, got %+v", change)
		}
	}
}

func TestRenameDirectAttributeDoesNotRenameOtherClassMember(t *testing.T) {
	code := "class Foo:\n    def __init__(self):\n        self.value = 1\n\n    def read(self):\n        return self.value\n\nclass Bar:\n    def __init__(self):\n        self.value = 2\n\n    def read(self):\n        return self.value\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	edit, err := s.Rename(renameParams(uri, code, 5, 20, "renamed"))
	if err != nil {
		t.Fatalf("unexpected rename error: %v", err)
	}
	changes := edit.Changes[uri]
	if len(changes) != 2 {
		t.Fatalf("unexpected direct attribute rename edits: %+v", edit.Changes)
	}
	for _, change := range changes {
		if change.Range.Start.Line >= 7 {
			t.Fatalf("expected Foo.value edits only, got %+v", changes)
		}
	}
}

func renameParams(uri lsp.DocumentURI, code string, line, char int, newName string) *lsp.RenameParams {
	li := source.NewLineIndex(code)
	offset := li.PositionToOffset(line, char)
	rLine, rChar := li.OffsetToPosition(offset)
	return &lsp.RenameParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Position:     lsp.Position{Line: rLine, Character: rChar},
		NewName:      newName,
	}
}
