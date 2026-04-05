package server

import (
	"path/filepath"
	"testing"

	"rahu/lsp"
)

func TestCompletionImportModule(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "mod.py"), "foo = 1\n")

	s := newWorkspaceServer(t, root)
	uri := pathToURI(filepath.Join(root, "main.py"))
	code := "import mo"
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 0, Character: len(code)}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "mod")
}

func TestCompletionFromImportExports(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "mod.py"), "foo = 1\nbar = 2\n")

	s := newWorkspaceServer(t, root)
	uri := pathToURI(filepath.Join(root, "main.py"))
	code := "from mod import fo"
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 0, Character: len(code)}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "foo")
}

func TestCompletionAliasedModuleMember(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\nbar = 2\n")

	s := newWorkspaceServer(t, root)
	uri := pathToURI(filepath.Join(root, "main.py"))
	code := "import pkg.mod as m\nm."
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 1, Character: 2}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "foo")
	assertCompletionLabel(t, items, "bar")
}

func TestCompletionUsesOpenDocumentSnapshot(t *testing.T) {
	root := t.TempDir()
	modPath := filepath.Join(root, "pkg", "mod.py")
	writeWorkspaceFile(t, modPath, "foo = 1\n")

	s := newWorkspaceServer(t, root)
	modURI := pathToURI(modPath)
	s.Open(lsp.TextDocumentItem{URI: modURI, Text: "bar = 1\n", Version: 1})
	s.analyze(s.Get(modURI))

	mainURI := pathToURI(filepath.Join(root, "main.py"))
	code := "import pkg.mod as m\nm."
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: code, Version: 1})
	s.analyze(s.Get(mainURI))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: mainURI}, Position: lsp.Position{Line: 1, Character: 2}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "bar")
	assertCompletionMissing(t, items, "foo")
}

func TestCompletionBareImportPackageSuggestsChildModule(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "__init__.py"), "")
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\n")

	s := newWorkspaceServer(t, root)
	uri := pathToURI(filepath.Join(root, "main.py"))
	code := "import pkg.mod\npkg."
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 1, Character: 4}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "mod")
}

func TestCompletionBareImportPackagePrefixFiltering(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "__init__.py"), "")
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\n")

	s := newWorkspaceServer(t, root)
	uri := pathToURI(filepath.Join(root, "main.py"))
	code := "import pkg.mod\npkg.mo"
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 1, Character: 6}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "mod")
}

func TestCompletionBareImportPackageIncludesExportsAndChildModules(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "__init__.py"), "foo = 1\n")
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "bar = 1\n")

	s := newWorkspaceServer(t, root)
	uri := pathToURI(filepath.Join(root, "main.py"))
	code := "import pkg.mod\npkg."
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 1, Character: 4}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "foo")
	assertCompletionLabel(t, items, "mod")
}

func TestCompletionBareImportPackageWithoutInitReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\n")

	s := newWorkspaceServer(t, root)
	uri := pathToURI(filepath.Join(root, "main.py"))
	code := "import pkg.mod\npkg."
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 1, Character: 4}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty completion list without package __init__.py, got %+v", items)
	}
}

func TestCompletionSelfAttributeMembers(t *testing.T) {
	code := "class Foo:\n    def __init__(self):\n        self.value = 1\n\n    def method(self):\n        self.\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 5, Character: 13}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "value")
	assertCompletionLabel(t, items, "method")
}

func TestCompletionInstanceAttributeMembers(t *testing.T) {
	code := "class Foo:\n    def __init__(self):\n        self.value = 1\n\n    def method(self):\n        pass\n\nx = Foo()\nx.\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 8, Character: 2}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "value")
	assertCompletionLabel(t, items, "method")
}

func TestCompletionClassMembers(t *testing.T) {
	code := "class Foo:\n    def __init__(self):\n        self.value = 1\n\n    def method(self):\n        pass\n\nFoo.\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 7, Character: 4}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "value")
	assertCompletionLabel(t, items, "method")
}

func TestCompletionInheritedMembers(t *testing.T) {
	code := "class A:\n    def base(self):\n        pass\n\nclass B(A):\n    def child(self):\n        pass\n\nx = B()\nx.\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 9, Character: 2}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "base")
	assertCompletionLabel(t, items, "child")
}

func TestCompletionChainedAssignmentInstanceMembers(t *testing.T) {
	code := "class Foo:\n    def __init__(self):\n        self.value = 1\n\n    def method(self):\n        pass\n\nx = Foo()\ny = x\ny.\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 9, Character: 2}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "value")
	assertCompletionLabel(t, items, "method")
}

func TestCompletionUnionInstanceMembers(t *testing.T) {
	code := "class Foo:\n    def foo(self):\n        pass\n\nclass Bar:\n    def bar(self):\n        pass\n\nx = Foo()\nx = Bar()\nx.\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 10, Character: 2}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "foo")
	assertCompletionLabel(t, items, "bar")
}

func TestCompletionUnionMembersDeduped(t *testing.T) {
	code := "class Foo:\n    def common(self):\n        pass\n\nclass Bar:\n    def common(self):\n        pass\n\nx = Foo()\nx = Bar()\nx.\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 10, Character: 2}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	count := 0
	for _, item := range items {
		if item.Label == "common" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected deduped union member, got %+v", items)
	}
}

