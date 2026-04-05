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

func TestResolve_SubscriptAssignmentTarget(t *testing.T) {
	src := "a = [1]\na[0] = 2\na[0] += 3\n"
	tree := parser.New(src).Parse()

	global, _ := BuildScopes(tree)
	if _, ok := global.Symbols["a"]; !ok {
		t.Fatal("missing symbol a")
	}

	if _, errs := Resolve(tree, global); len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}
}

func TestResolve_KeywordArgValueOnly(t *testing.T) {
	src := "items = [1]\nfoo = print\nfoo(x=items[0])\n"
	tree := parser.New(src).Parse()

	global, _ := BuildScopes(tree)
	if _, errs := Resolve(tree, global); len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}
}

func TestResolveConstructorCallAssignsInferredInstanceType(t *testing.T) {
	src := "class Foo:\n    def method(self):\n        pass\n\nx = Foo()\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree)
	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	xSym := defs[mustNameNode(t, tree, "x")]
	if xSym == nil || xSym.Inferred == nil || xSym.Inferred.Kind != TypeInstance || xSym.Inferred.Symbol == nil || xSym.Inferred.Symbol.Name != "Foo" {
		t.Fatalf("expected inferred Foo instance type on x, got %+v", xSym)
	}
	if resolver.ExprTypes[findCallNode(t, tree)].Kind != TypeInstance {
		t.Fatalf("expected constructor call to have instance expr type, got %+v", resolver.ExprTypes[findCallNode(t, tree)])
	}
}

func TestResolvePropagatesInstanceTypeAcrossAssignment(t *testing.T) {
	src := "class Foo:\n    def method(self):\n        pass\n\nx = Foo()\ny = x\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	ySym := defs[mustNameNode(t, tree, "y")]
	if ySym == nil || ySym.Inferred == nil || ySym.Inferred.Kind != TypeInstance || ySym.Inferred.Symbol == nil || ySym.Inferred.Symbol.Name != "Foo" {
		t.Fatalf("expected inferred Foo instance type on y, got %+v", ySym)
	}
}

func TestResolveRepeatedAssignmentBuildsUnionType(t *testing.T) {
	src := "class Foo:\n    pass\n\nclass Bar:\n    pass\n\nx = Foo()\nx = Bar()\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	xSym := defs[mustNameNode(t, tree, "x")]
	if xSym == nil || xSym.Inferred == nil || xSym.Inferred.Kind != TypeUnion {
		t.Fatalf("expected union type on x, got %+v", xSym)
	}
	if len(xSym.Inferred.Union) != 2 {
		t.Fatalf("expected two union arms, got %+v", xSym.Inferred)
	}
}

func TestResolveListLiteralBuildsElementType(t *testing.T) {
	src := "class Foo:\n    pass\n\nxs = [Foo()]\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	xsSym := defs[mustNameNode(t, tree, "xs")]
	if xsSym == nil || xsSym.Inferred == nil || xsSym.Inferred.Kind != TypeList || xsSym.Inferred.Elem == nil || xsSym.Inferred.Elem.Kind != TypeInstance || xsSym.Inferred.Elem.Symbol == nil || xsSym.Inferred.Elem.Symbol.Name != "Foo" {
		t.Fatalf("expected list[Foo], got %+v", xsSym)
	}
}

func TestResolveMixedListLiteralBuildsUnionElementType(t *testing.T) {
	src := "class Foo:\n    pass\n\nclass Bar:\n    pass\n\nxs = [Foo(), Bar()]\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	xsSym := defs[mustNameNode(t, tree, "xs")]
	if xsSym == nil || xsSym.Inferred == nil || xsSym.Inferred.Kind != TypeList || xsSym.Inferred.Elem == nil || xsSym.Inferred.Elem.Kind != TypeUnion || len(xsSym.Inferred.Elem.Union) != 2 {
		t.Fatalf("expected list[Foo | Bar], got %+v", xsSym)
	}
}

