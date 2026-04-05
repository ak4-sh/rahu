package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rahu/lsp"
	"rahu/source"
)

func TestDefinitionFromImportCrossFile(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from pkg.mod import foo\nfoo\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	loc := mustDefinitionAt(t, s, mainURI, mainCode, 1, 0)
	if loc.URI != pathToURI(filepath.Join(root, "pkg", "mod.py")) {
		t.Fatalf("unexpected definition URI: got %q", loc.URI)
	}
	if loc.Range.Start.Line != 0 || loc.Range.Start.Character != 0 {
		t.Fatalf("unexpected definition position: got %d:%d", loc.Range.Start.Line, loc.Range.Start.Character)
	}
}

func TestDefinitionFromImportAliasCrossFile(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from pkg.mod import foo as bar\nbar\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	loc := mustDefinitionAt(t, s, mainURI, mainCode, 1, 0)
	if loc.URI != pathToURI(filepath.Join(root, "pkg", "mod.py")) {
		t.Fatalf("unexpected definition URI: got %q", loc.URI)
	}
}

func TestDefinitionImportAliasModuleCrossFile(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "import pkg.mod as m\nm\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	loc := mustDefinitionAt(t, s, mainURI, mainCode, 1, 0)
	if loc.URI != pathToURI(filepath.Join(root, "pkg", "mod.py")) {
		t.Fatalf("unexpected definition URI: got %q", loc.URI)
	}
	if loc.Range.Start.Line != 0 || loc.Range.Start.Character != 0 {
		t.Fatalf("unexpected module definition position: got %d:%d", loc.Range.Start.Line, loc.Range.Start.Character)
	}
}

func TestDefinitionImportPackageUsesInitModule(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "__init__.py"), "value = 1\n")
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "import pkg.mod\npkg\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	loc := mustDefinitionAt(t, s, mainURI, mainCode, 1, 0)
	if loc.URI != pathToURI(filepath.Join(root, "pkg", "__init__.py")) {
		t.Fatalf("unexpected definition URI: got %q", loc.URI)
	}
}

func TestDefinitionImportPackageWithoutInitReturnsNil(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "import pkg.mod\npkg\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	loc, err := s.Definition(&lsp.DefinitionParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: mainURI},
		Position: lsp.Position{
			Line:      1,
			Character: 0,
		},
	})
	if err == nil {
		t.Fatal("expected no definition result")
	}
	if loc != nil {
		t.Fatal("expected nil location when package __init__.py is missing")
	}
}

func TestHoverFromImportCrossFile(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from pkg.mod import foo\nfoo\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	hov := mustHoverAt(t, s, mainURI, 1, 0)
	if hov.Range == nil {
		t.Fatal("expected hover range")
	}
	if hov.Range.Start.Line != 0 || hov.Range.Start.Character != 0 {
		t.Fatalf("unexpected hover range start: got %d:%d", hov.Range.Start.Line, hov.Range.Start.Character)
	}
	content, ok := hov.Contents.(lsp.MarkupContent)
	if !ok {
		t.Fatalf("expected markup content, got %T", hov.Contents)
	}
	if !strings.Contains(content.Value, "mod.py:1") {
		t.Fatalf("expected cross-file hover footer, got %q", content.Value)
	}
}

func TestHoverFromImportAliasCrossFile(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from pkg.mod import foo as bar\nbar\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	hov := mustHoverAt(t, s, mainURI, 1, 0)
	if hov.Range == nil {
		t.Fatal("expected hover range")
	}
	if hov.Range.Start.Line != 0 || hov.Range.Start.Character != 0 {
		t.Fatalf("unexpected hover range start: got %d:%d", hov.Range.Start.Line, hov.Range.Start.Character)
	}
	content, ok := hov.Contents.(lsp.MarkupContent)
	if !ok {
		t.Fatalf("expected markup content, got %T", hov.Contents)
	}
	if !strings.Contains(content.Value, "mod.py:1") {
		t.Fatalf("expected cross-file hover footer, got %q", content.Value)
	}
}

