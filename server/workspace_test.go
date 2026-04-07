package server

import (
	"context"
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

func TestBuildModuleIndexSkipsCommonNonPythonDirs(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "pkg", "mod.py"))
	writeTestFile(t, filepath.Join(root, ".git", "hooks", "ignored.py"))
	writeTestFile(t, filepath.Join(root, "node_modules", "pkg", "ignored.py"))
	writeTestFile(t, filepath.Join(root, ".venv", "lib", "ignored.py"))
	writeTestFile(t, filepath.Join(root, "vendor", "ignored.py"))
	writeTestFile(t, filepath.Join(root, "build", "ignored.py"))

	s := New(nil)
	s.rootPath = root
	s.buildModuleIndex()

	if _, ok := s.LookupModule("pkg.mod"); !ok {
		t.Fatal("expected python module outside skipped dirs to be indexed")
	}
	for _, name := range []string{".git.hooks.ignored", "node_modules.pkg.ignored", ".venv.lib.ignored", "vendor.ignored", "build.ignored"} {
		if _, ok := s.LookupModule(name); ok {
			t.Fatalf("did not expect module in skipped dir to be indexed: %s", name)
		}
	}
}

func TestBuildModuleIndexMixedRepoOnlyIndexesPythonModules(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "scripts", "tool.py"))
	writeWorkspaceSource(t, filepath.Join(root, "main.go"), "package main\n")
	writeWorkspaceSource(t, filepath.Join(root, "README.md"), "# docs\n")
	writeWorkspaceSource(t, filepath.Join(root, "frontend", "app.ts"), "export const app = true\n")

	s := New(nil)
	s.rootPath = root
	s.buildModuleIndex()

	if got := len(s.modulesByName); got != 1 {
		t.Fatalf("unexpected module count: got %d want 1", got)
	}
	if _, ok := s.LookupModule("scripts.tool"); !ok {
		t.Fatal("expected only python module to be indexed")
	}
	for _, name := range []string{"main", "README", "frontend.app"} {
		if _, ok := s.LookupModule(name); ok {
			t.Fatalf("did not expect non-python module to be indexed: %s", name)
		}
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

	// Trigger background indexing and wait for completion
	s.Initialized(nil)
	if err := s.WaitForIndexing(); err != nil {
		t.Fatalf("indexing failed: %v", err)
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

	// Trigger background indexing and wait for completion
	s.Initialized(nil)
	if err := s.WaitForIndexing(); err != nil {
		t.Fatalf("indexing failed: %v", err)
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

	// Trigger background indexing and wait for completion
	s.Initialized(nil)
	if err := s.WaitForIndexing(); err != nil {
		t.Fatalf("indexing failed: %v", err)
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

func TestWorkspaceIndexWorkerCount(t *testing.T) {
	tests := []struct {
		name      string
		total     int
		available int
		want      int
	}{
		{name: "empty workspace", total: 0, available: 4, want: 1},
		{name: "single module", total: 1, available: 4, want: 1},
		{name: "limited by total", total: 2, available: 8, want: 2},
		{name: "limited by available", total: 10, available: 3, want: 3},
		{name: "clamped to hard cap", total: 20, available: 16, want: 8},
		{name: "invalid available falls back", total: 6, available: 0, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := workspaceIndexWorkerCountWithAvailable(tt.total, tt.available); got != tt.want {
				t.Fatalf("worker count for %d modules with %d available: got %d want %d", tt.total, tt.available, got, tt.want)
			}
		})
	}
}

func TestBuildWorkspaceSnapshotsWithPriorityParallel(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceSource(t, filepath.Join(root, "a.py"), "value = 1\n")
	writeWorkspaceSource(t, filepath.Join(root, "b.py"), "from a import value\nother = value\n")
	writeWorkspaceSource(t, filepath.Join(root, "c.py"), "from b import other\nthird = other\n")
	writeWorkspaceSource(t, filepath.Join(root, "d.py"), "from c import third\nresult = third\n")
	writeWorkspaceSource(t, filepath.Join(root, "e.py"), "from d import result\nfinal = result\n")

	s := New(nil)
	s.rootPath = root
	s.buildModuleIndex()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.buildWorkspaceSnapshotsWithPriority(ctx, cancel); err != nil {
		t.Fatalf("parallel workspace snapshot build failed: %v", err)
	}

	if len(s.moduleSnapshotsByName) != 5 {
		t.Fatalf("unexpected snapshot count: got %d want 5", len(s.moduleSnapshotsByName))
	}
	modD, ok := s.LookupModule("d")
	if !ok {
		t.Fatal("expected module d to be indexed")
	}
	imports := s.moduleImportsByURI[modD.URI]
	if len(imports) != 1 || imports[0] != "c" {
		t.Fatalf("unexpected imports for d.py: %+v", imports)
	}
	dependents := s.reverseDepsByModule["d"]
	modE, ok := s.LookupModule("e")
	if !ok {
		t.Fatal("expected module e to be indexed")
	}
	if _, ok := dependents[modE.URI]; !ok {
		t.Fatalf("expected reverse dependency from d to e, got %+v", dependents)
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
