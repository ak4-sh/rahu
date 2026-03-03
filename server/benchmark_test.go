package server

import (
	"strings"
	"testing"

	"rahu/analyser"
	"rahu/lsp"
	"rahu/parser"
	l "rahu/server/locate"
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

	module := doc.AST

	b.ResetTimer()
	for b.Loop() {
		name := l.NameAtPos(module, 500)
		if name != nil {
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
		name := l.NameAtPos(doc.AST, offset)
		if name != nil {
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

	module := doc.AST

	positions := []int{100, 250, 500, 750, 1000, 1500, 2000, 2500, 3000}

	b.ResetTimer()
	for b.Loop() {
		for _, pos := range positions {
			name := l.NameAtPos(module, pos)
			if name != nil {
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
			name := l.NameAtPos(doc.AST, offset)
			if name != nil {
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
		module := p.Parse()

		global := analyser.BuildScopes(module)
		analyser.PromoteClassMembers(global)
		resolver, _ := analyser.Resolve(module, global)

		_ = resolver
		_ = global
	}
}