func TestCompletionListElementMembers(t *testing.T) {
	code := "class Foo:\n    def value(self):\n        pass\n\nxs = [Foo()]\nxs[0].\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 5, Character: 6}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "value")
}

func TestCompletionMixedListElementUnionMembers(t *testing.T) {
	code := "class Foo:\n    def foo(self):\n        pass\n\nclass Bar:\n    def bar(self):\n        pass\n\nxs = [Foo(), Bar()]\nxs[0].\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 9, Character: 6}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "foo")
	assertCompletionLabel(t, items, "bar")
}

func TestCompletionTupleElementUnionMembers(t *testing.T) {
	code := "class Foo:\n    def foo(self):\n        pass\n\nclass Bar:\n    def bar(self):\n        pass\n\nt = (Foo(), Bar())\nt[0].\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 9, Character: 5}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "foo")
	assertCompletionLabel(t, items, "bar")
}

func TestCompletionListAppendElementMembers(t *testing.T) {
	code := "class Foo:\n    def value(self):\n        pass\n\nxs = []\nxs.append(Foo())\nxs[0].\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 6, Character: 6}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "value")
}

func TestCompletionListAppendUnionMembers(t *testing.T) {
	code := "class Foo:\n    def foo(self):\n        pass\n\nclass Bar:\n    def bar(self):\n        pass\n\nxs = []\nxs.append(Foo())\nxs.append(Bar())\nxs[0].\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 11, Character: 6}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "foo")
	assertCompletionLabel(t, items, "bar")
}

func TestCompletionUnknownReceiverReturnsEmpty(t *testing.T) {
	code := "x.\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 0, Character: 2}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty completion list for unknown receiver, got %+v", items)
	}
}

func TestCompletionAliasedModuleRegression(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\n")

	s := newWorkspaceServer(t, root)
	uri := pathToURI(filepath.Join(root, "main.py"))
	code := "import pkg.mod as m\nm."
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 1, Character: 2}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "foo")
}

func TestCompletionMissingModuleReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	s := newWorkspaceServer(t, root)
	uri := pathToURI(filepath.Join(root, "main.py"))
	code := "from missing import "
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 0, Character: len(code)}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty completion list, got %+v", items)
	}
}

func TestCompletionGenericModuleNames(t *testing.T) {
	code := "foo = 1\n\ndef bar():\n    pass\n\nba"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 5, Character: 2}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "bar")
}

func TestCompletionGenericFunctionLocalsAndParams(t *testing.T) {
	code := "def fn(arg):\n    local = 1\n    lo"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 2, Character: 6}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "local")
	items, err = s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 2, Character: 1}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "arg")
}

func TestCompletionGenericIncludesBuiltins(t *testing.T) {
	code := "def fn():\n    pri"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 1, Character: 7}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "print")
}

func TestCompletionGenericIncludesImportedNames(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "mod.py"), "foo = 1\n")

	s := newWorkspaceServer(t, root)
	uri := pathToURI(filepath.Join(root, "main.py"))
	code := "from mod import foo\nfo"
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 1, Character: 2}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "foo")
}

func TestCompletionGenericIncludesClassMethodsInClassScope(t *testing.T) {
	code := "class Foo:\n    def alpha(self):\n        pass\n\n    def beta(self):\n        pass\n\n    be"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 7, Character: 6}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "beta")
}

func TestCompletionGenericScopePrecedence(t *testing.T) {
	code := "name = 1\n\ndef fn():\n    name = 2\n    na"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 4, Character: 6}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	count := 0
	for _, item := range items {
		if item.Label == "name" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected one visible 'name' completion, got %+v", items)
	}
}

func TestCompletionRankingKeepsBuiltinWhenShadowed(t *testing.T) {
	code := "print = 1\npri"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 1, Character: 3}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	count := 0
	for _, item := range items {
		if item.Label == "print" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("expected shadowed builtin to remain visible, got %+v", items)
	}
	if items[0].Label != "print" || items[0].Detail != "variable" {
		t.Fatalf("expected local print to rank first, got %+v", items[:min(2, len(items))])
	}
}

func TestCompletionRankingPrefersLocalOverGlobal(t *testing.T) {
	code := "name = 1\n\ndef fn():\n    name = 2\n    na"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 4, Character: 6}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if items[0].Label != "name" || items[0].Detail != "variable" {
		t.Fatalf("expected local name to rank first, got %+v", items)
	}
	count := 0
	for _, item := range items {
		if item.Label == "name" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected only nearest non-builtin duplicate, got %+v", items)
	}
}

func TestCompletionRankingPrefersNonUnderscorePrefix(t *testing.T) {
	code := "bar = 1\n_bar = 2\nba"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 2, Character: 2}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if len(items) == 0 || items[0].Label != "bar" {
		t.Fatalf("expected non-underscore prefix match first, got %+v", items)
	}
}

func assertCompletionLabel(t *testing.T, items []lsp.CompletionItem, want string) {
	t.Helper()
	for _, item := range items {
		if item.Label == want {
			return
		}
	}
	t.Fatalf("expected completion %q, got %+v", want, items)
}

func assertCompletionMissing(t *testing.T, items []lsp.CompletionItem, unwanted string) {
	t.Helper()
	for _, item := range items {
		if item.Label == unwanted {
			t.Fatalf("did not expect completion %q, got %+v", unwanted, items)
		}
	}
}
