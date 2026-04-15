package server

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"rahu/analyser"
	"rahu/lsp"
	"rahu/parser"
	"rahu/parser/ast"
	l "rahu/server/locate"
	"rahu/source"
)

func generatePythonLines(lineCount int) string {
	var sb strings.Builder

	for i := range lineCount {
		switch i % 5 {
		case 0:
			sb.WriteString("class Class")
			sb.WriteString(string(rune('A' + i%26)))
			sb.WriteString(":\n")
			sb.WriteString("    def __init__(self):\n")
			sb.WriteString("        self.x = 1\n")
			sb.WriteString("        self.y = 2\n")
		case 1:
			sb.WriteString("def func")
			sb.WriteString(string(rune('A' + i%26)))
			sb.WriteString("(a, b):\n")
			sb.WriteString("    return a + b\n")
		case 2:
			sb.WriteString("x")
			sb.WriteString(string(rune('a' + i%26)))
			sb.WriteString(" = ")
			sb.WriteString(string(rune('a' + i%26)))
			sb.WriteString(" + 1\n")
		case 3:
			sb.WriteString("if ")
			sb.WriteString(string(rune('a' + i%26)))
			sb.WriteString(" > 0:\n")
			sb.WriteString("    print(")
			sb.WriteString(string(rune('a' + i%26)))
			sb.WriteString(")\n")
		case 4:
			sb.WriteString("for i in range(10):\n")
			sb.WriteString("    x = i * 2\n")
		}
	}

	return sb.String()
}

func generateMediumPython() string {
	var sb strings.Builder
	sb.WriteString("import os\n")
	sb.WriteString("import sys\n\n")

	for i := range 20 {
		sb.WriteString("class Class")
		sb.WriteString(string(rune('A' + i%26)))
		sb.WriteString(":\n")
		sb.WriteString("    def __init__(self):\n")
		sb.WriteString("        self.x = 1\n")
		sb.WriteString("        self.y = 2\n")
		sb.WriteString("    def method(self):\n")
		sb.WriteString("        return self.x + self.y\n\n")
	}

	for i := range 30 {
		sb.WriteString("def func")
		sb.WriteString(string(rune('A' + i%26)))
		sb.WriteString("(a, b):\n")
		sb.WriteString("    result = a + b\n")
		sb.WriteString("    if result > 0:\n")
		sb.WriteString("        return result\n")
		sb.WriteString("    return 0\n\n")
	}

	sb.WriteString("x = 1\n")
	sb.WriteString("y = 2\n")
	sb.WriteString("z = x + y\n")
	sb.WriteString("print(z)\n")

	return sb.String()
}

func generateLargePython() string {
	var sb strings.Builder

	for i := range 100 {
		sb.WriteString("class Class")
		sb.WriteString(string(rune('A' + i%26)))
		sb.WriteString(string(rune('A' + (i/26)%26)))
		sb.WriteString(":\n")
		sb.WriteString("    def __init__(self, x, y):\n")
		sb.WriteString("        self.x = x\n")
		sb.WriteString("        self.y = y\n")
		sb.WriteString("        self.result = x + y\n")
		sb.WriteString("    def add(self):\n")
		sb.WriteString("        return self.x + self.y\n")
		sb.WriteString("    def sub(self):\n")
		sb.WriteString("        return self.x - self.y\n")
		sb.WriteString("    def mul(self):\n")
		sb.WriteString("        return self.x * self.y\n")
		sb.WriteString("    def div(self):\n")
		sb.WriteString("        if self.y != 0:\n")
		sb.WriteString("            return self.x / self.y\n")
		sb.WriteString("        return 0\n\n")
	}

	for i := range 150 {
		sb.WriteString("def func")
		sb.WriteString(string(rune('A' + i%26)))
		sb.WriteString(string(rune('A' + (i/26)%26)))
		sb.WriteString("(a, b, c=10):\n")
		sb.WriteString("    x = a + b\n")
		sb.WriteString("    y = b * c\n")
		sb.WriteString("    if x > y:\n")
		sb.WriteString("        return x - y\n")
		sb.WriteString("    elif y > x:\n")
		sb.WriteString("        return y - x\n")
		sb.WriteString("    return 0\n\n")
	}

	sb.WriteString("x = 1\n")
	sb.WriteString("y = 2\n")
	sb.WriteString("z = x + y\n")
	sb.WriteString("w = x * y\n")
	sb.WriteString("v = x - y\n")
	sb.WriteString("print(x, y, z, w, v)\n")

	return sb.String()
}