func TestHoverImportPackageUsesInitModule(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "__init__.py"), "value = 1\n")
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "import pkg.mod\npkg\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	hov := mustHoverAt(t, s, mainURI, 1, 0)
	if hov.Range == nil {
		t.Fatal("expected hover range")
	}
	if hov.Range.Start.Line != 0 || hov.Range.Start.Character != 0 {
		t.Fatalf("unexpected hover range start: got %d:%d", hov.Range.Start.Line, hov.Range.Start.Character)
	}
	content, ok := hov.Contents.(lsp.MarkupContent)
	if !ok {
		t.Fatalf("expected markup content, got %T", hov.Contents)
	}
	if !strings.Contains(content.Value, "__init__.py:1") {
		t.Fatalf("expected package hover footer, got %q", content.Value)
	}
}

func TestHoverShowsInferredInstanceType(t *testing.T) {
	code := "class Foo:\n    pass\n\nx = Foo()\nx\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	hov := mustHoverAt(t, s, uri, 4, 0)
	content, ok := hov.Contents.(lsp.MarkupContent)
	if !ok {
		t.Fatalf("expected markup content, got %T", hov.Contents)
	}
	if !strings.Contains(content.Value, "variable(x: Foo") {
		t.Fatalf("expected inferred instance type in hover, got %q", content.Value)
	}
}

func TestHoverShowsInferredBuiltinType(t *testing.T) {
	code := "n = 1\nn\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	hov := mustHoverAt(t, s, uri, 1, 0)
	content, ok := hov.Contents.(lsp.MarkupContent)
	if !ok {
		t.Fatalf("expected markup content, got %T", hov.Contents)
	}
	if !strings.Contains(content.Value, "variable(n: int") {
		t.Fatalf("expected inferred builtin type in hover, got %q", content.Value)
	}
}

func TestHoverShowsUnionType(t *testing.T) {
	code := "class Foo:\n    pass\n\nclass Bar:\n    pass\n\nx = Foo()\nx = Bar()\nx\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	hov := mustHoverAt(t, s, uri, 7, 0)
	content, ok := hov.Contents.(lsp.MarkupContent)
	if !ok {
		t.Fatalf("expected markup content, got %T", hov.Contents)
	}
	if !strings.Contains(content.Value, "variable(x: Foo | Bar") {
		t.Fatalf("expected union type in hover, got %q", content.Value)
	}
}

func TestHoverShowsAnnotatedParameterType(t *testing.T) {
	code := "class Foo:\n    pass\n\ndef f(x: Foo):\n    x\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	hov := mustHoverAt(t, s, uri, 4, 4)
	content, ok := hov.Contents.(lsp.MarkupContent)
	if !ok {
		t.Fatalf("expected markup content, got %T", hov.Contents)
	}
	if !strings.Contains(content.Value, "parameter(x: Foo)") {
		t.Fatalf("expected annotated parameter type in hover, got %q", content.Value)
	}
}

func TestHoverShowsAnnotatedReturnTypePropagation(t *testing.T) {
	code := "class Foo:\n    pass\n\ndef make() -> Foo:\n    return Foo()\n\nvalue = make()\nvalue\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	hov := mustHoverAt(t, s, uri, 7, 0)
	content, ok := hov.Contents.(lsp.MarkupContent)
	if !ok {
		t.Fatalf("expected markup content, got %T", hov.Contents)
	}
	if !strings.Contains(content.Value, "variable(value: Foo") {
		t.Fatalf("expected annotated return type in hover, got %q", content.Value)
	}
}

func TestHoverShowsListAnnotation(t *testing.T) {
	code := "def f(items: list[int]):\n    items\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	hov := mustHoverAt(t, s, uri, 1, 4)
	content, ok := hov.Contents.(lsp.MarkupContent)
	if !ok {
		t.Fatalf("expected markup content, got %T", hov.Contents)
	}
	if !strings.Contains(content.Value, "parameter(items: list[int])") {
		t.Fatalf("expected list[int] in hover, got %q", content.Value)
	}
}

func TestHoverShowsTupleAnnotation(t *testing.T) {
	code := "def f(pair: tuple[str, int]):\n    pair\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	hov := mustHoverAt(t, s, uri, 1, 4)
	content, ok := hov.Contents.(lsp.MarkupContent)
	if !ok {
		t.Fatalf("expected markup content, got %T", hov.Contents)
	}
	if !strings.Contains(content.Value, "parameter(pair: tuple[str, int])") {
		t.Fatalf("expected tuple[str, int] in hover, got %q", content.Value)
	}
}

