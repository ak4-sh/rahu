package server

import (
	"path/filepath"
	"testing"

	"rahu/lsp"
)

func TestSnapshotLRUEvictsOldestResidentModule(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "a.py"), "foo = 1\n")
	writeWorkspaceFile(t, filepath.Join(root, "b.py"), "bar = 2\n")
	writeWorkspaceFile(t, filepath.Join(root, "c.py"), "baz = 3\n")

	s := New(nil)
	s.rootPath = root
	s.maxCachedModules = 2
	s.buildModuleIndex()

	modA, _ := s.LookupModule("a")
	modB, _ := s.LookupModule("b")
	modC, _ := s.LookupModule("c")

	if _, ok := s.analyzeModuleFile(modA); !ok {
		t.Fatal("expected a.py analysis to succeed")
	}
	if _, ok := s.analyzeModuleFile(modB); !ok {
		t.Fatal("expected b.py analysis to succeed")
	}
	if _, ok := s.analyzeModuleFile(modC); !ok {
		t.Fatal("expected c.py analysis to succeed")
	}

	if _, ok := s.getModuleSnapshotByName("a"); ok {
		t.Fatal("expected a.py snapshot to be evicted")
	}
	if _, ok := s.getModuleSnapshotByName("b"); !ok {
		t.Fatal("expected b.py snapshot to remain resident")
	}
	if _, ok := s.getModuleSnapshotByName("c"); !ok {
		t.Fatal("expected c.py snapshot to remain resident")
	}
	if got := residentSnapshotCount(s); got != 2 {
		t.Fatalf("unexpected resident snapshot count: got %d want 2", got)
	}
	assertRefIndexEvicted(t, s, modA.URI)
}

func TestSnapshotLRUPinsOpenWorkspaceModules(t *testing.T) {
	root := t.TempDir()
	aPath := filepath.Join(root, "a.py")
	bPath := filepath.Join(root, "b.py")
	cPath := filepath.Join(root, "c.py")
	writeWorkspaceFile(t, aPath, "foo = 1\n")
	writeWorkspaceFile(t, bPath, "bar = 2\n")
	writeWorkspaceFile(t, cPath, "baz = 3\n")

	s := New(nil)
	s.rootPath = root
	s.maxCachedModules = 2
	s.buildModuleIndex()

	aURI := pathToURI(aPath)
	s.Open(lsp.TextDocumentItem{URI: aURI, Text: "foo = 1\n", Version: 1})

	modA, _ := s.LookupModule("a")
	modB, _ := s.LookupModule("b")
	modC, _ := s.LookupModule("c")
	_, _ = s.analyzeModuleFile(modA)
	_, _ = s.analyzeModuleFile(modB)
	_, _ = s.analyzeModuleFile(modC)

	if _, ok := s.getModuleSnapshotByName("a"); !ok {
		t.Fatal("expected open module a.py to stay resident")
	}
	if _, ok := s.getModuleSnapshotByName("c"); !ok {
		t.Fatal("expected newest module c.py to stay resident")
	}
	if _, ok := s.getModuleSnapshotByName("b"); ok {
		t.Fatal("expected b.py to be evicted instead of pinned a.py")
	}

	s.Close(aURI)
}

func TestSnapshotLRUReloadsEvictedModuleOnDemand(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "a.py"), "foo = 1\n")
	writeWorkspaceFile(t, filepath.Join(root, "b.py"), "bar = 2\n")
	writeWorkspaceFile(t, filepath.Join(root, "c.py"), "baz = 3\n")

	s := New(nil)
	s.rootPath = root
	s.maxCachedModules = 2
	s.buildModuleIndex()

	modA, _ := s.LookupModule("a")
	modB, _ := s.LookupModule("b")
	modC, _ := s.LookupModule("c")

	firstA, _ := s.analyzeModuleFile(modA)
	_, _ = s.analyzeModuleFile(modB)
	_, _ = s.analyzeModuleFile(modC)
	if _, ok := s.getModuleSnapshotByName("a"); ok {
		t.Fatal("expected a.py to be evicted before reload")
	}

	reloadedA, ok := s.analyzeModuleFile(modA)
	if !ok || reloadedA == nil {
		t.Fatal("expected evicted module to reload successfully")
	}
	if reloadedA == firstA {
		t.Fatal("expected reload to rebuild a new snapshot instance")
	}
	if _, ok := s.getModuleSnapshotByName("a"); !ok {
		t.Fatal("expected a.py to be resident after reload")
	}
}

func residentSnapshotCount(s *Server) int {
	s.snapshotsMu.RLock()
	defer s.snapshotsMu.RUnlock()
	return len(s.moduleSnapshotsByURI)
}

func assertRefIndexEvicted(t *testing.T, s *Server, uri lsp.DocumentURI) {
	t.Helper()

	s.refIndex.mu.RLock()
	defer s.refIndex.mu.RUnlock()
	if _, ok := s.refIndex.byURI[uri]; ok {
		t.Fatalf("expected refIndex entries for %q to be evicted", uri)
	}
}