func TestNameAtPos(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		line         int // 1-based (human)
		col          int // 1-based (human)
		expectedName string
	}{
		{"simple variable reference", "x = 1\ny = x", 2, 5, "x"},
		{"variable on first line", "foo = 42", 1, 1, "foo"},
		{"name in binary operation", "a = 1\nb = 2\nc = a + b", 3, 5, "a"},
		{"name in function call", "x = 1\nprint(x)", 2, 7, "x"},
		{"name in comparison", "x = 1\nif x > 0:\n    pass", 2, 4, "x"},
		{"name in while loop", "x = 1\nwhile x < 10:\n    x = x + 1", 2, 7, "x"},
		{"name in list", "x = 1\ny = 2\nz = [x, y]", 3, 6, "x"},
		{"name in tuple", "x = 1\ny = (x, 2)", 2, 6, "x"},
		{"name in boolean operation", "x = True\ny = False\nz = x and y", 3, 5, "x"},
		{"name in function default argument", "default_val = 10\ndef foo(x=default_val):\n    pass", 2, 14, "default_val"},
		{"name in partial function default argument", "def foo(x=default_val)", 1, 11, "default_val"},
		{"name in partial class base", "class Foo(Bar)", 1, 11, "Bar"},
		{"position outside any name", "x = 1", 1, 10, ""},
		{"empty module", "", 1, 1, ""},
		{"name at boundary", "xyz = 1", 1, 3, "xyz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(tt.code)
			tree := p.Parse()
			li := source.NewLineIndex(tt.code)

			offset := li.PositionToOffset(tt.line-1, tt.col-1)

			name := l.NameAtPos(tree, offset)

			if tt.expectedName == "" {
				if name != ast.NoNode {
					got, _ := tree.NameText(name)
					t.Errorf("expected nil, got %q", got)
				}
				return
			}

			if name == ast.NoNode {
				t.Fatalf("expected %q, got nil", tt.expectedName)
			}

			got, _ := tree.NameText(name)
			if got != tt.expectedName {
				t.Errorf("expected %q, got %q", tt.expectedName, got)
			}
		})
	}
}

func TestNameAtPos_NilModule(t *testing.T) {
	if l.NameAtPos(nil, 0) != ast.NoNode {
		t.Error("expected nil for nil module")
	}
}

func TestContains(t *testing.T) {
	code := "hello\nworld"
	li := source.NewLineIndex(code)

	start := li.PositionToOffset(0, 0)
	end := li.PositionToOffset(0, 5)

	rng := ast.Range{Start: uint32(start), End: uint32(end)}

	tests := []struct {
		name     string
		line     int
		char     int
		expected bool
	}{
		{"inside", 0, 2, true},
		{"start boundary", 0, 0, true},
		{"end boundary", 0, 5, true},
		{"outside", 1, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos := li.PositionToOffset(tt.line, tt.char)
			if l.Contains(rng, pos) != tt.expected {
				t.Fatalf("unexpected result")
			}
		})
	}
}

func TestDefinition(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		line         int
		character    int
		expectError  bool
		expectedLine int
		expectedChar int
		expectNilDoc bool
	}{
		{"goto variable definition", "x = 1\ny = x", 1, 4, false, 0, 0, false},
		{"goto function definition", "def foo():\n    pass\nfoo()", 2, 0, false, 0, 4, false},
		{"builtin returns error", "print(1)", 0, 0, true, 0, 0, false},
		{"nil document", "", 0, 0, true, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{docs: map[lsp.DocumentURI]*Document{}}
			uri := lsp.DocumentURI("file:///test.py")

			if !tt.expectNilDoc {
				p := parser.New(tt.code)
				tree := p.Parse()
				global, _ := analyser.BuildScopes(tree, tt.code)
				resolver, _ := analyser.Resolve(tree, global)

				s.docs[uri] = &Document{
					URI:       uri,
					Version:   1,
					Text:      tt.code,
					LineIndex: source.NewLineIndex(tt.code),
					Tree:      tree,
					Symbols:   resolver.Resolved,
				}
			}

			loc, err := s.Definition(&lsp.DefinitionParams{
				TextDocument: lsp.TextDocumentIdentifier{URI: uri},
				Position: lsp.Position{
					Line:      tt.line,
					Character: tt.character,
				},
			})

			if tt.expectError {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if loc.Range.Start.Line != tt.expectedLine {
				t.Errorf("expected line %d, got %d", tt.expectedLine, loc.Range.Start.Line)
			}

			if loc.Range.Start.Character != tt.expectedChar {
				t.Errorf("expected char %d, got %d", tt.expectedChar, loc.Range.Start.Character)
			}
		})
	}
}

func TestDefinition_Shadowing(t *testing.T) {
	code := `x = 1
def foo():
    x = 2
    return x
y = x
`

	s := &Server{docs: make(map[lsp.DocumentURI]*Document)}

	uri := lsp.DocumentURI("file:///test.py")

	p := parser.New(code)
	tree := p.Parse()
	global, _ := analyser.BuildScopes(tree, code)
	resolver, _ := analyser.Resolve(tree, global)

	s.docs[uri] = &Document{
		URI:       uri,
		Version:   1,
		Text:      code,
		LineIndex: source.NewLineIndex(code),
		Tree:      tree,
		Symbols:   resolver.Resolved,
	}

	tests := []struct {
		line         int
		character    int
		expectedLine int
		expectedChar int
	}{
		{3, 11, 2, 4},
		{4, 4, 0, 0},
	}

	for _, tt := range tests {
		params := &lsp.DefinitionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			Position: lsp.Position{
				Line:      tt.line,
				Character: tt.character,
			},
		}

		loc, err := s.Definition(params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if loc.Range.Start.Line != tt.expectedLine {
			t.Fatalf("expected line %d, got %d", tt.expectedLine, loc.Range.Start.Line)
		}
	}
}

