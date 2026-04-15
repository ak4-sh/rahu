package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rahu/analyser"
	"rahu/lsp"
	ast "rahu/parser/ast"
	"rahu/source"
)

func TestComputeExportHashSkipsRevisitedClassSymbols(t *testing.T) {
	a := &analyser.Symbol{Name: "A", Kind: analyser.SymClass, ID: 1}
	b := &analyser.Symbol{Name: "B", Kind: analyser.SymClass, ID: 2}
	a.Members = analyser.NewScope(nil, analyser.ScopeMember)
	b.Members = analyser.NewScope(nil, analyser.ScopeMember)
	a.Members.Symbols[b.Name] = b
	b.Members.Symbols[a.Name] = a

	exports := map[string]*analyser.Symbol{
		"A": a,
		"B": b,
	}

	first := computeExportHash(exports)
	second := computeExportHash(exports)
	if first == 0 {
		t.Fatal("expected non-zero hash for cyclic exports")
	}
	if first != second {
		t.Fatalf("expected stable hash for cyclic exports, got %d then %d", first, second)
	}
}

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

func TestWorkspaceStubModulePrefersPyi(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.pyi"), "foo: int\n")
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 'py'\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from pkg.mod import foo\nfoo\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	loc := mustDefinitionAt(t, s, mainURI, mainCode, 1, 0)
	wantURI := pathToURI(filepath.Join(root, "pkg", "mod.pyi"))
	if loc.URI != wantURI {
		t.Fatalf("unexpected workspace stub module URI: got %q want %q", loc.URI, wantURI)
	}
}

func TestWorkspaceStubPackagePrefersInitPyi(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "__init__.pyi"), "foo: int\n")
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "__init__.py"), "foo = 'py'\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from pkg import foo\nfoo\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	loc := mustDefinitionAt(t, s, mainURI, mainCode, 1, 0)
	wantURI := pathToURI(filepath.Join(root, "pkg", "__init__.pyi"))
	if loc.URI != wantURI {
		t.Fatalf("unexpected workspace stub package URI: got %q want %q", loc.URI, wantURI)
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

func TestCompletionInheritedMembersFromImportedBase(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "class Bar:\n    def base(self):\n        pass\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from pkg.mod import Bar\n\nclass Foo(Bar):\n    pass\n\nitem = Foo()\nitem.\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: mainURI}, Position: lsp.Position{Line: 6, Character: 5}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "base")
}

func TestCompletionInheritedMembersFromImportedQualifiedBase(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "class Bar:\n    def base(self):\n        pass\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "import pkg.mod as mod\n\nclass Foo(mod.Bar):\n    pass\n\nitem = Foo()\nitem.\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: mainURI}, Position: lsp.Position{Line: 6, Character: 5}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "base")
}

func TestCompletionInheritedMembersFromImportedSubmoduleBase(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "__init__.py"), "")
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "submod.py"), "class Bar:\n    def base(self):\n        pass\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from pkg import submod\n\nclass Foo(submod.Bar):\n    pass\n\nitem = Foo()\nitem.\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: mainURI}, Position: lsp.Position{Line: 6, Character: 5}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "base")
}

func TestCompletionInheritedMembersFromExternalQualifiedBase(t *testing.T) {
	root := t.TempDir()
	extRoot := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(extRoot, "extpkg", "mod.py"), "class Bar:\n    def base(self):\n        pass\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "import extpkg.mod as mod\n\nclass Foo(mod.Bar):\n    pass\n\nitem = Foo()\nitem.\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServerWithExternalRoots(t, root, extRoot)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	items, err := s.Completion(&lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: mainURI}, Position: lsp.Position{Line: 6, Character: 5}})
	if err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	assertCompletionLabel(t, items, "base")
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

