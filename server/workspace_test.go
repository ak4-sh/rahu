package server

import (
	"os"
	"path/filepath"
	"testing"

	"rahu/lsp"
)

func TestModuleNameFromPath(t *testing.T) {
	root := t.TempDir()

	tests := []struct {
		name string
		path string
		want string
		ok   bool
	}{
		{name: "module file", path: filepath.Join(root, "pkg", "mod.py"), want: "pkg.mod", ok: true},
		{name: "nested module file", path: filepath.Join(root, "pkg", "sub", "mod.py"), want: "pkg.sub.mod", ok: true},
		{name: "package init", path: filepath.Join(root, "pkg", "__init__.py"), want: "pkg", ok: true},
		{name: "root init skipped", path: filepath.Join(root, "__init__.py"), want: "", ok: false},
		{name: "non python skipped", path: filepath.Join(root, "pkg", "mod.txt"), want: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := moduleNameFromPath(root, tt.path)
			if ok != tt.ok {
				t.Fatalf("ok mismatch: got %v want %v", ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("module mismatch: got %q want %q", got, tt.want)
			}
		})
	}
}

func TestBuildModuleIndex(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "pkg", "__init__.py"))
	writeTestFile(t, filepath.Join(root, "pkg", "mod.py"))
	writeTestFile(t, filepath.Join(root, "pkg", "sub", "thing.py"))
	writeTestFile(t, filepath.Join(root, "notes.txt"))

	s := New(nil)
	s.rootPath = root
	s.buildModuleIndex()

	if _, ok := s.LookupModule("pkg"); !ok {
		t.Fatal("expected package module to be indexed")
	}

	mod, ok := s.LookupModule("pkg.mod")
	if !ok {
		t.Fatal("expected pkg.mod to be indexed")
	}
	if mod.Path != filepath.Join(root, "pkg", "mod.py") {
		t.Fatalf("unexpected module path: %q", mod.Path)
	}

	byURI, ok := s.LookupModuleByURI(mod.URI)
	if !ok {
		t.Fatal("expected module lookup by URI to succeed")
	}
	if byURI.Name != "pkg.mod" {
		t.Fatalf("unexpected module name by URI: %q", byURI.Name)
	}

	if _, ok := s.LookupModule("notes"); ok {
		t.Fatal("did not expect non-python files to be indexed")
	}
}

func TestInitializeBuildsModuleIndex(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "pkg", "mod.py"))

	rootURI := pathToURI(root)
	s := New(nil)

	_, err := s.Initialize(&lsp.InitializeParams{RootURI: &rootURI})
	if err != nil {
		t.Fatalf("unexpected initialize error: %v", err)
	}

	if s.rootURI != rootURI {
		t.Fatalf("root URI mismatch: got %q want %q", s.rootURI, rootURI)
	}
	if s.rootPath != root {
		t.Fatalf("root path mismatch: got %q want %q", s.rootPath, root)
	}
	if _, ok := s.LookupModule("pkg.mod"); !ok {
		t.Fatal("expected initialize to build module index")
	}
}

func TestInitializeBuildsWorkspaceSnapshots(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "pkg", "mod.py"))

	rootURI := pathToURI(root)
	s := New(nil)

	_, err := s.Initialize(&lsp.InitializeParams{RootURI: &rootURI})
	if err != nil {
		t.Fatalf("unexpected initialize error: %v", err)
	}

	snapshot, ok := s.moduleSnapshotsByName["pkg.mod"]
	if !ok || snapshot == nil {
		t.Fatal("expected initialize to build workspace snapshot")
	}
	if snapshot.Exports == nil {
		t.Fatal("expected snapshot exports to be extracted")
	}
	if _, ok := snapshot.Exports["x"]; !ok {
		t.Fatal("expected top-level export x in snapshot")
	}
}

func TestInitializeBuildsWorkspaceDependencies(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "pkg", "mod.py"))
	writeWorkspaceSource(t, filepath.Join(root, "main.py"), "from pkg.mod import x\n")

	rootURI := pathToURI(root)
	s := New(nil)

	_, err := s.Initialize(&lsp.InitializeParams{RootURI: &rootURI})
	if err != nil {
		t.Fatalf("unexpected initialize error: %v", err)
	}

	mainURI := pathToURI(filepath.Join(root, "main.py"))
	imports := s.moduleImportsByURI[mainURI]
	if len(imports) != 1 || imports[0] != "pkg.mod" {
		t.Fatalf("unexpected imports for main.py: %+v", imports)
	}
	dependents := s.reverseDepsByModule["pkg.mod"]
	if _, ok := dependents[mainURI]; !ok {
		t.Fatal("expected reverse dependency from pkg.mod to main.py")
	}
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte("x = 1\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
}

func writeWorkspaceSource(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
}