var (
	smallCode      = generatePythonLines(50)
	mediumCode     = generateMediumPython()
	largeCode      = generateLargePython()
	extraLargeCode = generatePythonLines(5000)
)

const (
	benchWorkspaceSmall  = 48
	benchWorkspaceMedium = 96
	benchWorkspaceLarge  = 192
)

// Benchmark tiers:
//
// Fast:
//   Safe for routine local runs. Intended command:
//   go test ./server -run=^$ -bench='Fast' -benchmem
//
// Medium:
//   Manual comparison runs that exercise larger workspaces and cache limits.
//   Intended command:
//   go test ./server -run=^$ -bench='Medium' -benchmem -benchtime=3x
//
// Stress:
//   Expensive opt-in runs for cache pressure and indexing churn.
//   Intended command:
//   go test ./server -run=^$ -bench='Stress' -benchmem -benchtime=1x
//
// Custom metrics:
//   resident_modules - number of workspace snapshots still resident after the benchmarked operation.
//   refindex_uris    - number of URIs retained in the reference index after cache pressure.

func writeBenchmarkFile(b *testing.B, path, content string) {
	b.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		b.Fatalf("write failed: %v", err)
	}
}

func benchmarkModuleCode(name string, imports []string) string {
	var sb strings.Builder
	for _, imp := range imports {
		sb.WriteString("from ")
		sb.WriteString(imp)
		sb.WriteString(" import func_0\n")
	}
	sb.WriteString("\n")
	for i := range 4 {
		sb.WriteString(fmt.Sprintf("class %sClass%d:\n", name, i))
		sb.WriteString("    def __init__(self, x, y):\n")
		sb.WriteString("        self.x = x\n")
		sb.WriteString("        self.y = y\n")
		sb.WriteString("    def total(self):\n")
		sb.WriteString("        return self.x + self.y\n\n")
	}
	for i := range 8 {
		sb.WriteString(fmt.Sprintf("def func_%d(value, step=%d):\n", i, i+1))
		sb.WriteString("    current = value + step\n")
		if len(imports) > 0 {
			sb.WriteString("    current = current + func_0(value)\n")
		}
		sb.WriteString("    return current\n\n")
	}
	sb.WriteString(fmt.Sprintf("CONSTANT_%s = func_0(1)\n", strings.ToUpper(name)))
	return sb.String()
}

func createBenchmarkWorkspace(b *testing.B, moduleCount, importsPerModule int) string {
	b.Helper()
	root := b.TempDir()
	for i := range moduleCount {
		name := fmt.Sprintf("m%03d", i)
		imports := make([]string, 0, importsPerModule)
		for j := 1; j <= importsPerModule && i-j >= 0; j++ {
			imports = append(imports, fmt.Sprintf("m%03d", i-j))
		}
		writeBenchmarkFile(b, filepath.Join(root, name+".py"), benchmarkModuleCode(name, imports))
	}
	return root
}

type startupReadinessMetrics struct {
	priorityReadyNS int64
	allOpenReadyNS  int64
	priorityModules int
	priorityRounds  int
	residentModules int
}

func createStartupReadinessWorkspaceShallow(b *testing.B, unrelatedCount int) (string, map[string]string) {
	b.Helper()
	root := b.TempDir()
	libCode := benchmarkModuleCode("lib", nil)
	mainCode := "from lib import func_0\nresult = func_0(1)\n"
	writeBenchmarkFile(b, filepath.Join(root, "lib.py"), libCode)
	writeBenchmarkFile(b, filepath.Join(root, "main.py"), mainCode)
	for i := range unrelatedCount {
		name := fmt.Sprintf("u%03d", i)
		writeBenchmarkFile(b, filepath.Join(root, name+".py"), benchmarkModuleCode(name, nil))
	}
	return root, map[string]string{"main.py": mainCode}
}

