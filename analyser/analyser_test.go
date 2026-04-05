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

func TestResolveParameterAnnotationAssignsType(t *testing.T) {
	src := "class Foo:\n    def method(self):\n        pass\n\ndef f(x: Foo):\n    x\n"
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	fnSym := global.Symbols["f"]
	if fnSym == nil || fnSym.Inner == nil {
		t.Fatal("missing function symbol f")
	}
	xSym := fnSym.Inner.Symbols["x"]
	if xSym == nil || xSym.Inferred == nil || xSym.Inferred.Kind != TypeInstance || xSym.Inferred.Symbol == nil || xSym.Inferred.Symbol.Name != "Foo" {
		t.Fatalf("expected annotated Foo type on x, got %+v", xSym)
	}
}

func TestResolveReturnAnnotationSetsCallExprType(t *testing.T) {
	src := "class Foo:\n    def method(self):\n        pass\n\ndef make() -> Foo:\n    return Foo()\n\nx = make()\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree)
	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	makeSym := global.Symbols["make"]
	if makeSym == nil || makeSym.Returns == nil || makeSym.Returns.Kind != TypeInstance || makeSym.Returns.Symbol == nil || makeSym.Returns.Symbol.Name != "Foo" {
		t.Fatalf("expected annotated Foo return type on make, got %+v", makeSym)
	}
	xSym := defs[mustNameNode(t, tree, "x")]
	if xSym == nil || xSym.Inferred == nil || xSym.Inferred.Kind != TypeInstance || xSym.Inferred.Symbol == nil || xSym.Inferred.Symbol.Name != "Foo" {
		t.Fatalf("expected propagated Foo type on x, got %+v", xSym)
	}
	if resolver.ExprTypes[findCallNode(t, tree)].Kind != TypeInstance || resolver.ExprTypes[findCallNode(t, tree)].Symbol == nil || resolver.ExprTypes[findCallNode(t, tree)].Symbol.Name != "Foo" {
		t.Fatalf("expected call expr type Foo, got %+v", resolver.ExprTypes[findCallNode(t, tree)])
	}
}

func TestResolveListAnnotationAssignsElementType(t *testing.T) {
	src := "def f(items: list[int]):\n    items\n"
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	fnSym := global.Symbols["f"]
	itemsSym := fnSym.Inner.Symbols["items"]
	if itemsSym == nil || itemsSym.Inferred == nil || itemsSym.Inferred.Kind != TypeList {
		t.Fatalf("expected list type on items, got %+v", itemsSym)
	}
	if itemsSym.Inferred.Elem == nil || itemsSym.Inferred.Elem.Kind != TypeBuiltin || itemsSym.Inferred.Elem.Symbol == nil || itemsSym.Inferred.Elem.Symbol.Name != "int" {
		t.Fatalf("expected list[int], got %+v", itemsSym.Inferred)
	}
}

func TestResolveTupleAnnotationAssignsItems(t *testing.T) {
	src := "def f(pair: tuple[str, int]):\n    pair\n"
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	fnSym := global.Symbols["f"]
	pairSym := fnSym.Inner.Symbols["pair"]
	if pairSym == nil || pairSym.Inferred == nil || pairSym.Inferred.Kind != TypeTuple {
		t.Fatalf("expected tuple type on pair, got %+v", pairSym)
	}
	if len(pairSym.Inferred.Items) != 2 {
		t.Fatalf("expected 2 tuple items, got %+v", pairSym.Inferred)
	}
	if pairSym.Inferred.Items[0] == nil || pairSym.Inferred.Items[0].Kind != TypeBuiltin || pairSym.Inferred.Items[0].Symbol == nil || pairSym.Inferred.Items[0].Symbol.Name != "str" {
		t.Fatalf("expected first tuple item str, got %+v", pairSym.Inferred.Items[0])
	}
	if pairSym.Inferred.Items[1] == nil || pairSym.Inferred.Items[1].Kind != TypeBuiltin || pairSym.Inferred.Items[1].Symbol == nil || pairSym.Inferred.Items[1].Symbol.Name != "int" {
		t.Fatalf("expected second tuple item int, got %+v", pairSym.Inferred.Items[1])
	}
}

func TestResolveNestedListAnnotationAssignsNestedType(t *testing.T) {
	src := "def f(matrix: list[list[int]]):\n    matrix\n"
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	fnSym := global.Symbols["f"]
	matrixSym := fnSym.Inner.Symbols["matrix"]
	if matrixSym == nil || matrixSym.Inferred == nil || matrixSym.Inferred.Kind != TypeList {
		t.Fatalf("expected outer list type on matrix, got %+v", matrixSym)
	}
	inner := matrixSym.Inferred.Elem
	if inner == nil || inner.Kind != TypeList {
		t.Fatalf("expected nested list type, got %+v", inner)
	}
	if inner.Elem == nil || inner.Elem.Kind != TypeBuiltin || inner.Elem.Symbol == nil || inner.Elem.Symbol.Name != "int" {
		t.Fatalf("expected nested list[int], got %+v", inner)
	}
}