func TestResolveTupleSubscriptBuildsUnionElementType(t *testing.T) {
	src := "class Foo:\n    pass\n\nclass Bar:\n    pass\n\nt = (Foo(), Bar())\nx = t[0]\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	xSym := defs[mustNameNode(t, tree, "x")]
	if xSym == nil || xSym.Inferred == nil || xSym.Inferred.Kind != TypeUnion || len(xSym.Inferred.Union) != 2 {
		t.Fatalf("expected Foo | Bar from tuple subscript, got %+v", xSym)
	}
}

func TestResolveSpecialBuiltinNameGuard(t *testing.T) {
	src := "if __name__ == \"__main__\":\n    pass\n"
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}
}

func TestResolveSpecialBuiltinNamePropagatesStrType(t *testing.T) {
	src := "x = __name__\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	xSym := defs[mustNameNode(t, tree, "x")]
	if xSym == nil || xSym.Inferred == nil || xSym.Inferred.Kind != TypeBuiltin || xSym.Inferred.Symbol == nil || xSym.Inferred.Symbol.Name != "str" {
		t.Fatalf("expected inferred str type on x, got %+v", xSym)
	}
}

func TestResolveDictLiteralTraversal(t *testing.T) {
	src := "base = 1\ndef sq(x):\n    return x\ndef sin(x):\n    return x\ndata = {\"name\": base, \"root\": sq(16), \"sine\": sin(0)}\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	dataSym := defs[mustNameNode(t, tree, "data")]
	if dataSym == nil || dataSym.Inferred == nil || dataSym.Inferred.Kind != TypeBuiltin || dataSym.Inferred.Symbol == nil || dataSym.Inferred.Symbol.Name != "dict" {
		t.Fatalf("expected inferred dict type on data, got %+v", dataSym)
	}
}

func TestResolveListAppendNoUndefinedAttributeAndPropagatesElementType(t *testing.T) {
	src := "class Foo:\n    def method(self):\n        pass\n\nxs = []\nxs.append(Foo())\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	xsSym := defs[mustNameNode(t, tree, "xs")]
	if xsSym == nil || xsSym.Inferred == nil || xsSym.Inferred.Kind != TypeList || xsSym.Inferred.Elem == nil || xsSym.Inferred.Elem.Kind != TypeInstance || xsSym.Inferred.Elem.Symbol == nil || xsSym.Inferred.Elem.Symbol.Name != "Foo" {
		t.Fatalf("expected list[Foo] after append, got %+v", xsSym)
	}
}

func TestResolveListAppendBuildsUnionElementType(t *testing.T) {
	src := "class Foo:\n    pass\n\nclass Bar:\n    pass\n\nxs = []\nxs.append(Foo())\nxs.append(Bar())\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	xsSym := defs[mustNameNode(t, tree, "xs")]
	if xsSym == nil || xsSym.Inferred == nil || xsSym.Inferred.Kind != TypeList || xsSym.Inferred.Elem == nil || xsSym.Inferred.Elem.Kind != TypeUnion || len(xsSym.Inferred.Elem.Union) != 2 {
		t.Fatalf("expected list[Foo | Bar] after appends, got %+v", xsSym)
	}
}

func mustNameNode(t *testing.T, tree *ast.AST, name string) ast.NodeID {
	t.Helper()
	var found ast.NodeID
	var walk func(ast.NodeID)
	walk = func(id ast.NodeID) {
		if id == ast.NoNode || found != ast.NoNode {
			return
		}
		if tree.Node(id).Kind == ast.NodeName {
			if text, ok := tree.NameText(id); ok && text == name {
				found = id
				return
			}
		}
		for _, child := range tree.Children(id) {
			walk(child)
		}
	}
	walk(tree.Root)
	if found == ast.NoNode {
		t.Fatalf("missing name node %q", name)
	}
	return found
}

func findCallNode(t *testing.T, tree *ast.AST) ast.NodeID {
	t.Helper()
	var found ast.NodeID
	var walk func(ast.NodeID)
	walk = func(id ast.NodeID) {
		if id == ast.NoNode || found != ast.NoNode {
			return
		}
		if tree.Node(id).Kind == ast.NodeCall {
			found = id
			return
		}
		for _, child := range tree.Children(id) {
			walk(child)
		}
	}
	walk(tree.Root)
	if found == ast.NoNode {
		t.Fatal("missing call node")
	}
	return found
}