func TestDefinitionFromExternalStubModulePrefersPyi(t *testing.T) {
	root := t.TempDir()
	extRoot := filepath.Join(t.TempDir(), "site-packages")
	writeWorkspaceFile(t, filepath.Join(extRoot, "extpkg.pyi"), "value: int\n")
	writeWorkspaceFile(t, filepath.Join(extRoot, "extpkg.py"), "value = 'py'\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from extpkg import value\nvalue\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServerWithExternalRoots(t, root, extRoot)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	loc := mustDefinitionAt(t, s, mainURI, mainCode, 1, 0)
	wantURI := pathToURI(filepath.Join(extRoot, "extpkg.pyi"))
	if loc.URI != wantURI {
		t.Fatalf("unexpected external stub definition URI: got %q want %q", loc.URI, wantURI)
	}
}

func TestDefinitionFromExternalStubPackagePrefersInitPyi(t *testing.T) {
	root := t.TempDir()
	extRoot := filepath.Join(t.TempDir(), "site-packages")
	writeWorkspaceFile(t, filepath.Join(extRoot, "extpkg", "__init__.pyi"), "value: int\n")
	writeWorkspaceFile(t, filepath.Join(extRoot, "extpkg", "__init__.py"), "value = 'py'\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from extpkg import value\nvalue\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServerWithExternalRoots(t, root, extRoot)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	loc := mustDefinitionAt(t, s, mainURI, mainCode, 1, 0)
	wantURI := pathToURI(filepath.Join(extRoot, "extpkg", "__init__.pyi"))
	if loc.URI != wantURI {
		t.Fatalf("unexpected external stub package URI: got %q want %q", loc.URI, wantURI)
	}
}

func TestImportBuiltinModuleSysResolves(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.py")
	mainCode := "import sys\nsys\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	for _, err := range s.Get(mainURI).SemErrs {
		if err.Msg == "unresolved module: sys" || err.Msg == "undefined name: sys" {
			t.Fatalf("unexpected builtin import diagnostic: %+v", err)
		}
	}

	loc := mustDefinitionAt(t, s, mainURI, mainCode, 1, 0)
	if got := string(loc.URI); !strings.HasPrefix(got, "builtin:///") {
		t.Fatalf("unexpected builtin module URI: got %q", got)
	}
}

func TestFromImportBuiltinModuleMemberSysPathResolves(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from sys import path\npath\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	for _, err := range s.Get(mainURI).SemErrs {
		if err.Msg == "unresolved module: sys" || err.Msg == "cannot import name 'path' from 'sys'" || err.Msg == "undefined name: path" {
			t.Fatalf("unexpected builtin from-import diagnostic: %+v", err)
		}
	}

	loc := mustDefinitionAt(t, s, mainURI, mainCode, 1, 0)
	if got := string(loc.URI); !strings.HasPrefix(got, "builtin:///") {
		t.Fatalf("unexpected builtin member URI: got %q", got)
	}
}

func TestFromImportStdlibCollectionsOrderedDictResolves(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from collections import OrderedDict\nOrderedDict\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	for _, err := range s.Get(mainURI).SemErrs {
		if err.Msg == "cannot import name 'OrderedDict' from 'collections'" {
			t.Fatalf("unexpected stdlib import diagnostic: %+v", err)
		}
	}
}

func TestFromImportStdlibDatetimeDatetimeResolves(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from datetime import datetime\ndatetime\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	for _, err := range s.Get(mainURI).SemErrs {
		if err.Msg == "cannot import name 'datetime' from 'datetime'" {
			t.Fatalf("unexpected stdlib import diagnostic: %+v", err)
		}
	}
}

func TestImportedVersionStringSplitDoesNotReportUndefinedAttribute(t *testing.T) {
	root := t.TempDir()
	extRoot := filepath.Join(t.TempDir(), "site-packages")
	writeWorkspaceFile(t, filepath.Join(extRoot, "urllib3", "__init__.py"), "__version__ = \"1.26.0\"\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from urllib3 import __version__ as urllib3_version\n\nis_urllib3_1 = int(urllib3_version.split(\".\")[0]) == 1\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServerWithExternalRoots(t, root, extRoot)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	for _, err := range s.Get(mainURI).SemErrs {
		if err.Msg == "undefined attribute: split" || err.Msg == "undefined name: urllib3_version" {
			t.Fatalf("unexpected imported version diagnostic: %+v", err)
		}
	}
}

func TestNewStatementsDoNotProduceUnexpectedDiagnostics(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.py")
	mainCode := "flag = True\nitems = [1]\n\ndef outer():\n    value = 1\n    def inner():\n        nonlocal value\n        global flag\n        assert value, 'bad'\n        del items[0]\n        return value\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	for _, err := range s.Get(mainURI).SemErrs {
		if strings.Contains(err.Msg, "unexpected token") || strings.Contains(err.Msg, "undefined name") {
			t.Fatalf("unexpected semantic diagnostic: %+v", err)
		}
	}
}