func TestResolveDictAnnotationAssignsKeyValueTypes(t *testing.T) {
	src := "def f(mapping: dict[str, int]):\n    mapping\n"
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	fnSym := global.Symbols["f"]
	mappingSym := fnSym.Inner.Symbols["mapping"]
	if mappingSym == nil || mappingSym.Inferred == nil || mappingSym.Inferred.Kind != TypeDict {
		t.Fatalf("expected dict type on mapping, got %+v", mappingSym)
	}
	if mappingSym.Inferred.Key == nil || mappingSym.Inferred.Key.Kind != TypeBuiltin || mappingSym.Inferred.Key.Symbol == nil || mappingSym.Inferred.Key.Symbol.Name != "str" {
		t.Fatalf("expected dict key type str, got %+v", mappingSym.Inferred.Key)
	}
	if mappingSym.Inferred.Elem == nil || mappingSym.Inferred.Elem.Kind != TypeBuiltin || mappingSym.Inferred.Elem.Symbol == nil || mappingSym.Inferred.Elem.Symbol.Name != "int" {
		t.Fatalf("expected dict value type int, got %+v", mappingSym.Inferred.Elem)
	}
}

func TestResolveSetAnnotationAssignsElementType(t *testing.T) {
	src := "def f(items: set[int]):\n    items\n"
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	fnSym := global.Symbols["f"]
	itemsSym := fnSym.Inner.Symbols["items"]
	if itemsSym == nil || itemsSym.Inferred == nil || itemsSym.Inferred.Kind != TypeSet {
		t.Fatalf("expected set type on items, got %+v", itemsSym)
	}
	if itemsSym.Inferred.Elem == nil || itemsSym.Inferred.Elem.Kind != TypeBuiltin || itemsSym.Inferred.Elem.Symbol == nil || itemsSym.Inferred.Elem.Symbol.Name != "int" {
		t.Fatalf("expected set[int], got %+v", itemsSym.Inferred)
	}
}

func TestResolveNestedDictAnnotationAssignsNestedTypes(t *testing.T) {
	src := "def f(nested: dict[str, list[int]]):\n    nested\n"
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	fnSym := global.Symbols["f"]
	nestedSym := fnSym.Inner.Symbols["nested"]
	if nestedSym == nil || nestedSym.Inferred == nil || nestedSym.Inferred.Kind != TypeDict {
		t.Fatalf("expected dict type on nested, got %+v", nestedSym)
	}
	if nestedSym.Inferred.Key == nil || nestedSym.Inferred.Key.Kind != TypeBuiltin || nestedSym.Inferred.Key.Symbol == nil || nestedSym.Inferred.Key.Symbol.Name != "str" {
		t.Fatalf("expected dict key type str, got %+v", nestedSym.Inferred.Key)
	}
	if nestedSym.Inferred.Elem == nil || nestedSym.Inferred.Elem.Kind != TypeList {
		t.Fatalf("expected dict value list type, got %+v", nestedSym.Inferred.Elem)
	}
	if nestedSym.Inferred.Elem.Elem == nil || nestedSym.Inferred.Elem.Elem.Kind != TypeBuiltin || nestedSym.Inferred.Elem.Elem.Symbol == nil || nestedSym.Inferred.Elem.Elem.Symbol.Name != "int" {
		t.Fatalf("expected dict value list[int], got %+v", nestedSym.Inferred.Elem)
	}
}

func TestResolveAnnotatedVariableAssignsBuiltinType(t *testing.T) {
	src := "x: int = 1\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	xSym := defs[mustNameNode(t, tree, "x")]
	if xSym == nil || xSym.Inferred == nil || xSym.Inferred.Kind != TypeBuiltin || xSym.Inferred.Symbol == nil || xSym.Inferred.Symbol.Name != "int" {
		t.Fatalf("expected annotated int type on x, got %+v", xSym)
	}
}

func TestResolveAnnotatedVariableWithoutValueAssignsType(t *testing.T) {
	src := "x: set[int]\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	xSym := defs[mustNameNode(t, tree, "x")]
	if xSym == nil || xSym.Inferred == nil || xSym.Inferred.Kind != TypeSet || xSym.Inferred.Elem == nil || xSym.Inferred.Elem.Symbol == nil || xSym.Inferred.Elem.Symbol.Name != "int" {
		t.Fatalf("expected annotated set[int] type on x, got %+v", xSym)
	}
}

func TestResolveAnnotatedVariablePrefersAnnotationOverEmptyLiteral(t *testing.T) {
	src := "items: list[int] = []\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	itemsSym := defs[mustNameNode(t, tree, "items")]
	if itemsSym == nil || itemsSym.Inferred == nil || itemsSym.Inferred.Kind != TypeList || itemsSym.Inferred.Elem == nil || itemsSym.Inferred.Elem.Symbol == nil || itemsSym.Inferred.Elem.Symbol.Name != "int" {
		t.Fatalf("expected annotated list[int] type on items, got %+v", itemsSym)
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