func createStartupReadinessWorkspaceDeep(b *testing.B, depth, unrelatedCount int) (string, map[string]string) {
	b.Helper()
	root := b.TempDir()
	for i := depth - 1; i >= 0; i-- {
		name := fmt.Sprintf("dep%03d", i)
		imports := []string(nil)
		if i+1 < depth {
			imports = []string{fmt.Sprintf("dep%03d", i+1)}
		}
		writeBenchmarkFile(b, filepath.Join(root, name+".py"), benchmarkModuleCode(name, imports))
	}
	mainCode := "from dep000 import func_0\nresult = func_0(1)\n"
	writeBenchmarkFile(b, filepath.Join(root, "main.py"), mainCode)
	for i := range unrelatedCount {
		name := fmt.Sprintf("u%03d", i)
		imports := []string(nil)
		if i > 0 {
			imports = []string{fmt.Sprintf("u%03d", i-1)}
		}
		writeBenchmarkFile(b, filepath.Join(root, name+".py"), benchmarkModuleCode(name, imports))
	}
	return root, map[string]string{"main.py": mainCode}
}

func createStartupReadinessWorkspaceShared(b *testing.B, unrelatedCount int) (string, map[string]string) {
	b.Helper()
	root := b.TempDir()
	writeBenchmarkFile(b, filepath.Join(root, "core.py"), benchmarkModuleCode("core", nil))
	writeBenchmarkFile(b, filepath.Join(root, "util.py"), benchmarkModuleCode("util", nil))
	main1 := "from core import func_0\nfrom util import func_0 as util_func\nresult = func_0(1) + util_func(2)\n"
	main2 := "from util import func_0\nfrom core import func_0 as core_func\nvalue = func_0(1) + core_func(2)\n"
	writeBenchmarkFile(b, filepath.Join(root, "main1.py"), main1)
	writeBenchmarkFile(b, filepath.Join(root, "main2.py"), main2)
	for i := range unrelatedCount {
		name := fmt.Sprintf("u%03d", i)
		imports := []string(nil)
		if i%5 == 0 && i > 0 {
			imports = []string{fmt.Sprintf("u%03d", i-1)}
		}
		writeBenchmarkFile(b, filepath.Join(root, name+".py"), benchmarkModuleCode(name, imports))
	}
	return root, map[string]string{"main1.py": main1, "main2.py": main2}
}

func createStartupReadinessWorkspaceSparse(b *testing.B, unrelatedCount int) (string, map[string]string) {
	b.Helper()
	root := b.TempDir()
	writeBenchmarkFile(b, filepath.Join(root, "core.py"), benchmarkModuleCode("core", nil))
	writeBenchmarkFile(b, filepath.Join(root, "helper.py"), benchmarkModuleCode("helper", []string{"core"}))
	writeBenchmarkFile(b, filepath.Join(root, "util.py"), benchmarkModuleCode("util", []string{"core"}))
	mainCode := "from helper import func_0\nfrom util import func_0 as util_func\nresult = func_0(1) + util_func(2)\n"
	writeBenchmarkFile(b, filepath.Join(root, "main.py"), mainCode)
	for i := range unrelatedCount {
		name := fmt.Sprintf("u%03d", i)
		imports := make([]string, 0, 2)
		if i > 0 {
			imports = append(imports, fmt.Sprintf("u%03d", i-1))
		}
		if i > 2 && i%7 == 0 {
			imports = append(imports, fmt.Sprintf("u%03d", i-3))
		}
		writeBenchmarkFile(b, filepath.Join(root, name+".py"), benchmarkModuleCode(name, imports))
	}
	return root, map[string]string{"main.py": mainCode}
}

func openBenchmarkWorkspaceFiles(b *testing.B, s *Server, root string, openFiles map[string]string) []lsp.DocumentURI {
	b.Helper()
	names := make([]string, 0, len(openFiles))
	for name := range openFiles {
		names = append(names, name)
	}
	sort.Strings(names)
	uris := make([]lsp.DocumentURI, 0, len(names))
	for i, name := range names {
		uri := pathToURI(filepath.Join(root, name))
		s.Open(lsp.TextDocumentItem{URI: uri, Text: openFiles[name], Version: i + 1})
		uris = append(uris, uri)
	}
	return uris
}

func benchmarkStartupReadinessOnce(b *testing.B, root string, openFiles map[string]string, maxCachedModules int) startupReadinessMetrics {
	b.Helper()
	s := newBenchmarkWorkspaceServer(b, root, maxCachedModules)
	_ = openBenchmarkWorkspaceFiles(b, s, root, openFiles)
	started := time.Now()
	s.buildWorkspaceSnapshots()
	if s.startup == nil {
		b.Fatal("expected startup readiness state")
	}
	if len(openFiles) != 0 {
		if s.startup.priorityReadyAt.IsZero() {
			b.Fatal("expected priority readiness timestamp")
		}
		if s.startup.allOpenFilesReadyAt.IsZero() {
			b.Fatal("expected all-open-files readiness timestamp")
		}
	}
	metrics := startupReadinessMetrics{
		priorityModules: s.startup.priorityModuleCount,
		priorityRounds:  s.startup.prioritySurfaceRounds,
		residentModules: benchmarkResidentSnapshotCount(s),
	}
	if !s.startup.priorityReadyAt.IsZero() {
		metrics.priorityReadyNS = s.startup.priorityReadyAt.Sub(started).Nanoseconds()
	}
	if !s.startup.allOpenFilesReadyAt.IsZero() {
		metrics.allOpenReadyNS = s.startup.allOpenFilesReadyAt.Sub(started).Nanoseconds()
	}
	if metrics.priorityModules == 0 {
		b.Fatal("expected non-zero priority module count")
	}
	return metrics
}

