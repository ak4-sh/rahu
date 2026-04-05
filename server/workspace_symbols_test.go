package server

import (
	"path/filepath"
	"testing"

	"rahu/lsp"
)

func TestWorkspaceSymbolSearchSingleModule(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\nclass Bar:\n    pass\n\ndef baz():\n    pass\n")

	s := newWorkspaceServer(t, root)
	results, err := s.WorkspaceSymbol(&lsp.WorkspaceSymbolParams{Query: "ba"})
	if err != nil {
		t.Fatalf("unexpected workspace symbol error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("unexpected result count: got %d want 2", len(results))
	}
	if results[0].Name != "Bar" || results[1].Name != "baz" {
		t.Fatalf("unexpected symbol names: %+v", results)
	}
}

func TestWorkspaceSymbolSearchAcrossModules(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "a.py"), "foo = 1\n")
	writeWorkspaceFile(t, filepath.Join(root, "b.py"), "bar = 1\n")

	s := newWorkspaceServer(t, root)
	results, err := s.WorkspaceSymbol(&lsp.WorkspaceSymbolParams{Query: ""})
	if err != nil {
		t.Fatalf("unexpected workspace symbol error: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected symbols from multiple modules, got %+v", results)
	}
	assertWorkspaceSymbol(t, results, "foo", "a")
	assertWorkspaceSymbol(t, results, "bar", "b")
}

func TestWorkspaceSymbolLocationMatchesDefinition(t *testing.T) {
	root := t.TempDir()
	modPath := filepath.Join(root, "pkg", "mod.py")
	writeWorkspaceFile(t, modPath, "foo = 1\n")

	s := newWorkspaceServer(t, root)
	results, err := s.WorkspaceSymbol(&lsp.WorkspaceSymbolParams{Query: "foo"})
	if err != nil {
		t.Fatalf("unexpected workspace symbol error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("unexpected result count: %+v", results)
	}
	if results[0].Location.URI != pathToURI(modPath) {
		t.Fatalf("unexpected symbol URI: got %q", results[0].Location.URI)
	}
	if results[0].Location.Range.Start.Line != 0 || results[0].Location.Range.Start.Character != 0 {
		t.Fatalf("unexpected symbol position: got %d:%d", results[0].Location.Range.Start.Line, results[0].Location.Range.Start.Character)
	}
}

func TestWorkspaceSymbolSkipsUnresolvedImports(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "main.py"), "from missing.mod import foo\n")

	s := newWorkspaceServer(t, root)
	results, err := s.WorkspaceSymbol(&lsp.WorkspaceSymbolParams{Query: "foo"})
	if err != nil {
		t.Fatalf("unexpected workspace symbol error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected unresolved imports to be excluded, got %+v", results)
	}
}

func TestWorkspaceSymbolUsesOpenDocumentSnapshot(t *testing.T) {
	root := t.TempDir()
	modPath := filepath.Join(root, "pkg", "mod.py")
	writeWorkspaceFile(t, modPath, "foo = 1\n")

	s := newWorkspaceServer(t, root)
	modURI := pathToURI(modPath)
	s.Open(lsp.TextDocumentItem{URI: modURI, Text: "bar = 1\n", Version: 1})
	s.analyze(s.Get(modURI))

	results, err := s.WorkspaceSymbol(&lsp.WorkspaceSymbolParams{Query: "bar"})
	if err != nil {
		t.Fatalf("unexpected workspace symbol error: %v", err)
	}
	assertWorkspaceSymbol(t, results, "bar", "pkg.mod")

	results, err = s.WorkspaceSymbol(&lsp.WorkspaceSymbolParams{Query: "foo"})
	if err != nil {
		t.Fatalf("unexpected workspace symbol error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected stale disk export to disappear for open doc, got %+v", results)
	}
}

func assertWorkspaceSymbol(t *testing.T, results []lsp.SymbolInformation, name, container string) {
	t.Helper()
	for _, result := range results {
		if result.Name == name && result.ContainerName == container {
			return
		}
	}
	t.Fatalf("expected symbol %q in container %q, got %+v", name, container, results)
}