func TestHoverShowsDictAnnotation(t *testing.T) {
	code := "def f(mapping: dict[str, int]):\n    mapping\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	hov := mustHoverAt(t, s, uri, 1, 4)
	content, ok := hov.Contents.(lsp.MarkupContent)
	if !ok {
		t.Fatalf("expected markup content, got %T", hov.Contents)
	}
	if !strings.Contains(content.Value, "parameter(mapping: dict[str, int])") {
		t.Fatalf("expected dict[str, int] in hover, got %q", content.Value)
	}
}

func TestHoverShowsSetAnnotation(t *testing.T) {
	code := "def f(items: set[int]):\n    items\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	hov := mustHoverAt(t, s, uri, 1, 4)
	content, ok := hov.Contents.(lsp.MarkupContent)
	if !ok {
		t.Fatalf("expected markup content, got %T", hov.Contents)
	}
	if !strings.Contains(content.Value, "parameter(items: set[int])") {
		t.Fatalf("expected set[int] in hover, got %q", content.Value)
	}
}

func TestHoverShowsAnnotatedVariableType(t *testing.T) {
	code := "value: dict[str, int] = {}\nvalue\n"
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: code, Version: 1})
	s.analyze(s.Get(uri))

	hov := mustHoverAt(t, s, uri, 1, 0)
	content, ok := hov.Contents.(lsp.MarkupContent)
	if !ok {
		t.Fatalf("expected markup content, got %T", hov.Contents)
	}
	if !strings.Contains(content.Value, "variable(value: dict[str, int]") {
		t.Fatalf("expected annotated variable type in hover, got %q", content.Value)
	}
}

func TestUnresolvedModuleDiagnosticForImport(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.py")
	mainCode := "import missing\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	doc := s.Get(mainURI)
	assertSemanticDiagnostic(t, doc, "unresolved module: missing", 0, 7)
}

func TestUnresolvedModuleDiagnosticForFromImport(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from missing.mod import foo\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	doc := s.Get(mainURI)
	assertSemanticDiagnostic(t, doc, "unresolved module: missing.mod", 0, 5)
}

func TestUnresolvedModuleDiagnosticForStrictPackageBinding(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "import pkg.mod\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	doc := s.Get(mainURI)
	assertSemanticDiagnostic(t, doc, "unresolved module: pkg", 0, 7)
}

func TestNoUnresolvedModuleDiagnosticWhenModuleExists(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from pkg.mod import foo\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	for _, err := range s.Get(mainURI).SemErrs {
		if err.Msg == "unresolved module: pkg.mod" || err.Msg == "unresolved module: pkg" {
			t.Fatalf("unexpected unresolved module error: %+v", err)
		}
	}
}

func TestDefinitionFromExternalImportLazyResolution(t *testing.T) {
	root := t.TempDir()
	extRoot := filepath.Join(t.TempDir(), "site-packages")
	writeWorkspaceFile(t, filepath.Join(extRoot, "extpkg", "__init__.py"), "def foo():\n    pass\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from extpkg import foo\nfoo\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServerWithExternalRoots(t, root, extRoot)
	if len(s.externalModulesByName) != 0 {
		t.Fatalf("expected no external modules before import-driven lookup, got %+v", s.externalModulesByName)
	}
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	loc := mustDefinitionAt(t, s, mainURI, mainCode, 1, 0)
	wantURI := pathToURI(filepath.Join(extRoot, "extpkg", "__init__.py"))
	if loc.URI != wantURI {
		t.Fatalf("unexpected external definition URI: got %q want %q", loc.URI, wantURI)
	}
	if _, ok := s.externalModulesByName["extpkg"]; !ok {
		t.Fatal("expected external module to be cached after lazy resolution")
	}
}

func TestCompletionFromExternalModuleExports(t *testing.T) {
	root := t.TempDir()
	extRoot := filepath.Join(t.TempDir(), "site-packages")
	writeWorkspaceFile(t, filepath.Join(extRoot, "extpkg", "__init__.py"), "def foo():\n    pass\nbar = 1\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "import extpkg\nextpkg.\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServerWithExternalRoots(t, root, extRoot)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: mainURI}, Position: lsp.Position{Line: 1, Character: len("extpkg.")}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "foo")
	assertCompletionLabel(t, items, "bar")
}