func runStartupReadinessBenchmark(b *testing.B, root string, openFiles map[string]string, maxCachedModules int) {
	b.Helper()
	var last startupReadinessMetrics
	b.ResetTimer()
	for b.Loop() {
		last = benchmarkStartupReadinessOnce(b, root, openFiles, maxCachedModules)
	}
	b.ReportMetric(float64(last.priorityReadyNS), "priority_ready_ns")
	b.ReportMetric(float64(last.allOpenReadyNS), "all_open_ready_ns")
	b.ReportMetric(float64(last.priorityModules), "priority_modules")
	b.ReportMetric(float64(last.priorityRounds), "priority_rounds")
	b.ReportMetric(float64(last.residentModules), "resident_modules")
}

func newBenchmarkWorkspaceServer(b *testing.B, root string, maxCachedModules int) *Server {
	b.Helper()
	s := New(nil)
	s.rootPath = root
	s.maxCachedModules = maxCachedModules
	s.buildModuleIndex()
	return s
}

func benchmarkWarmAllModules(b *testing.B, s *Server) []ModuleFile {
	b.Helper()
	s.indexMu.RLock()
	mods := make([]ModuleFile, 0, len(s.modulesByName))
	for _, mod := range s.modulesByName {
		mods = append(mods, mod)
	}
	s.indexMu.RUnlock()
	for _, mod := range mods {
		if _, ok := s.analyzeModuleFile(mod); !ok {
			b.Fatalf("failed to analyze module %s", mod.Name)
		}
	}
	return mods
}

func benchmarkResidentSnapshotCount(s *Server) int {
	s.snapshotsMu.RLock()
	defer s.snapshotsMu.RUnlock()
	return len(s.moduleSnapshotsByURI)
}

func benchmarkRefIndexURIEntryCount(s *Server) int {
	s.refIndex.mu.RLock()
	defer s.refIndex.mu.RUnlock()
	return len(s.refIndex.byURI)
}

func benchmarkEvictModule(b *testing.B, s *Server, target string, mods []ModuleFile) {
	b.Helper()
	for _, mod := range mods {
		if mod.Name == target {
			continue
		}
		if _, ok := s.analyzeModuleFile(mod); !ok {
			b.Fatalf("failed to analyze module %s during eviction", mod.Name)
		}
		if _, ok := s.getModuleSnapshotByName(target); !ok {
			return
		}
	}
	b.Fatalf("failed to evict module %s", target)
}

func BenchmarkServerStartup(b *testing.B) {
	for b.Loop() {
		s := New(nil)
		_ = s
	}
}

func BenchmarkAnalysisSmall(b *testing.B) {
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")

	for b.Loop() {
		s.Open(lsp.TextDocumentItem{URI: uri, Text: smallCode, Version: 1})
		doc := s.Get(uri)
		s.analyze(doc)
		s.Close(uri)
	}
}

func BenchmarkAnalysisMedium(b *testing.B) {
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")

	for b.Loop() {
		s.Open(lsp.TextDocumentItem{URI: uri, Text: mediumCode, Version: 1})
		doc := s.Get(uri)
		s.analyze(doc)
		s.Close(uri)
	}
}

func BenchmarkAnalysisLarge(b *testing.B) {
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")

	for b.Loop() {
		s.Open(lsp.TextDocumentItem{URI: uri, Text: largeCode, Version: 1})
		doc := s.Get(uri)
		s.analyze(doc)
		s.Close(uri)
	}
}

func BenchmarkAnalysisExtraLarge(b *testing.B) {
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")

	for b.Loop() {
		s.Open(lsp.TextDocumentItem{URI: uri, Text: extraLargeCode, Version: 1})
		doc := s.Get(uri)
		s.analyze(doc)
		s.Close(uri)
	}
}

func BenchmarkDefinitionLookup(b *testing.B) {
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: mediumCode, Version: 1})
	doc := s.Get(uri)
	s.analyze(doc)

	tree := doc.Tree

	b.ResetTimer()
	for b.Loop() {
		name := l.NameAtPos(tree, 500)
		if name != ast.NoNode {
			_ = name
		}
	}

	s.Close(uri)
}

func BenchmarkHoverLookup(b *testing.B) {
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: mediumCode, Version: 1})
	doc := s.Get(uri)
	s.analyze(doc)

	b.ResetTimer()
	for b.Loop() {
		offset := 500
		name := l.NameAtPos(doc.Tree, offset)
		if name != ast.NoNode {
			sym := doc.Symbols[name]
			if sym != nil {
				_ = sym
			}
		}
	}

	s.Close(uri)
}

