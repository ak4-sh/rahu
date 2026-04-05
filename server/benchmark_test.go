package server

import (
	"strings"
	"testing"

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