func TestWorkspaceModuleShadowsExternalModule(t *testing.T) {
	root := t.TempDir()
	extRoot := filepath.Join(t.TempDir(), "site-packages")
	writeWorkspaceFile(t, filepath.Join(root, "extpkg.py"), "value = 1\n")
	writeWorkspaceFile(t, filepath.Join(extRoot, "extpkg", "__init__.py"), "value = 2\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from extpkg import value\nvalue\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServerWithExternalRoots(t, root, extRoot)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	loc := mustDefinitionAt(t, s, mainURI, mainCode, 1, 0)
	wantURI := pathToURI(filepath.Join(root, "extpkg.py"))
	if loc.URI != wantURI {
		t.Fatalf("expected workspace module to shadow external module, got %q want %q", loc.URI, wantURI)
	}
	if _, ok := s.externalModulesByName["extpkg"]; ok {
		t.Fatal("did not expect external module to be loaded when workspace module shadows it")
	}
}

func TestMissingImportedNameDiagnostic(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from pkg.mod import bar\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	doc := s.Get(mainURI)
	assertSemanticDiagnostic(t, doc, "cannot import name 'bar' from 'pkg.mod'", 0, 20)
}

func TestMissingImportedNameDiagnosticWithAlias(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from pkg.mod import bar as baz\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	doc := s.Get(mainURI)
	assertSemanticDiagnostic(t, doc, "cannot import name 'bar' from 'pkg.mod'", 0, 20)
}

func TestDefinitionRelativeSiblingImportCrossFile(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "__init__.py"), "")
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "b.py"), "foo = 1\n")
	mainPath := filepath.Join(root, "pkg", "a.py")
	mainCode := "from .b import foo\nfoo\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	loc := mustDefinitionAt(t, s, mainURI, mainCode, 1, 0)
	if loc.URI != pathToURI(filepath.Join(root, "pkg", "b.py")) {
		t.Fatalf("unexpected definition URI: got %q", loc.URI)
	}
}

func TestDefinitionRelativeParentImportCrossFile(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "__init__.py"), "")
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "sub", "__init__.py"), "")
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "util.py"), "foo = 1\n")
	mainPath := filepath.Join(root, "pkg", "sub", "a.py")
	mainCode := "from ..util import foo\nfoo\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	loc := mustDefinitionAt(t, s, mainURI, mainCode, 1, 0)
	if loc.URI != pathToURI(filepath.Join(root, "pkg", "util.py")) {
		t.Fatalf("unexpected definition URI: got %q", loc.URI)
	}
}

func TestDefinitionRelativePackageImportUsesInit(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "__init__.py"), "foo = 1\n")
	mainPath := filepath.Join(root, "pkg", "a.py")
	mainCode := "from . import foo\nfoo\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	loc := mustDefinitionAt(t, s, mainURI, mainCode, 1, 0)
	if loc.URI != pathToURI(filepath.Join(root, "pkg", "__init__.py")) {
		t.Fatalf("unexpected definition URI: got %q", loc.URI)
	}
}

func TestHoverFromImportModulePathResolvesFullModule(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "storybox", "__init__.py"), "APP_NAME = \"Storybox\"\n")
	writeWorkspaceFile(t, filepath.Join(root, "storybox", "engine.py"), "def run():\n    pass\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from storybox.engine import run\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	hov := mustHoverAt(t, s, mainURI, 0, 6)
	content, ok := hov.Contents.(lsp.MarkupContent)
	if !ok {
		t.Fatalf("expected markup content, got %T", hov.Contents)
	}
	if !strings.Contains(content.Value, "module(storybox.engine)") {
		t.Fatalf("expected full-module hover on import path, got %q", content.Value)
	}
	if !strings.Contains(content.Value, "engine.py:1") {
		t.Fatalf("expected engine.py footer, got %q", content.Value)
	}

	hov = mustHoverAt(t, s, mainURI, 0, 15)
	content, ok = hov.Contents.(lsp.MarkupContent)
	if !ok {
		t.Fatalf("expected markup content, got %T", hov.Contents)
	}
	if !strings.Contains(content.Value, "module(storybox.engine)") {
		t.Fatalf("expected full-module hover on engine segment, got %q", content.Value)
	}
}