func BenchmarkThroughputAnalysisSmall(b *testing.B) {
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")

	for b.Loop() {
		s.Open(lsp.TextDocumentItem{URI: uri, Text: smallCode, Version: 1})
		doc := s.Get(uri)
		s.analyze(doc)
	}
	s.Close(uri)
}

func BenchmarkThroughputAnalysisMedium(b *testing.B) {
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")

	for b.Loop() {
		s.Open(lsp.TextDocumentItem{URI: uri, Text: mediumCode, Version: 1})
		doc := s.Get(uri)
		s.analyze(doc)
	}
	s.Close(uri)
}

func BenchmarkThroughputAnalysisLarge(b *testing.B) {
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")

	for b.Loop() {
		s.Open(lsp.TextDocumentItem{URI: uri, Text: largeCode, Version: 1})
		doc := s.Get(uri)
		s.analyze(doc)
	}
	s.Close(uri)
}

func BenchmarkDefinitionLookupAll(b *testing.B) {
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: mediumCode, Version: 1})
	doc := s.Get(uri)
	s.analyze(doc)

	tree := doc.Tree

	positions := []int{100, 250, 500, 750, 1000, 1500, 2000, 2500, 3000}

	b.ResetTimer()
	for b.Loop() {
		for _, pos := range positions {
			name := l.NameAtPos(tree, pos)
			if name != ast.NoNode {
				_ = name
			}
		}
	}

	s.Close(uri)
}

func BenchmarkHoverLookupAll(b *testing.B) {
	s := New(nil)
	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{URI: uri, Text: mediumCode, Version: 1})
	doc := s.Get(uri)
	s.analyze(doc)

	offsets := []int{100, 250, 500, 750, 1000, 1500, 2000, 2500, 3000}

	b.ResetTimer()
	for b.Loop() {
		for _, offset := range offsets {
			name := l.NameAtPos(doc.Tree, offset)
			if name != ast.NoNode {
				sym := doc.Symbols[name]
				if sym != nil {
					_ = sym
				}
			}
		}
	}

	s.Close(uri)
}

func BenchmarkColdStartAnalysis(b *testing.B) {
	for b.Loop() {
		s := New(nil)
		uri := lsp.DocumentURI("file:///test.py")
		s.Open(lsp.TextDocumentItem{URI: uri, Text: smallCode, Version: 1})
		doc := s.Get(uri)
		s.analyze(doc)
		s.Close(uri)
	}
}

func BenchmarkParserOnly(b *testing.B) {
	input := largeCode

	b.ResetTimer()
	for b.Loop() {
		p := parser.New(input)
		_ = p.Parse()
	}
}

func BenchmarkFullPipeline(b *testing.B) {
	input := largeCode

	b.ResetTimer()
	for b.Loop() {
		p := parser.New(input)
		tree := p.Parse()

		global, _ := analyser.BuildScopes(tree, input)
		analyser.PromoteClassMembers(global)
		resolver, _ := analyser.Resolve(tree, global)

		_ = resolver
		_ = global
	}
}

func BenchmarkMediumWorkspaceIndex_Unbounded(b *testing.B) {
	root := createBenchmarkWorkspace(b, benchWorkspaceMedium, 2)
	b.ResetTimer()
	for b.Loop() {
		s := newBenchmarkWorkspaceServer(b, root, benchWorkspaceMedium)
		s.buildWorkspaceSnapshots()
	}
}

func BenchmarkStartupReadiness_OneOpenFile_ShallowImports(b *testing.B) {
	root, openFiles := createStartupReadinessWorkspaceShallow(b, benchWorkspaceMedium)
	runStartupReadinessBenchmark(b, root, openFiles, 256)
}

func BenchmarkStartupReadiness_OneOpenFile_DeepImports(b *testing.B) {
	root, openFiles := createStartupReadinessWorkspaceDeep(b, 32, benchWorkspaceMedium)
	runStartupReadinessBenchmark(b, root, openFiles, 256)
}

func BenchmarkStartupReadiness_MultipleOpenFiles_SharedDeps(b *testing.B) {
	root, openFiles := createStartupReadinessWorkspaceShared(b, benchWorkspaceMedium)
	runStartupReadinessBenchmark(b, root, openFiles, 256)
}

func BenchmarkStartupReadiness_LargeWorkspace_UnrelatedModules(b *testing.B) {
	root, openFiles := createStartupReadinessWorkspaceSparse(b, benchWorkspaceLarge)
	runStartupReadinessBenchmark(b, root, openFiles, 64)
}

func BenchmarkMediumWorkspaceIndex_LRU256(b *testing.B) {
	root := createBenchmarkWorkspace(b, benchWorkspaceMedium, 2)
	var resident int
	b.ResetTimer()
	for b.Loop() {
		s := newBenchmarkWorkspaceServer(b, root, 256)
		s.buildWorkspaceSnapshots()
		resident = benchmarkResidentSnapshotCount(s)
	}
	b.ReportMetric(float64(resident), "resident_modules")
}

