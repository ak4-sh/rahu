package locate_test

import (
	"testing"

	"rahu/parser"
	a "rahu/parser/ast"
	l "rahu/server/locate"
	"rahu/source"
)

func TestNameAtPos(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		line         int
		col          int
		expectedName string
	}{
		{"simple variable reference", "x = 1\ny = x", 2, 5, "x"},
		{"name in function call", "x = 1\nprint(x)", 2, 7, "x"},
		{"name in comparison", "x = 1\nif x > 0:\n    pass", 2, 4, "x"},
		{"name in function default argument", "default_val = 10\ndef foo(x=default_val):\n    pass", 2, 14, "default_val"},
		{"name in partial class base", "class Foo(Bar)", 1, 11, "Bar"},
		{"position outside any name", "x = 1", 1, 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := parser.New(tt.code).Parse()
			li := source.NewLineIndex(tt.code)
			offset := li.PositionToOffset(tt.line-1, tt.col-1)

			name := l.NameAtPos(tree, offset)
			if tt.expectedName == "" {
				if name != a.NoNode {
					got, _ := tree.NameText(name)
					t.Fatalf("expected no name, got %q", got)
				}
				return
			}

			if name == a.NoNode {
				t.Fatalf("expected %q, got no node", tt.expectedName)
			}

			got, ok := tree.NameText(name)
			if !ok {
				t.Fatalf("expected name node for %q", tt.expectedName)
			}
			if got != tt.expectedName {
				t.Fatalf("expected %q, got %q", tt.expectedName, got)
			}
		})
	}
}

func TestNameAtPosNilModule(t *testing.T) {
	if l.NameAtPos(nil, 0) != a.NoNode {
		t.Fatal("expected no node for nil tree")
	}
}

func TestLocateAtPos(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		line         int
		col          int
		expectedKind l.ResultKind
		expectedText string
	}{
		{"simple name", "x = 1\ny = x", 2, 5, l.NameResult, "x"},
		{"attribute wins on attribute text", "obj.value", 1, 5, l.AttributeResult, "value"},
		{"base name on attribute expression", "obj.value", 1, 2, l.NameResult, "obj"},
		{"nested inner attribute", "obj.inner.value", 1, 6, l.AttributeResult, "inner"},
		{"nested outer attribute", "obj.inner.value", 1, 12, l.AttributeResult, "value"},
		{"outside any symbol", "x = 1", 1, 10, l.NoResult, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := parser.New(tt.code).Parse()
			li := source.NewLineIndex(tt.code)
			offset := li.PositionToOffset(tt.line-1, tt.col-1)

			res := l.LocateAtPos(tree, offset)
			if res.Kind != tt.expectedKind {
				t.Fatalf("expected kind %v, got %v", tt.expectedKind, res.Kind)
			}

			if tt.expectedKind == l.NoResult {
				if res.Node != a.NoNode {
					t.Fatalf("expected no node, got %d", res.Node)
				}
				return
			}

			got, ok := locatedText(tree, res)
			if !ok {
				t.Fatalf("expected text for locate result %#v", res)
			}
			if got != tt.expectedText {
				t.Fatalf("expected text %q, got %q", tt.expectedText, got)
			}
		})
	}
}

func TestLocateAtPosNilModule(t *testing.T) {
	res := l.LocateAtPos(nil, 0)
	if res.Kind != l.NoResult || res.Node != a.NoNode {
		t.Fatalf("expected no result, got %#v", res)
	}
}

func TestAttributeAtPos(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		line         int
		col          int
		expectedAttr string
	}{
		{"simple attribute", "obj.x", 1, 5, "x"},
		{"nested inner attribute", "obj.inner.x", 1, 6, "inner"},
		{"nested outer attribute", "obj.inner.x", 1, 12, "x"},
		{"attribute in call args", "print(obj.value)", 1, 11, "value"},
		{"attribute assignment target", "self.value = other.value", 1, 7, "value"},
		{"attribute in return", "def f():\n    return self.attr", 2, 17, "attr"},
		{"attribute in if body", "if ok:\n    self.ready", 2, 10, "ready"},
		{"position outside attribute", "obj.x", 1, 2, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := parser.New(tt.code).Parse()
			li := source.NewLineIndex(tt.code)
			offset := li.PositionToOffset(tt.line-1, tt.col-1)

			attr := l.AttributeAtPos(tree, offset)
			if tt.expectedAttr == "" {
				if attr != a.NoNode {
					got, ok := attributeName(tree, attr)
					if !ok {
						got = "<non-attribute>"
					}
					t.Fatalf("expected no attribute, got %q", got)
				}
				return
			}

			if attr == a.NoNode {
				t.Fatalf("expected attribute %q, got no node", tt.expectedAttr)
			}

			got, ok := attributeName(tree, attr)
			if !ok {
				t.Fatalf("expected attribute node for %q", tt.expectedAttr)
			}
			if got != tt.expectedAttr {
				t.Fatalf("expected attribute %q, got %q", tt.expectedAttr, got)
			}
		})
	}
}

func TestAttributeAtPosNilModule(t *testing.T) {
	if l.AttributeAtPos(nil, 0) != a.NoNode {
		t.Fatal("expected no node for nil tree")
	}
}

func attributeName(tree *a.AST, expr a.NodeID) (string, bool) {
	if tree == nil || expr == a.NoNode || tree.Node(expr).Kind != a.NodeAttribute {
		return "", false
	}

	base := tree.Nodes[expr].FirstChild
	if base == a.NoNode {
		return "", false
	}

	attr := tree.Nodes[base].NextSibling
	if attr == a.NoNode {
		return "", false
	}

	return tree.NameText(attr)
}

func locatedText(tree *a.AST, res l.Result) (string, bool) {
	switch res.Kind {
	case l.NameResult:
		return tree.NameText(res.Node)
	case l.AttributeResult:
		return attributeName(tree, res.Node)
	default:
		return "", false
	}
}