func TestDefinitionImportModulePathResolvesFullModule(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "__init__.py"), "")
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "import pkg.mod\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	loc := mustDefinitionAt(t, s, mainURI, mainCode, 0, 8)
	if loc.URI != pathToURI(filepath.Join(root, "pkg", "mod.py")) {
		t.Fatalf("unexpected definition URI: got %q", loc.URI)
	}
	loc = mustDefinitionAt(t, s, mainURI, mainCode, 0, 11)
	if loc.URI != pathToURI(filepath.Join(root, "pkg", "mod.py")) {
		t.Fatalf("unexpected definition URI for module segment: got %q", loc.URI)
	}
}

func TestDefinitionRelativeImportModulePathResolvesFullModule(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "__init__.py"), "")
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\n")
	mainPath := filepath.Join(root, "pkg", "a.py")
	mainCode := "from .mod import foo\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	loc := mustDefinitionAt(t, s, mainURI, mainCode, 0, 7)
	if loc.URI != pathToURI(filepath.Join(root, "pkg", "mod.py")) {
		t.Fatalf("unexpected relative module definition URI: got %q", loc.URI)
	}
}

func TestRelativeImportUnresolvedModuleDiagnostic(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "__init__.py"), "")
	mainPath := filepath.Join(root, "pkg", "a.py")
	mainCode := "from .missing import foo\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	assertSemanticDiagnostic(t, s.Get(mainURI), "unresolved module: pkg.missing", 0, 5)
}

func TestNoMissingImportedNameDiagnosticWhenExportExists(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from pkg.mod import foo\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	for _, err := range s.Get(mainURI).SemErrs {
		if err.Msg == "cannot import name 'foo' from 'pkg.mod'" {
			t.Fatalf("unexpected missing import diagnostic: %+v", err)
		}
	}
}

func TestUnresolvedModuleDoesNotReportMissingImportedName(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from missing.mod import foo\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	doc := s.Get(mainURI)
	assertSemanticDiagnostic(t, doc, "unresolved module: missing.mod", 0, 5)
	for _, err := range doc.SemErrs {
		if err.Msg == "cannot import name 'foo' from 'missing.mod'" {
			t.Fatalf("unexpected missing import diagnostic when module is unresolved: %+v", err)
		}
	}
}