func TestAdjacentParenthesizedFStringsDoNotProduceDiagnostics(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.py")
	mainCode := "class Auth:\n    def __init__(self, username):\n        self.username = username\n\n    def build(self, realm, nonce, path, respdig):\n        base = (\n            f'username=\"{self.username}\", realm=\"{realm}\", nonce=\"{nonce}\", ' \n            f'uri=\"{path}\", response=\"{respdig}\"'\n        )\n        return base\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	doc := s.Get(mainURI)
	if doc == nil {
		t.Fatal("expected open document")
	}
	if len(doc.SemErrs) != 0 {
		t.Fatalf("unexpected semantic diagnostics: %+v", doc.SemErrs)
	}
	if doc.Tree == nil {
		t.Fatal("expected parsed tree")
	}

	classNode := doc.Tree.Children(doc.Tree.Root)[0]
	_, _, classBody := doc.Tree.ClassParts(classNode)
	fn := doc.Tree.Children(classBody)[1]
	_, _, body := doc.Tree.FunctionParts(fn)
	bodyStmt := doc.Tree.Children(body)[0]
	assignKids := doc.Tree.Children(bodyStmt)
	if len(assignKids) != 2 {
		t.Fatalf("unexpected assignment children: %d", len(assignKids))
	}
	if doc.Tree.Node(assignKids[0]).Kind != ast.NodeFString {
		t.Fatalf("expected merged f-string assignment value, got %s", doc.Tree.Node(assignKids[0]).Kind)
	}
}

