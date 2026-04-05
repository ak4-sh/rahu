package locate_test

import (
	"strings"
	"testing"

	"rahu/parser"
	l "rahu/server/locate"
	"rahu/source"
)

// TestPositionIndexMatchesLinear verifies that the indexed lookup produces
// the same results as the linear tree traversal for various positions.
func TestPositionIndexMatchesLinear(t *testing.T) {
	tests := []struct {
		name string
		code string
		line int
		col  int
	}{
		{"simple variable reference", "x = 1\ny = x", 2, 5},
		{"name in function call", "x = 1\nprint(x)", 2, 7},
		{"name in comparison", "x = 1\nif x > 0:\n    pass", 2, 4},
		{"name in function default argument", "default_val = 10\ndef foo(x=default_val):\n    pass", 2, 14},
		{"name in class base", "class Foo(Bar):\n    pass", 1, 11},
		{"position outside any name", "x = 1", 1, 10},
		{"attribute wins on attribute text", "obj.value", 1, 5},
		{"base name on attribute expression", "obj.value", 1, 2},
		{"nested inner attribute", "obj.inner.value", 1, 6},
		{"nested outer attribute", "obj.inner.value", 1, 12},
		{"function name", "def foo():\n    pass", 1, 5},
		{"class name", "class MyClass:\n    pass", 1, 8},
		{"import alias", "import os as operating_system", 1, 15},
		{"from import name", "from os import path", 1, 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := parser.New(tt.code).Parse()
			li := source.NewLineIndex(tt.code)
			offset := li.PositionToOffset(tt.line-1, tt.col-1)

			// Build index
			idx := l.Build(tree)

			// Get results from both methods
			linearRes := l.LocateAtPos(tree, offset)
			indexedRes := l.LocateAtPosIndexed(tree, offset, idx)

			// Results should match
			if linearRes.Kind != indexedRes.Kind {
				t.Errorf("kind mismatch: linear=%v, indexed=%v", linearRes.Kind, indexedRes.Kind)
			}
			if linearRes.Node != indexedRes.Node {
				t.Errorf("node mismatch: linear=%v, indexed=%v", linearRes.Node, indexedRes.Node)
			}
		})
	}
}

// TestPositionIndexBuild verifies that index building works correctly.
func TestPositionIndexBuild(t *testing.T) {
	code := `
x = 1
y = x + 2
def foo(a, b):
    return a + b
class Bar:
    def method(self):
        self.value = 10
`
	tree := parser.New(code).Parse()
	idx := l.Build(tree)

	// Should have entries for all names: x, y, x, foo, a, b, a, b, Bar, method, self, self, value
	// and attribute entries for: self.value
	if idx.Size() == 0 {
		t.Fatal("expected non-empty index")
	}
	t.Logf("Index contains %d entries for %d AST nodes", idx.Size(), len(tree.Nodes))
}

// TestPositionIndexNilTree verifies handling of nil tree.
func TestPositionIndexNilTree(t *testing.T) {
	idx := l.Build(nil)
	if idx == nil {
		t.Fatal("Build(nil) should return empty index, not nil")
	}
	if idx.Size() != 0 {
		t.Fatalf("expected empty index, got %d entries", idx.Size())
	}

	res := idx.Lookup(0)
	if res.Kind != l.NoResult {
		t.Fatalf("expected NoResult for empty index, got %v", res.Kind)
	}
}

// TestPositionIndexEmptyTree verifies handling of empty tree.
func TestPositionIndexEmptyTree(t *testing.T) {
	tree := parser.New("").Parse()
	idx := l.Build(tree)

	res := idx.Lookup(0)
	if res.Kind != l.NoResult {
		t.Fatalf("expected NoResult for empty tree, got %v", res.Kind)
	}
}

// TestPositionIndexFallback verifies fallback to linear when index is nil.
func TestPositionIndexFallback(t *testing.T) {
	code := "x = 1\ny = x"
	tree := parser.New(code).Parse()
	li := source.NewLineIndex(code)
	offset := li.PositionToOffset(1, 4) // 'x' on second line

	// With nil index, should fall back to linear
	res := l.LocateAtPosIndexed(tree, offset, nil)
	if res.Kind != l.NameResult {
		t.Fatalf("expected NameResult, got %v", res.Kind)
	}

	text, ok := tree.NameText(res.Node)
	if !ok || text != "x" {
		t.Fatalf("expected 'x', got %q", text)
	}
}