func TestRefreshModuleAndDependentsUpdatesOpenImporterDiagnostics(t *testing.T) {
	root := t.TempDir()
	modPath := filepath.Join(root, "pkg", "mod.py")
	writeWorkspaceFile(t, modPath, "foo = 1\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from pkg.mod import foo\nfoo\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	modURI := pathToURI(modPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.Open(lsp.TextDocumentItem{URI: modURI, Text: "foo = 1\n", Version: 1})

	if got := len(s.Get(mainURI).SemErrs); got != 0 {
		t.Fatalf("unexpected initial semantic errors: %+v", s.Get(mainURI).SemErrs)
	}

	s.Update(modURI, "bar = 1\n", 2)
	s.analyze(s.Get(modURI))

	doc := s.Get(mainURI)
	assertSemanticDiagnostic(t, doc, "cannot import name 'foo' from 'pkg.mod'", 0, 20)
}

func TestDidCloseRebuildsWorkspaceModuleFromDisk(t *testing.T) {
	root := t.TempDir()
	modPath := filepath.Join(root, "pkg", "mod.py")
	writeWorkspaceFile(t, modPath, "foo = 1\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from pkg.mod import foo\nfoo\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	modURI := pathToURI(modPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.Open(lsp.TextDocumentItem{URI: modURI, Text: "bar = 1\n", Version: 1})
	s.analyze(s.Get(modURI))

	assertSemanticDiagnostic(t, s.Get(mainURI), "cannot import name 'foo' from 'pkg.mod'", 0, 20)

	s.DidClose(&lsp.DidCloseTextDocumentParams{TextDocument: lsp.TextDocumentIdentifier{URI: modURI}})

	for _, err := range s.Get(mainURI).SemErrs {
		if err.Msg == "cannot import name 'foo' from 'pkg.mod'" {
			t.Fatalf("expected importer diagnostics to clear after closing edited module: %+v", s.Get(mainURI).SemErrs)
		}
	}
}

func TestRefreshModuleAndDependents_TransitiveImportChain(t *testing.T) {
	root := t.TempDir()
	aPath := filepath.Join(root, "a.py")
	bPath := filepath.Join(root, "b.py")
	cPath := filepath.Join(root, "c.py")

	writeWorkspaceFile(t, cPath, "foo = 1\n")
	writeWorkspaceFile(t, bPath, "from c import foo\n")
	writeWorkspaceFile(t, aPath, "from b import foo\nfoo\n")

	s := newWorkspaceServer(t, root)
	aURI := pathToURI(aPath)
	bURI := pathToURI(bPath)
	cURI := pathToURI(cPath)

	s.Open(lsp.TextDocumentItem{URI: aURI, Text: "from b import foo\nfoo\n", Version: 1})
	s.Open(lsp.TextDocumentItem{URI: bURI, Text: "from c import foo\n", Version: 1})
	s.Open(lsp.TextDocumentItem{URI: cURI, Text: "foo = 1\n", Version: 1})

	if got := len(s.Get(aURI).SemErrs); got != 0 {
		t.Fatalf("unexpected initial semantic errors in a.py: %+v", s.Get(aURI).SemErrs)
	}
	if got := len(s.Get(bURI).SemErrs); got != 0 {
		t.Fatalf("unexpected initial semantic errors in b.py: %+v", s.Get(bURI).SemErrs)
	}

	s.Update(cURI, "bar = 1\n", 2)
	s.analyze(s.Get(cURI))

	assertSemanticDiagnostic(t, s.Get(bURI), "cannot import name 'foo' from 'c'", 0, 14)
	assertSemanticDiagnostic(t, s.Get(aURI), "cannot import name 'foo' from 'b'", 0, 14)
}

func TestRefreshModuleAndDependents_CyclicImports(t *testing.T) {
	root := t.TempDir()
	aPath := filepath.Join(root, "a.py")
	bPath := filepath.Join(root, "b.py")

	aCode := "from b import bar\nfoo = 1\n"
	bCode := "from a import foo\nbar = 1\n"
	writeWorkspaceFile(t, aPath, aCode)
	writeWorkspaceFile(t, bPath, bCode)

	s := newWorkspaceServer(t, root)
	aURI := pathToURI(aPath)
	bURI := pathToURI(bPath)

	s.Open(lsp.TextDocumentItem{URI: aURI, Text: aCode, Version: 1})
	s.Open(lsp.TextDocumentItem{URI: bURI, Text: bCode, Version: 1})

	if got := len(s.Get(aURI).SemErrs); got != 0 {
		t.Fatalf("unexpected initial semantic errors in a.py: %+v", s.Get(aURI).SemErrs)
	}
	if got := len(s.Get(bURI).SemErrs); got != 0 {
		t.Fatalf("unexpected initial semantic errors in b.py: %+v", s.Get(bURI).SemErrs)
	}

	s.Update(aURI, "from b import bar\nbaz = 1\n", 2)
	s.analyze(s.Get(aURI))

	assertSemanticDiagnostic(t, s.Get(bURI), "cannot import name 'foo' from 'a'", 0, 14)
}

func TestDidCloseRebuildsDiskState_WithCyclicImports(t *testing.T) {
	root := t.TempDir()
	aPath := filepath.Join(root, "a.py")
	bPath := filepath.Join(root, "b.py")

	aCode := "from b import bar\nfoo = 1\n"
	bCode := "from a import foo\nbar = 1\n"
	writeWorkspaceFile(t, aPath, aCode)
	writeWorkspaceFile(t, bPath, bCode)

	s := newWorkspaceServer(t, root)
	aURI := pathToURI(aPath)
	bURI := pathToURI(bPath)

	s.Open(lsp.TextDocumentItem{URI: aURI, Text: aCode, Version: 1})
	s.Open(lsp.TextDocumentItem{URI: bURI, Text: bCode, Version: 1})

	if got := len(s.Get(aURI).SemErrs); got != 0 {
		t.Fatalf("unexpected initial semantic errors in a.py: %+v", s.Get(aURI).SemErrs)
	}
	if got := len(s.Get(bURI).SemErrs); got != 0 {
		t.Fatalf("unexpected initial semantic errors in b.py: %+v", s.Get(bURI).SemErrs)
	}

	s.Update(aURI, "from b import bar\nbaz = 1\n", 2)
	s.analyze(s.Get(aURI))
	assertSemanticDiagnostic(t, s.Get(bURI), "cannot import name 'foo' from 'a'", 0, 14)

	s.DidClose(&lsp.DidCloseTextDocumentParams{TextDocument: lsp.TextDocumentIdentifier{URI: aURI}})

	if s.Get(aURI) != nil {
		t.Fatal("expected a.py to be closed")
	}
	for _, err := range s.Get(bURI).SemErrs {
		if err.Msg == "cannot import name 'foo' from 'a'" {
			t.Fatalf("expected cyclic importer diagnostics to clear after closing edited module: %+v", s.Get(bURI).SemErrs)
		}
	}
}

func TestBuildModuleSnapshotExportHashIgnoresFunctionBodyChanges(t *testing.T) {
	s := New(nil)
	uri := lsp.DocumentURI("file:///mod.py")
	before := "def foo(x):\n    return x\n"
	after := "def foo(x):\n    y = x + 1\n    return y\n"

	beforeSnapshot := s.buildModuleSnapshot("mod", uri, "/tmp/mod.py", before, source.NewLineIndex(before))
	afterSnapshot := s.buildModuleSnapshot("mod", uri, "/tmp/mod.py", after, source.NewLineIndex(after))

	if beforeSnapshot.ExportHash == 0 || afterSnapshot.ExportHash == 0 {
		t.Fatal("expected non-zero export hash")
	}
	if beforeSnapshot.ExportHash != afterSnapshot.ExportHash {
		t.Fatalf("expected identical export hashes for body-only change: %d vs %d", beforeSnapshot.ExportHash, afterSnapshot.ExportHash)
	}
}

func TestBuildModuleSnapshotExportHashDetectsSignatureChanges(t *testing.T) {
	s := New(nil)
	uri := lsp.DocumentURI("file:///mod.py")
	before := "def foo(x):\n    return x\n"
	after := "def foo(x, y=1):\n    return x + y\n"

	beforeSnapshot := s.buildModuleSnapshot("mod", uri, "/tmp/mod.py", before, source.NewLineIndex(before))
	afterSnapshot := s.buildModuleSnapshot("mod", uri, "/tmp/mod.py", after, source.NewLineIndex(after))

	if beforeSnapshot.ExportHash == afterSnapshot.ExportHash {
		t.Fatalf("expected export hash to change after signature change: %d", beforeSnapshot.ExportHash)
	}
}

func TestRefreshModuleAndDependentsSkipsCascadeWhenExportsUnchanged(t *testing.T) {
	root := t.TempDir()
	aPath := filepath.Join(root, "a.py")
	bPath := filepath.Join(root, "b.py")
	cPath := filepath.Join(root, "c.py")

	writeWorkspaceFile(t, aPath, "def foo(x):\n    return x\n")
	writeWorkspaceFile(t, bPath, "from a import foo\n\ndef bar(v):\n    return foo(v)\n")
	writeWorkspaceFile(t, cPath, "from b import bar\n\nresult = bar(1)\n")

	s := newWorkspaceServer(t, root)
	beforeA := s.moduleSnapshotsByName["a"]
	beforeB := s.moduleSnapshotsByName["b"]
	beforeC := s.moduleSnapshotsByName["c"]
	if beforeA == nil || beforeB == nil || beforeC == nil {
		t.Fatalf("expected initial snapshots for a, b, c: %+v %+v %+v", beforeA, beforeB, beforeC)
	}

	writeWorkspaceFile(t, aPath, "def foo(x):\n    y = x + 1\n    return y\n")
	s.refreshModuleAndDependents(pathToURI(aPath))

	afterA := s.moduleSnapshotsByName["a"]
	afterB := s.moduleSnapshotsByName["b"]
	afterC := s.moduleSnapshotsByName["c"]

	if afterA == beforeA {
		t.Fatal("expected root module snapshot to be rebuilt")
	}
	if afterB != beforeB {
		t.Fatal("expected direct dependent snapshot to be reused when exports are unchanged")
	}
	if afterC != beforeC {
		t.Fatal("expected transitive dependent snapshot to be reused when exports are unchanged")
	}
}

func TestRefreshModuleAndDependentsRebuildsDependentsWhenExportsChange(t *testing.T) {
	root := t.TempDir()
	aPath := filepath.Join(root, "a.py")
	bPath := filepath.Join(root, "b.py")

	writeWorkspaceFile(t, aPath, "def foo(x):\n    return x\n")
	writeWorkspaceFile(t, bPath, "from a import foo\n\ndef bar(v):\n    return foo(v)\n")

	s := newWorkspaceServer(t, root)
	beforeB := s.moduleSnapshotsByName["b"]
	if beforeB == nil {
		t.Fatal("expected initial snapshot for b")
	}

	writeWorkspaceFile(t, aPath, "def foo(x, y=1):\n    return x + y\n")
	s.refreshModuleAndDependents(pathToURI(aPath))

	afterB := s.moduleSnapshotsByName["b"]
	if afterB == beforeB {
		t.Fatal("expected dependent snapshot to be rebuilt after export signature change")
	}
}

func newWorkspaceServer(t *testing.T, root string) *Server {
	t.Helper()

	s := New(nil)
	rootURI := pathToURI(root)
	if _, err := s.Initialize(&lsp.InitializeParams{RootURI: &rootURI}); err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	// Trigger background indexing and wait for completion
	s.Initialized(nil)
	if err := s.WaitForIndexing(); err != nil {
		t.Fatalf("indexing failed: %v", err)
	}

	return s
}

func newWorkspaceServerWithExternalRoots(t *testing.T, root string, roots ...string) *Server {
	t.Helper()
	s := newWorkspaceServer(t, root)
	s.indexMu.Lock()
	s.externalSearchRoots = append([]string(nil), roots...)
	s.externalModulesByName = make(map[string]ModuleFile)
	s.externalModulesByURI = make(map[lsp.DocumentURI]ModuleFile)
	s.indexMu.Unlock()
	return s
}

func writeWorkspaceFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
}

func mustDefinitionAt(t *testing.T, s *Server, uri lsp.DocumentURI, code string, line, char int) *lsp.Location {
	t.Helper()

	loc, err := s.Definition(&lsp.DefinitionParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Position:     lsp.Position{Line: line, Character: char},
	})
	if err != nil {
		t.Fatalf("unexpected definition error: %v", err)
	}
	if loc == nil {
		t.Fatal("expected definition location")
	}
	return loc
}

func mustHoverAt(t *testing.T, s *Server, uri lsp.DocumentURI, line, char int) *lsp.Hover {
	t.Helper()

	hov, err := s.Hover(&lsp.HoverParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
		Position:     lsp.Position{Line: line, Character: char},
	})
	if err != nil {
		t.Fatalf("unexpected hover error: %v", err)
	}
	if hov == nil {
		t.Fatal("expected hover")
	}
	return hov
}

func assertSemanticDiagnostic(t *testing.T, doc *Document, want string, line, char int) {
	t.Helper()

	if doc == nil {
		t.Fatal("expected document")
	}
	for _, err := range doc.SemErrs {
		if err.Msg == want {
			posLine, posChar := doc.LineIndex.OffsetToPosition(int(err.Span.Start))
			if posLine != line || posChar != char {
				t.Fatalf("unexpected diagnostic position for %q: got %d:%d want %d:%d", want, posLine, posChar, line, char)
			}
			return
		}
	}
	t.Fatalf("expected semantic diagnostic %q, got %+v", want, doc.SemErrs)
}