func BenchmarkStressWorkspaceIndex_LRU32(b *testing.B) {
	root := createBenchmarkWorkspace(b, benchWorkspaceLarge, 2)
	var resident int
	b.ResetTimer()
	for b.Loop() {
		s := newBenchmarkWorkspaceServer(b, root, 32)
		s.buildWorkspaceSnapshots()
		resident = benchmarkResidentSnapshotCount(s)
	}
	b.ReportMetric(float64(resident), "resident_modules")
}

func BenchmarkFastModuleLookup_Resident(b *testing.B) {
	root := createBenchmarkWorkspace(b, benchWorkspaceMedium, 2)
	s := newBenchmarkWorkspaceServer(b, root, 256)
	benchmarkWarmAllModules(b, s)
	b.ResetTimer()
	for b.Loop() {
		snapshot, ok := s.analyzeModuleByName("m000")
		if !ok || snapshot == nil {
			b.Fatal("expected resident module lookup to succeed")
		}
	}
}

func BenchmarkFastModuleLookup_EvictedReload(b *testing.B) {
	root := createBenchmarkWorkspace(b, benchWorkspaceMedium, 2)
	s := newBenchmarkWorkspaceServer(b, root, 32)
	mods := benchmarkWarmAllModules(b, s)
	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		benchmarkEvictModule(b, s, "m000", mods)
		b.StartTimer()
		snapshot, ok := s.analyzeModuleByName("m000")
		if !ok || snapshot == nil {
			b.Fatal("expected evicted module reload to succeed")
		}
	}
}

func BenchmarkFastWorkspaceSymbol_AllResident(b *testing.B) {
	root := createBenchmarkWorkspace(b, benchWorkspaceSmall, 2)
	s := newBenchmarkWorkspaceServer(b, root, benchWorkspaceSmall)
	benchmarkWarmAllModules(b, s)
	params := &lsp.WorkspaceSymbolParams{Query: "func_"}
	b.ResetTimer()
	for b.Loop() {
		results, err := s.WorkspaceSymbol(params)
		if err != nil || len(results) == 0 {
			b.Fatalf("workspace symbols failed: %v", err)
		}
	}
}

func BenchmarkMediumWorkspaceSymbol_LRU256(b *testing.B) {
	root := createBenchmarkWorkspace(b, benchWorkspaceMedium, 2)
	s := newBenchmarkWorkspaceServer(b, root, 256)
	benchmarkWarmAllModules(b, s)
	params := &lsp.WorkspaceSymbolParams{Query: "func_"}
	var resident int
	b.ResetTimer()
	for b.Loop() {
		results, err := s.WorkspaceSymbol(params)
		if err != nil || len(results) == 0 {
			b.Fatalf("workspace symbols failed: %v", err)
		}
		resident = benchmarkResidentSnapshotCount(s)
	}
	b.ReportMetric(float64(resident), "resident_modules")
}

func BenchmarkStressWorkspaceSymbol_LRU32(b *testing.B) {
	root := createBenchmarkWorkspace(b, benchWorkspaceLarge, 2)
	s := newBenchmarkWorkspaceServer(b, root, 32)
	benchmarkWarmAllModules(b, s)
	params := &lsp.WorkspaceSymbolParams{Query: "func_"}
	var resident int
	b.ResetTimer()
	for b.Loop() {
		results, err := s.WorkspaceSymbol(params)
		if err != nil || len(results) == 0 {
			b.Fatalf("workspace symbols failed: %v", err)
		}
		resident = benchmarkResidentSnapshotCount(s)
	}
	b.ReportMetric(float64(resident), "resident_modules")
}

func BenchmarkFastCompletionFromImport_Resident(b *testing.B) {
	root := b.TempDir()
	writeBenchmarkFile(b, filepath.Join(root, "lib.py"), benchmarkModuleCode("lib", nil))
	mainCode := "from lib import fu"
	mainPath := filepath.Join(root, "main.py")
	writeBenchmarkFile(b, mainPath, mainCode)

	s := newBenchmarkWorkspaceServer(b, root, 16)
	if _, ok := s.analyzeModuleByName("lib"); !ok {
		b.Fatal("expected lib module analysis to succeed")
	}
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	params := &lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: mainURI}, Position: lsp.Position{Line: 0, Character: len(mainCode)}}
	b.ResetTimer()
	for b.Loop() {
		items, err := s.Completion(params)
		if err != nil || len(items) == 0 {
			b.Fatalf("completion failed: %v", err)
		}
	}
	s.Close(mainURI)
}