// generateLargeCode creates Python code with many names for benchmarking.
func generateLargeCode(numFunctions int) string {
	var sb strings.Builder
	sb.WriteString("# Large Python module for benchmarking\n")
	sb.WriteString("import os\n")
	sb.WriteString("import sys\n\n")

	for i := 0; i < numFunctions; i++ {
		sb.WriteString("def func")
		sb.WriteString(itoa(i))
		sb.WriteString("(a, b, c):\n")
		sb.WriteString("    x = a + b\n")
		sb.WriteString("    y = x * c\n")
		sb.WriteString("    if y > 0:\n")
		sb.WriteString("        return y\n")
		sb.WriteString("    return x\n\n")
	}

	return sb.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// BenchmarkLocateLinear benchmarks the original linear tree traversal.
func BenchmarkLocateLinear(b *testing.B) {
	code := generateLargeCode(100) // 100 functions, ~500 names
	tree := parser.New(code).Parse()
	li := source.NewLineIndex(code)

	// Find position in the middle of the file
	lines := strings.Split(code, "\n")
	midLine := len(lines) / 2
	offset := li.PositionToOffset(midLine, 8)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.LocateAtPos(tree, offset)
	}
}

// BenchmarkLocateIndexed benchmarks the indexed binary search lookup.
func BenchmarkLocateIndexed(b *testing.B) {
	code := generateLargeCode(100) // 100 functions, ~500 names
	tree := parser.New(code).Parse()
	li := source.NewLineIndex(code)
	idx := l.Build(tree)

	// Find position in the middle of the file
	lines := strings.Split(code, "\n")
	midLine := len(lines) / 2
	offset := li.PositionToOffset(midLine, 8)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.LocateAtPosIndexed(tree, offset, idx)
	}
}

// BenchmarkBuildIndex benchmarks index construction time.
func BenchmarkBuildIndex(b *testing.B) {
	code := generateLargeCode(100)
	tree := parser.New(code).Parse()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Build(tree)
	}
}

// BenchmarkLocateLinearWorstCase benchmarks looking up a name at the end of file.
func BenchmarkLocateLinearWorstCase(b *testing.B) {
	code := generateLargeCode(100)
	tree := parser.New(code).Parse()
	li := source.NewLineIndex(code)

	// Find position near the end of the file (worst case for linear scan)
	lines := strings.Split(code, "\n")
	lastLine := len(lines) - 5
	offset := li.PositionToOffset(lastLine, 8)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.LocateAtPos(tree, offset)
	}
}

// BenchmarkLocateIndexedWorstCase benchmarks indexed lookup at end of file.
func BenchmarkLocateIndexedWorstCase(b *testing.B) {
	code := generateLargeCode(100)
	tree := parser.New(code).Parse()
	li := source.NewLineIndex(code)
	idx := l.Build(tree)

	// Find position near the end of the file
	lines := strings.Split(code, "\n")
	lastLine := len(lines) - 5
	offset := li.PositionToOffset(lastLine, 8)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.LocateAtPosIndexed(tree, offset, idx)
	}
}

// BenchmarkLocateLinearLarge benchmarks linear search on a larger file.
func BenchmarkLocateLinearLarge(b *testing.B) {
	code := generateLargeCode(500) // 500 functions
	tree := parser.New(code).Parse()
	li := source.NewLineIndex(code)

	lines := strings.Split(code, "\n")
	lastLine := len(lines) - 5
	offset := li.PositionToOffset(lastLine, 8)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.LocateAtPos(tree, offset)
	}
}

// BenchmarkLocateIndexedLarge benchmarks indexed search on a larger file.
func BenchmarkLocateIndexedLarge(b *testing.B) {
	code := generateLargeCode(500)
	tree := parser.New(code).Parse()
	li := source.NewLineIndex(code)
	idx := l.Build(tree)

	lines := strings.Split(code, "\n")
	lastLine := len(lines) - 5
	offset := li.PositionToOffset(lastLine, 8)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.LocateAtPosIndexed(tree, offset, idx)
	}
}