func TestResponseLinksStyleMethodWithBlankLineBeforeReturnDoesNotProduceDiagnostics(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.py")
	mainCode := "class Response:\n    @property\n    def links(self):\n        header = self.headers.get(\"link\")\n\n        resolved_links = {}\n\n        if header:\n            links = parse_header_links(header)\n\n            for link in links:\n                key = link.get(\"rel\") or link.get(\"url\")\n                resolved_links[key] = link\n\n        return resolved_links\n"
	writeWorkspaceFile(t, filepath.Join(root, "helpers.py"), "def parse_header_links(value):\n    return []\n")
	writeWorkspaceFile(t, mainPath, "from helpers import parse_header_links\n\n"+mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: "from helpers import parse_header_links\n\n" + mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	doc := s.Get(mainURI)
	if doc == nil {
		t.Fatal("expected open document")
	}
	for _, err := range doc.SemErrs {
		if strings.Contains(err.Msg, "unexpected token") {
			t.Fatalf("unexpected semantic diagnostic: %+v", err)
		}
	}
}

func TestWarningMessageFStringConversionDoesNotProduceParseErrors(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.py")
	mainCode := "def build_warning(username):\n    return (\n        \"Non-string usernames will no longer be supported in Requests \"\n        f\"3.0.0. Please convert the object you've passed in ({username!r}) to \"\n        \"a string or bytes object in the near future to avoid \"\n        \"problems.\"\n    )\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	snapshot := s.buildBaseModuleSnapshot("main", mainURI, mainPath, mainCode, source.NewLineIndex(mainCode))
	if snapshot == nil {
		t.Fatal("expected module snapshot")
	}
	if len(snapshot.ParseErrs) != 0 {
		t.Fatalf("unexpected parse errors: %+v", snapshot.ParseErrs)
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

func TestFromImportExternalModuleWithUnionAnnotations(t *testing.T) {
	root := t.TempDir()
	extRoot := filepath.Join(t.TempDir(), "site-packages")
	writeWorkspaceFile(t, filepath.Join(extRoot, "extpkg", "__init__.py"), "")
	writeWorkspaceFile(t, filepath.Join(extRoot, "extpkg", "poolmanager.py"), "Alias: int | None = None\n\nclass PoolManager:\n    pass\n\ndef proxy_from_url(url: str | None = None):\n    return url\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from extpkg.poolmanager import PoolManager, proxy_from_url\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServerWithExternalRoots(t, root, extRoot)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	for _, err := range s.Get(mainURI).SemErrs {
		if err.Msg == "cannot import name 'PoolManager' from 'extpkg.poolmanager'" || err.Msg == "cannot import name 'proxy_from_url' from 'extpkg.poolmanager'" {
			t.Fatalf("unexpected missing import diagnostic: %+v", err)
		}
	}
}

func TestFromImportExternalPackageSubmodule(t *testing.T) {
	root := t.TempDir()
	extRoot := filepath.Join(t.TempDir(), "site-packages")
	writeWorkspaceFile(t, filepath.Join(extRoot, "extpkg", "__init__.py"), "")
	writeWorkspaceFile(t, filepath.Join(extRoot, "extpkg", "poolmanager.py"), "class PoolManager:\n    pass\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from extpkg import poolmanager\npoolmanager\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServerWithExternalRoots(t, root, extRoot)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	for _, err := range s.Get(mainURI).SemErrs {
		if err.Msg == "cannot import name 'poolmanager' from 'extpkg'" {
			t.Fatalf("unexpected missing submodule diagnostic: %+v", err)
		}
	}

	loc := mustDefinitionAt(t, s, mainURI, mainCode, 1, 0)
	wantURI := pathToURI(filepath.Join(extRoot, "extpkg", "poolmanager.py"))
	if loc.URI != wantURI {
		t.Fatalf("unexpected submodule definition URI: got %q want %q", loc.URI, wantURI)
	}
}

func TestFromImportExternalModuleParenthesizedNames(t *testing.T) {
	root := t.TempDir()
	extRoot := filepath.Join(t.TempDir(), "site-packages")
	writeWorkspaceFile(t, filepath.Join(extRoot, "extpkg", "exceptions.py"), "class ClosedPoolError:\n    pass\n\nclass ConnectTimeoutError:\n    pass\n\nclass MaxRetryError:\n    pass\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from extpkg.exceptions import (\n    ClosedPoolError,\n    ConnectTimeoutError,\n    MaxRetryError,\n)\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServerWithExternalRoots(t, root, extRoot)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	for _, err := range s.Get(mainURI).SemErrs {
		if err.Msg == "cannot import name 'ClosedPoolError' from 'extpkg.exceptions'" || err.Msg == "cannot import name 'ConnectTimeoutError' from 'extpkg.exceptions'" || err.Msg == "cannot import name 'MaxRetryError' from 'extpkg.exceptions'" {
			t.Fatalf("unexpected missing import diagnostic: %+v", err)
		}
	}
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

func TestStarImportFromWorkspaceModuleBindsExportedNames(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\n_hidden = 2\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from pkg.mod import *\nfoo\n_hidden\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	for _, err := range s.Get(mainURI).SemErrs {
		if err.Msg == "undefined name: foo" {
			t.Fatalf("unexpected undefined imported name: %+v", err)
		}
	}
	assertSemanticDiagnostic(t, s.Get(mainURI), "undefined name: _hidden", 2, 0)

	loc := mustDefinitionAt(t, s, mainURI, mainCode, 1, 0)
	wantURI := pathToURI(filepath.Join(root, "pkg", "mod.py"))
	if loc.URI != wantURI {
		t.Fatalf("unexpected star import definition URI: got %q want %q", loc.URI, wantURI)
	}
}

func TestStarImportUsesStaticAllWhenPresent(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\nbar = 2\n__all__ = ['bar']\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from pkg.mod import *\nfoo\nbar\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	assertSemanticDiagnostic(t, s.Get(mainURI), "undefined name: foo", 1, 0)
	for _, err := range s.Get(mainURI).SemErrs {
		if err.Msg == "undefined name: bar" {
			t.Fatalf("unexpected undefined __all__ import: %+v", err)
		}
	}
}

func TestStarImportFallsBackWhenAllIsNotStatic(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(root, "pkg", "mod.py"), "foo = 1\nbar = 2\n__all__ = names\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from pkg.mod import *\nfoo\nbar\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServer(t, root)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	for _, err := range s.Get(mainURI).SemErrs {
		if err.Msg == "undefined name: foo" || err.Msg == "undefined name: bar" {
			t.Fatalf("unexpected undefined fallback import: %+v", err)
		}
	}
}

func TestStarImportFromExternalModuleBindsExportedNames(t *testing.T) {
	root := t.TempDir()
	extRoot := filepath.Join(t.TempDir(), "site-packages")
	writeWorkspaceFile(t, filepath.Join(extRoot, "extpkg", "__init__.py"), "")
	writeWorkspaceFile(t, filepath.Join(extRoot, "extpkg", "mod.py"), "foo = 1\n_hidden = 2\n")
	mainPath := filepath.Join(root, "main.py")
	mainCode := "from extpkg.mod import *\nfoo\n_hidden\n"
	writeWorkspaceFile(t, mainPath, mainCode)

	s := newWorkspaceServerWithExternalRoots(t, root, extRoot)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))

	for _, err := range s.Get(mainURI).SemErrs {
		if err.Msg == "undefined name: foo" {
			t.Fatalf("unexpected undefined imported name: %+v", err)
		}
	}
	assertSemanticDiagnostic(t, s.Get(mainURI), "undefined name: _hidden", 2, 0)
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

func mustDefinitionAt(t *testing.T, s *Server, uri lsp.DocumentURI, _ string, line, char int) *lsp.Location {
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