func BenchmarkFastCompletionFromImport_Evicted(b *testing.B) {
	root := b.TempDir()
	writeBenchmarkFile(b, filepath.Join(root, "lib.py"), benchmarkModuleCode("lib", nil))
	for i := range benchWorkspaceSmall {
		name := fmt.Sprintf("m%03d", i)
		writeBenchmarkFile(b, filepath.Join(root, name+".py"), benchmarkModuleCode(name, nil))
	}
	mainCode := "from lib import fu"
	mainPath := filepath.Join(root, "main.py")
	writeBenchmarkFile(b, mainPath, mainCode)

	s := newBenchmarkWorkspaceServer(b, root, 8)
	mods := benchmarkWarmAllModules(b, s)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	params := &lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: mainURI}, Position: lsp.Position{Line: 0, Character: len(mainCode)}}
	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		benchmarkEvictModule(b, s, "lib", mods)
		b.StartTimer()
		items, err := s.Completion(params)
		if err != nil || len(items) == 0 {
			b.Fatalf("completion failed: %v", err)
		}
	}
	s.Close(mainURI)
}

func BenchmarkFastModuleMemberCompletion_Resident(b *testing.B) {
	root := b.TempDir()
	writeBenchmarkFile(b, filepath.Join(root, "lib.py"), benchmarkModuleCode("lib", nil))
	mainCode := "import lib\nlib.fu"
	mainPath := filepath.Join(root, "main.py")
	writeBenchmarkFile(b, mainPath, mainCode)

	s := newBenchmarkWorkspaceServer(b, root, 16)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))
	params := &lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: mainURI}, Position: lsp.Position{Line: 1, Character: len("lib.fu")}}
	b.ResetTimer()
	for b.Loop() {
		items, err := s.Completion(params)
		if err != nil || len(items) == 0 {
			b.Fatalf("completion failed: %v", err)
		}
	}
	s.Close(mainURI)
}

func BenchmarkFastModuleMemberCompletion_Evicted(b *testing.B) {
	root := b.TempDir()
	writeBenchmarkFile(b, filepath.Join(root, "lib.py"), benchmarkModuleCode("lib", nil))
	for i := range benchWorkspaceSmall {
		name := fmt.Sprintf("m%03d", i)
		writeBenchmarkFile(b, filepath.Join(root, name+".py"), benchmarkModuleCode(name, nil))
	}
	mainCode := "import lib\nlib.fu"
	mainPath := filepath.Join(root, "main.py")
	writeBenchmarkFile(b, mainPath, mainCode)

	s := newBenchmarkWorkspaceServer(b, root, 8)
	mods := benchmarkWarmAllModules(b, s)
	mainURI := pathToURI(mainPath)
	s.Open(lsp.TextDocumentItem{URI: mainURI, Text: mainCode, Version: 1})
	s.analyze(s.Get(mainURI))
	params := &lsp.CompletionParams{TextDocument: lsp.TextDocumentIdentifier{URI: mainURI}, Position: lsp.Position{Line: 1, Character: len("lib.fu")}}
	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		benchmarkEvictModule(b, s, "lib", mods)
		b.StartTimer()
		items, err := s.Completion(params)
		if err != nil || len(items) == 0 {
			b.Fatalf("completion failed: %v", err)
		}
	}
	s.Close(mainURI)
}

func BenchmarkMediumModuleRebuild_NoEviction(b *testing.B) {
	root := createBenchmarkWorkspace(b, benchWorkspaceMedium, 1)
	s := newBenchmarkWorkspaceServer(b, root, benchWorkspaceMedium)
	s.indexMu.RLock()
	mods := make([]ModuleFile, 0, len(s.modulesByName))
	for _, mod := range s.modulesByName {
		mods = append(mods, mod)
	}
	s.indexMu.RUnlock()
	b.ResetTimer()
	for b.Loop() {
		for _, mod := range mods {
			_, _ = s.rebuildModuleByURI(mod.URI)
		}
	}
}

func BenchmarkStressModuleRebuild_WithEviction(b *testing.B) {
	root := createBenchmarkWorkspace(b, benchWorkspaceLarge, 1)
	s := newBenchmarkWorkspaceServer(b, root, 16)
	s.indexMu.RLock()
	mods := make([]ModuleFile, 0, len(s.modulesByName))
	for _, mod := range s.modulesByName {
		mods = append(mods, mod)
	}
	s.indexMu.RUnlock()
	b.ResetTimer()
	for b.Loop() {
		for _, mod := range mods {
			_, _ = s.rebuildModuleByURI(mod.URI)
		}
	}
}

func BenchmarkMediumLRUEviction_RefIndexTrim(b *testing.B) {
	root := createBenchmarkWorkspace(b, benchWorkspaceMedium, 2)
	var resident int
	var refEntries int
	b.ResetTimer()
	for b.Loop() {
		s := newBenchmarkWorkspaceServer(b, root, 16)
		benchmarkWarmAllModules(b, s)
		resident = benchmarkResidentSnapshotCount(s)
		refEntries = benchmarkRefIndexURIEntryCount(s)
	}
	b.ReportMetric(float64(resident), "resident_modules")
	b.ReportMetric(float64(refEntries), "refindex_uris")
}
