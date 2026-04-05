package analyser

import (
	"testing"

	"rahu/parser"
	"rahu/parser/ast"
)

func TestFunctionScope(t *testing.T) {
	input := `
        def f(a, b):
            x = 1
    `

	p := parser.New(input)
	tree := p.Parse()

	global, _ := BuildScopes(tree)

	if _, ok := global.Symbols["f"]; !ok {
		t.Fatal("missing function symbol f")
	}

	fn := global.Children[0]

	for _, name := range []string{"a", "b", "x"} {
		if _, ok := fn.Symbols[name]; !ok {
			t.Fatalf("missing symbol %s", name)
		}
	}
}

func TestSimpleResolution(t *testing.T) {
	src := `
	x = 1
	y = x
	`

	p := parser.New(src)
	tree := p.Parse()
	global, _ := BuildScopes(tree)

	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	var xUses []ast.NodeID
	collectNames(tree, tree.Root, &xUses)

	if resolver.Resolved[xUses[0]].Name != "x" {
		t.Fatal("x did not resolve to symbol x")
	}
}

func collectNames(tree *ast.AST, id ast.NodeID, out *[]ast.NodeID) {
	if id == ast.NoNode {
		return
	}

	if tree.Node(id).Kind == ast.NodeName {
		*out = append(*out, id)
	}

	for _, child := range tree.Children(id) {
		collectNames(tree, child, out)
	}
}

func TestBuildScopes_AllowsPartialFunctionHeader(t *testing.T) {
	tree := parser.New("def foo(x=bar)").Parse()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("BuildScopes panicked on partial function: %v", r)
		}
	}()

	global, _ := BuildScopes(tree)
	if _, ok := global.Symbols["foo"]; !ok {
		t.Fatal("missing function symbol foo")
	}

	if _, errs := Resolve(tree, global); len(errs) == 0 {
		t.Fatal("expected unresolved default argument to produce an error")
	}
}

func TestBuildScopes_AllowsPartialClassHeader(t *testing.T) {
	tree := parser.New("class Foo(Bar)").Parse()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("BuildScopes panicked on partial class: %v", r)
		}
	}()

	global, _ := BuildScopes(tree)
	if _, ok := global.Symbols["Foo"]; !ok {
		t.Fatal("missing class symbol Foo")
	}

	if _, errs := Resolve(tree, global); len(errs) == 0 {
		t.Fatal("expected unresolved base class to produce an error")
	}
}
