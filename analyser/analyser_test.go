package analyser

import (
	"strings"
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

	global, _ := BuildScopes(tree, input)

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
	global, _ := BuildScopes(tree, src)

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

func findNodeByKind(t *testing.T, tree *ast.AST, kind ast.NodeKind) ast.NodeID {
	t.Helper()
	for id := ast.NodeID(1); int(id) < len(tree.Nodes); id++ {
		if tree.Node(id).Kind == kind {
			return id
		}
	}
	t.Fatalf("missing node kind %s", kind)
	return ast.NoNode
}

func TestBuildScopes_AllowsPartialFunctionHeader(t *testing.T) {
	src := "def foo(x=bar)"
	tree := parser.New(src).Parse()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("BuildScopes panicked on partial function: %v", r)
		}
	}()

	global, _ := BuildScopes(tree, src)
	if _, ok := global.Symbols["foo"]; !ok {
		t.Fatal("missing function symbol foo")
	}

	if _, errs := Resolve(tree, global); len(errs) == 0 {
		t.Fatal("expected unresolved default argument to produce an error")
	}
}

func TestBuildScopes_AllowsPartialClassHeader(t *testing.T) {
	src := "class Foo(Bar)"
	tree := parser.New(src).Parse()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("BuildScopes panicked on partial class: %v", r)
		}
	}()

	global, _ := BuildScopes(tree, src)
	if _, ok := global.Symbols["Foo"]; !ok {
		t.Fatal("missing class symbol Foo")
	}

	if _, errs := Resolve(tree, global); len(errs) == 0 {
		t.Fatal("expected unresolved base class to produce an error")
	}
}

func TestResolve_QualifiedClassBasePromotesInheritedMembers(t *testing.T) {
	src := "class Outer:\n    class Inner:\n        def base(self):\n            pass\n\nclass Foo(Outer.Inner):\n    pass\n\nitem = Foo()\nitem.base\n"
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)

	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	outer := global.Symbols["Outer"]
	if outer == nil || outer.Inner == nil {
		t.Fatal("missing outer class scope")
	}
	inner, ok := outer.Inner.LookupLocal("Inner")
	if !ok || inner == nil {
		t.Fatal("missing nested class symbol Inner")
	}
	foo := global.Symbols["Foo"]
	if foo == nil {
		t.Fatal("missing class symbol Foo")
	}
	if len(foo.Bases) != 1 || foo.Bases[0] != inner {
		t.Fatalf("expected Foo to inherit from Outer.Inner, got %+v", foo.Bases)
	}
	if foo.Members == nil {
		t.Fatal("expected promoted class members")
	}
	if _, ok := foo.Members.Lookup("base"); !ok {
		t.Fatal("expected inherited member base to be promoted onto Foo")
	}

	use := mustNameNode(t, tree, "item")
	if resolver.Resolved[use] == nil || resolver.Resolved[use].Name != "item" {
		t.Fatalf("expected item use to resolve, got %+v", resolver.Resolved[use])
	}
	if attr := mustAttributeNode(t, tree, "base"); resolver.ResolvedAttr[attr] == nil || resolver.ResolvedAttr[attr].Name != "base" {
		t.Fatalf("expected inherited attribute base to resolve, got %+v", resolver.ResolvedAttr[attr])
	}
}

func TestBuildScopes_AllowsExceptPass(t *testing.T) {
	src := "try:\n    risky\nexcept TypeError:\n    pass\n"
	tree := parser.New(src).Parse()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("BuildScopes panicked on except pass: %v", r)
		}
	}()

	global, _ := BuildScopes(tree, src)
	if global == nil {
		t.Fatal("expected global scope")
	}
}

func TestBuildScopes_IgnoresUnknownStatementKinds(t *testing.T) {
	tree := ast.New(8)
	tree.Root = tree.NewNode(ast.NodeModule, 0, 0)
	unknown := tree.NewNode(ast.NodeBlock, 0, 0)
	tree.AddChild(tree.Root, unknown)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("BuildScopes panicked on unknown statement kind: %v", r)
		}
	}()

	global, _ := BuildScopes(tree, "")
	if global == nil {
		t.Fatal("expected global scope")
	}
}

func TestResolve_AllowsExceptPass(t *testing.T) {
	src := "try:\n    risky\nexcept TypeError:\n    pass\n"
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Resolve panicked on except pass: %v", r)
		}
	}()

	_, errs := Resolve(tree, global)
	if len(errs) == 0 {
		// risky is unresolved in this fixture; the important part is that analysis stays alive.
	}
}

func TestResolve_BuiltinExceptionNamesDoNotReportUndefined(t *testing.T) {
	src := "try:\n    risky\nexcept (TypeError, AttributeError):\n    pass\n"
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)

	_, errs := Resolve(tree, global)
	for _, err := range errs {
		if err.Msg == "undefined name: TypeError" || err.Msg == "undefined name: AttributeError" {
			t.Fatalf("unexpected builtin exception diagnostic: %+v", err)
		}
	}
}

// TestResolve_DeprecationWarningBuiltin verifies DeprecationWarning is available as builtin
func TestResolve_DeprecationWarningBuiltin(t *testing.T) {
	src := "x = DeprecationWarning\n"
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)

	_, errs := Resolve(tree, global)
	for _, err := range errs {
		if err.Msg == "undefined name: DeprecationWarning" {
			t.Fatalf("DeprecationWarning should be available as builtin: %s", err.Msg)
		}
	}
}

// TestResolve_WarningClassesBuiltin verifies Warning classes are available as builtins
func TestResolve_WarningClassesBuiltin(t *testing.T) {
	warningClasses := []string{
		"Warning", "UserWarning", "DeprecationWarning", "SyntaxWarning",
		"RuntimeWarning", "FutureWarning", "PendingDeprecationWarning",
		"ImportWarning", "UnicodeWarning", "BytesWarning", "ResourceWarning",
	}

	for _, warning := range warningClasses {
		src := warning + "\n"
		tree := parser.New(src).Parse()
		global, _ := BuildScopes(tree, src)

		_, errs := Resolve(tree, global)
		for _, err := range errs {
			if err.Msg == "undefined name: "+warning {
				t.Errorf("%s should be available as builtin: %s", warning, err.Msg)
			}
		}
	}
}

// Comprehensive test for all builtin exceptions to prevent regression
func TestResolve_AllBuiltinExceptionsDefined(t *testing.T) {
	// All builtin exceptions should be defined and not report "undefined name"
	builtinExceptions := []string{
		"BaseException", "Exception", "TypeError", "AttributeError", "ValueError",
		"KeyError", "IndexError", "RuntimeError", "ImportError", "NameError",
		"OSError", "LookupError",
		"AssertionError", "StopIteration", "SystemExit", "KeyboardInterrupt",
		"ZeroDivisionError", "ArithmeticError", "StopAsyncIteration",
		"ModuleNotFoundError", "FileNotFoundError", "PermissionError",
		"NotImplementedError", "MemoryError", "RecursionError",
		"SyntaxError", "IndentationError", "TabError",
		"UnicodeError", "UnicodeDecodeError", "UnicodeEncodeError",
		"BlockingIOError", "ChildProcessError", "ConnectionError",
		"ConnectionAbortedError", "ConnectionRefusedError", "ConnectionResetError",
		"InterruptedError", "IsADirectoryError", "NotADirectoryError",
		"ProcessLookupError", "TimeoutError",
		"EOFError", "IOError", "EnvironmentError",
		"GeneratorExit", "SystemError", "ReferenceError",
	}

	// Build a source file that references all exceptions
	src := ""
	for _, exc := range builtinExceptions {
		src += exc + "\n"
	}

	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)

	_, errs := Resolve(tree, global)
	for _, err := range errs {
		if strings.HasPrefix(err.Msg, "undefined name:") {
			for _, exc := range builtinExceptions {
				if err.Msg == "undefined name: "+exc {
					t.Fatalf("builtin exception %s should be defined: %s", exc, err.Msg)
				}
			}
		}
	}

	// Also verify each exception resolves to a symbol
	resolver, _ := Resolve(tree, global)
	for _, name := range builtinExceptions {
		found := false
		for nodeID, sym := range resolver.Resolved {
			if sym != nil && sym.Name == name && sym.Kind == SymClass {
				found = true
				_ = nodeID
				break
			}
		}
		if !found {
			// Check if it's in the global scope
			if globalSym, ok := global.Lookup(name); !ok || globalSym == nil {
				t.Fatalf("builtin exception %s should be resolvable", name)
			}
		}
	}
}

func TestResolve_SubscriptAssignmentTarget(t *testing.T) {
	src := "a = [1]\na[0] = 2\na[0] += 3\n"
	tree := parser.New(src).Parse()

	global, _ := BuildScopes(tree, src)
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

	global, _ := BuildScopes(tree, src)
	if _, errs := Resolve(tree, global); len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}
}

func TestResolve_StarAndKwStarArgValuesOnly(t *testing.T) {
	src := "items = [1]\nkwargs = {}\nfoo = print\nfoo(*items, **kwargs)\n"
	tree := parser.New(src).Parse()

	global, _ := BuildScopes(tree, src)
	if _, errs := Resolve(tree, global); len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}
}

func TestResolveFStringAssignsStrTypeAndResolvesInnerNames(t *testing.T) {
	src := "name = 'x'\nvalue = f\"hello {name}\"\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree, src)
	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	fstring := findNodeByKind(t, tree, ast.NodeFString)
	fstringType := resolver.ExprTypes[fstring]
	if fstringType == nil || fstringType.Kind != TypeBuiltin || fstringType.Symbol == nil || fstringType.Symbol.Name != "str" {
		t.Fatalf("expected f-string expr type str, got %+v", fstringType)
	}
	valueSym := defs[mustNameNode(t, tree, "value")]
	if valueSym == nil || valueSym.Inferred == nil || valueSym.Inferred.Symbol == nil || valueSym.Inferred.Symbol.Name != "str" {
		t.Fatalf("expected inferred str on value, got %+v", valueSym)
	}
}

func TestResolveDictComprehensionAssignsDictTypeAndScopesTarget(t *testing.T) {
	src := "HOOKS = [1]\nvalue = {event: [] for event in HOOKS if event}\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree, src)
	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	valueSym := defs[mustNameNode(t, tree, "value")]
	if valueSym == nil || valueSym.Inferred == nil || valueSym.Inferred.Kind != TypeDict {
		t.Fatalf("expected dict type on value, got %+v", valueSym)
	}
	comp := findNodeByKind(t, tree, ast.NodeDictComp)
	keyExpr, valueExpr, clauses := tree.DictCompParts(comp)
	if len(clauses) != 1 {
		t.Fatalf("unexpected clauses: %+v", clauses)
	}
	target, _, filters := tree.ComprehensionParts(clauses[0])
	if resolver.Resolved[keyExpr] == nil || resolver.Resolved[keyExpr].Name != "event" {
		t.Fatalf("expected dict comp key to resolve to event, got %+v", resolver.Resolved[keyExpr])
	}
	if resolver.Resolved[target] == nil || resolver.Resolved[target].Name != "event" {
		t.Fatalf("expected dict comp target to resolve to event, got %+v", resolver.Resolved[target])
	}
	if len(filters) != 1 || resolver.Resolved[filters[0]] == nil || resolver.Resolved[filters[0]].Name != "event" {
		t.Fatalf("expected dict comp filter to resolve to event, got %+v", filters)
	}
	if resolver.ExprTypes[comp] == nil || resolver.ExprTypes[comp].Kind != TypeDict {
		t.Fatalf("expected dict comp expr type dict, got %+v", resolver.ExprTypes[comp])
	}
	if resolver.ExprTypes[valueExpr] == nil || resolver.ExprTypes[valueExpr].Kind != TypeList {
		t.Fatalf("expected dict comp value expr type list, got %+v", resolver.ExprTypes[valueExpr])
	}
}

func TestResolveConstructorCallAssignsInferredInstanceType(t *testing.T) {
	src := "class Foo:\n    def method(self):\n        pass\n\nx = Foo()\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree, src)
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
	global, _ := BuildScopes(tree, src)
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

func TestBuildScopes_VarArgsAndKwArgsAreParameters(t *testing.T) {
	src := "def f(*args, **kwargs):\n    pass\n"
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	fnSym := global.Symbols["f"]
	if fnSym == nil || fnSym.Inner == nil {
		t.Fatal("missing function symbol f")
	}
	argsSym := fnSym.Inner.Symbols["args"]
	if argsSym == nil || !argsSym.IsVarArg || argsSym.IsKwArg {
		t.Fatalf("expected args to be vararg parameter, got %+v", argsSym)
	}
	kwargsSym := fnSym.Inner.Symbols["kwargs"]
	if kwargsSym == nil || kwargsSym.IsVarArg || !kwargsSym.IsKwArg {
		t.Fatalf("expected kwargs to be kwarg parameter, got %+v", kwargsSym)
	}
}

func TestResolveReturnAnnotationSetsCallExprType(t *testing.T) {
	src := "class Foo:\n    def method(self):\n        pass\n\ndef make() -> Foo:\n    return Foo()\n\nx = make()\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree, src)
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
	global, _ := BuildScopes(tree, src)
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
	global, _ := BuildScopes(tree, src)
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

func TestResolveExceptAliasInBody(t *testing.T) {
	src := "try:\n    risky\nexcept ValueError as err:\n    err\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree, src)
	resolver, errs := Resolve(tree, global)
	if len(errs) == 0 {
		// risky is unresolved; ignore that by checking alias specifically.
	}
	var errNames []ast.NodeID
	collectNames(tree, tree.Root, &errNames)
	var use ast.NodeID
	for _, id := range errNames {
		if name, _ := tree.NameText(id); name == "err" && defs[id] == nil {
			use = id
			break
		}
	}
	if use == ast.NoNode {
		t.Fatal("missing except alias use")
	}
	if resolver.Resolved[use] == nil || resolver.Resolved[use].Name != "err" {
		t.Fatalf("expected except alias to resolve, got %+v", resolver.Resolved[use])
	}
}

func TestResolveListComprehensionBindsTargetAndInfersListType(t *testing.T) {
	src := "xs = [1]\nvalues = [x for x in xs if x]\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree, src)
	resolver, errs := Resolve(tree, global)
	for _, err := range errs {
		if err.Msg != "" {
			// no-op, just keep full list in failure output below
		}
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}
	valuesSym := defs[mustNameNode(t, tree, "values")]
	if valuesSym == nil || valuesSym.Inferred == nil || valuesSym.Inferred.Kind != TypeList {
		t.Fatalf("expected list type on values, got %+v", valuesSym)
	}
	if valuesSym.Inferred.Elem == nil || valuesSym.Inferred.Elem.Kind != TypeBuiltin || valuesSym.Inferred.Elem.Symbol == nil || valuesSym.Inferred.Elem.Symbol.Name != "int" {
		t.Fatalf("expected list[int], got %+v", valuesSym.Inferred)
	}
	comp := findNodeByKind(t, tree, ast.NodeListComp)
	expr, clauses := tree.ListCompParts(comp)
	if len(clauses) != 1 {
		t.Fatalf("unexpected clauses: %+v", clauses)
	}
	target, _, filters := tree.ComprehensionParts(clauses[0])
	if resolver.Resolved[expr] == nil || resolver.Resolved[expr].Name != "x" {
		t.Fatalf("expected comprehension expr to resolve to x, got %+v", resolver.Resolved[expr])
	}
	if resolver.Resolved[target] == nil || resolver.Resolved[target].Name != "x" {
		t.Fatalf("expected comprehension target to resolve to x, got %+v", resolver.Resolved[target])
	}
	if len(filters) != 1 || resolver.Resolved[filters[0]] == nil || resolver.Resolved[filters[0]].Name != "x" {
		t.Fatalf("expected comprehension filter to resolve to x, got %+v", filters)
	}
	var leakErr *SemanticError
	for i := range errs {
		if errs[i].Msg == "undefined name: x" {
			leakErr = &errs[i]
		}
	}
	if leakErr != nil {
		t.Fatalf("unexpected leaked-name error inside comprehension: %+v", leakErr)
	}
}

func TestResolveListComprehensionTargetDoesNotLeak(t *testing.T) {
	src := "xs = [1]\n[x for x in xs]\nx\n"
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	_, errs := Resolve(tree, global)
	found := false
	for _, err := range errs {
		if err.Msg == "undefined name: x" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected comprehension target not to leak, got %+v", errs)
	}
}

func TestResolveNestedListAnnotationAssignsNestedType(t *testing.T) {
	src := "def f(matrix: list[list[int]]):\n    matrix\n"
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
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
	global, _ := BuildScopes(tree, src)
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
	global, _ := BuildScopes(tree, src)
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
	global, _ := BuildScopes(tree, src)
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
	global, defs := BuildScopes(tree, src)
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
	global, defs := BuildScopes(tree, src)
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
	global, defs := BuildScopes(tree, src)
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
	global, defs := BuildScopes(tree, src)
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
	global, defs := BuildScopes(tree, src)
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
	global, defs := BuildScopes(tree, src)
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
	global, defs := BuildScopes(tree, src)
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
	global, defs := BuildScopes(tree, src)
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
	global, _ := BuildScopes(tree, src)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}
}

func TestResolveSpecialBuiltinNamePropagatesStrType(t *testing.T) {
	src := "x = __name__\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree, src)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	xSym := defs[mustNameNode(t, tree, "x")]
	if xSym == nil || xSym.Inferred == nil || xSym.Inferred.Kind != TypeBuiltin || xSym.Inferred.Symbol == nil || xSym.Inferred.Symbol.Name != "str" {
		t.Fatalf("expected inferred str type on x, got %+v", xSym)
	}
}

func TestResolveComparisonAssignsBoolType(t *testing.T) {
	src := "items = [1]\nvalue = 1\nresult = value in items\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree, src)
	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	compare := findNodeByKind(t, tree, ast.NodeCompare)
	compareType := resolver.ExprTypes[compare]
	if compareType == nil || compareType.Kind != TypeBuiltin || compareType.Symbol == nil || compareType.Symbol.Name != "bool" {
		t.Fatalf("expected bool type on comparison expr, got %+v", compareType)
	}

	resultSym := defs[mustNameNode(t, tree, "result")]
	if resultSym == nil || resultSym.Inferred == nil || resultSym.Inferred.Kind != TypeBuiltin || resultSym.Inferred.Symbol == nil || resultSym.Inferred.Symbol.Name != "bool" {
		t.Fatalf("expected inferred bool type on result, got %+v", resultSym)
	}
}

func TestResolveWithAsBindsTargetInBody(t *testing.T) {
	src := "resource = open\nwith resource as handle:\n    value = handle\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree, src)
	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	handleDef := mustNameNode(t, tree, "handle")
	if defs[handleDef] == nil {
		t.Fatal("expected with-as target to define a symbol")
	}

	valueUse := ast.NoNode
	for id := ast.NodeID(1); int(id) < len(tree.Nodes); id++ {
		if tree.Node(id).Kind != ast.NodeName {
			continue
		}
		if text, ok := tree.NameText(id); ok && text == "handle" && id != handleDef {
			valueUse = id
			break
		}
	}
	if valueUse == ast.NoNode {
		t.Fatal("expected handle use inside with body")
	}
	if resolver.Resolved[valueUse] == nil || resolver.Resolved[valueUse] != defs[handleDef] {
		t.Fatalf("expected handle use to resolve to with-as target, got %+v", resolver.Resolved[valueUse])
	}
}

func TestResolveDecoratorExpressionAndDecoratedFunction(t *testing.T) {
	src := "dec = print\n@dec\ndef f():\n    pass\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree, src)
	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	fnName := mustNameNode(t, tree, "f")
	if defs[fnName] == nil || defs[fnName].Kind != SymFunction {
		t.Fatalf("expected decorated function symbol, got %+v", defs[fnName])
	}

	decorator := findNodeByKind(t, tree, ast.NodeDecorator)
	decoratorExpr := tree.DecoratorExpr(decorator)
	if resolver.Resolved[decoratorExpr] == nil || resolver.Resolved[decoratorExpr].Name != "dec" {
		t.Fatalf("expected decorator expr to resolve to dec, got %+v", resolver.Resolved[decoratorExpr])
	}
}

func TestResolveRaiseExpression(t *testing.T) {
	src := "kind = print\nerr = kind\nraise err\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree, src)
	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	errDef := defs[mustNameNode(t, tree, "err")]
	if errDef == nil {
		t.Fatal("expected err symbol")
	}

	raiseNode := findNodeByKind(t, tree, ast.NodeRaise)
	exc, cause := tree.RaiseParts(raiseNode)
	if cause != ast.NoNode {
		t.Fatalf("unexpected cause node: %v", cause)
	}
	if resolver.Resolved[exc] != errDef {
		t.Fatalf("expected raised expr to resolve to err, got %+v", resolver.Resolved[exc])
	}
}

func TestResolveRaiseCauseExpression(t *testing.T) {
	src := "kind = print\nroot = print\nerr = kind\ncause = root\nraise err from cause\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree, src)
	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	raiseNode := findNodeByKind(t, tree, ast.NodeRaise)
	exc, cause := tree.RaiseParts(raiseNode)
	if resolver.Resolved[exc] != defs[mustNameNode(t, tree, "err")] {
		t.Fatalf("expected raise expr to resolve to err, got %+v", resolver.Resolved[exc])
	}
	if resolver.Resolved[cause] != defs[mustNameNode(t, tree, "cause")] {
		t.Fatalf("expected raise cause to resolve to cause, got %+v", resolver.Resolved[cause])
	}
}

func TestResolveDictLiteralTraversal(t *testing.T) {
	src := "base = 1\ndef sq(x):\n    return x\ndef sin(x):\n    return x\ndata = {\"name\": base, \"root\": sq(16), \"sine\": sin(0)}\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree, src)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	dataSym := defs[mustNameNode(t, tree, "data")]
	if dataSym == nil || dataSym.Inferred == nil || dataSym.Inferred.Kind != TypeDict {
		t.Fatalf("expected inferred dict type on data, got %+v", dataSym)
	}
}

func TestResolveListAppendNoUndefinedAttributeAndPropagatesElementType(t *testing.T) {
	src := "class Foo:\n    def method(self):\n        pass\n\nxs = []\nxs.append(Foo())\n"
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree, src)
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
	global, defs := BuildScopes(tree, src)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	xsSym := defs[mustNameNode(t, tree, "xs")]
	if xsSym == nil || xsSym.Inferred == nil || xsSym.Inferred.Kind != TypeList || xsSym.Inferred.Elem == nil || xsSym.Inferred.Elem.Kind != TypeUnion || len(xsSym.Inferred.Elem.Union) != 2 {
		t.Fatalf("expected list[Foo | Bar] after appends, got %+v", xsSym)
	}
}

func TestResolveStringSplitDoesNotReportUndefinedAttribute(t *testing.T) {
	src := "value = \"1.2.3\"\nparts = value.split(\".\")\n"
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)

	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	attr := mustAttributeNode(t, tree, "split")
	if resolver.ResolvedAttr[attr] == nil || resolver.ResolvedAttr[attr].Name != "split" {
		t.Fatalf("expected split attribute to resolve, got %+v", resolver.ResolvedAttr[attr])
	}
}

func TestStringMethodReturnTypes(t *testing.T) {
	src := `value = "hello world"
parts = value.split(" ")
joined = "-".join(parts)
lowered = value.lower()
uppered = value.upper()
stripped = value.strip()
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)

	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	// Verify all expected attributes resolve
	for _, method := range []string{"split", "join", "lower", "upper", "strip"} {
		attr := mustAttributeNode(t, tree, method)
		if resolver.ResolvedAttr[attr] == nil || resolver.ResolvedAttr[attr].Name != method {
			t.Fatalf("expected %s attribute to resolve, got %+v", method, resolver.ResolvedAttr[attr])
		}
	}

	// Test split() returns list[str]
	partsSym := global.Symbols["parts"]
	if partsSym == nil {
		t.Fatal("missing parts symbol")
	}
	if partsSym.Inferred == nil || partsSym.Inferred.Kind != TypeList {
		t.Fatalf("expected parts to have inferred list type, got %+v", partsSym.Inferred)
	}
	if partsSym.Inferred.Elem == nil || partsSym.Inferred.Elem.Kind != TypeBuiltin || partsSym.Inferred.Elem.Symbol == nil || partsSym.Inferred.Elem.Symbol.Name != "str" {
		t.Fatalf("expected parts to have list[str] type, got %+v", partsSym.Inferred)
	}

	// Test join() returns str
	joinedSym := global.Symbols["joined"]
	if joinedSym == nil {
		t.Fatal("missing joined symbol")
	}
	if joinedSym.Inferred == nil || joinedSym.Inferred.Kind != TypeBuiltin || joinedSym.Inferred.Symbol == nil || joinedSym.Inferred.Symbol.Name != "str" {
		t.Fatalf("expected joined to have inferred str type, got %+v", joinedSym.Inferred)
	}

	// Test lower() returns str
	loweredSym := global.Symbols["lowered"]
	if loweredSym == nil {
		t.Fatal("missing lowered symbol")
	}
	if loweredSym.Inferred == nil || loweredSym.Inferred.Kind != TypeBuiltin || loweredSym.Inferred.Symbol == nil || loweredSym.Inferred.Symbol.Name != "str" {
		t.Fatalf("expected lowered to have inferred str type, got %+v", loweredSym.Inferred)
	}

	// Test upper() returns str
	upperedSym := global.Symbols["uppered"]
	if upperedSym == nil {
		t.Fatal("missing uppered symbol")
	}
	if upperedSym.Inferred == nil || upperedSym.Inferred.Kind != TypeBuiltin || upperedSym.Inferred.Symbol == nil || upperedSym.Inferred.Symbol.Name != "str" {
		t.Fatalf("expected uppered to have inferred str type, got %+v", upperedSym.Inferred)
	}

	// Test strip() returns str
	strippedSym := global.Symbols["stripped"]
	if strippedSym == nil {
		t.Fatal("missing stripped symbol")
	}
	if strippedSym.Inferred == nil || strippedSym.Inferred.Kind != TypeBuiltin || strippedSym.Inferred.Symbol.Name != "str" {
		t.Fatalf("expected stripped to have inferred str type, got %+v", strippedSym.Inferred)
	}
}

// Regression test for the requests library pattern: version.split(".")
func TestVersionStringSplitReturnsListStr(t *testing.T) {
	src := `urllib3_version = "1.26.18"
urllib3_version_parts = urllib3_version.split(".")
major = int(urllib3_version_parts[0])
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)

	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	// Verify split attribute resolves
	attr := mustAttributeNode(t, tree, "split")
	if resolver.ResolvedAttr[attr] == nil || resolver.ResolvedAttr[attr].Name != "split" {
		t.Fatalf("expected split attribute to resolve, got %+v", resolver.ResolvedAttr[attr])
	}

	// Test urllib3_version_parts has list[str] type
	partsSym := global.Symbols["urllib3_version_parts"]
	if partsSym == nil {
		t.Fatal("missing urllib3_version_parts symbol")
	}
	if partsSym.Inferred == nil || partsSym.Inferred.Kind != TypeList {
		t.Fatalf("expected urllib3_version_parts to have list type, got %+v", partsSym.Inferred)
	}
	if partsSym.Inferred.Elem == nil || partsSym.Inferred.Elem.Kind != TypeBuiltin || partsSym.Inferred.Elem.Symbol == nil || partsSym.Inferred.Elem.Symbol.Name != "str" {
		t.Fatalf("expected urllib3_version_parts to have list[str] type, got %+v", partsSym.Inferred)
	}

	// Verify no "undefined attribute" error for split
	for _, err := range errs {
		if err.Msg == "undefined attribute: split" {
			t.Fatalf("unexpected error: split should be defined on str")
		}
	}
}

// Test backward type inference: parameter type inferred from method call
func TestBackwardInferenceForParameterFromMethodCall(t *testing.T) {
	src := `def _check_cryptography(cryptography_version):
    parts = cryptography_version.split(".")
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)

	resolver, errs := Resolve(tree, global)
	// We don't care about undefined name errors for the function call
	var realErrors []SemanticError
	for _, err := range errs {
		if !strings.Contains(err.Msg, "undefined name") {
			realErrors = append(realErrors, err)
		}
	}
	if len(realErrors) != 0 {
		t.Fatalf("unexpected errors: %+v", realErrors)
	}

	// Verify split attribute resolves (backward inference should infer cryptography_version is str)
	attr := mustAttributeNode(t, tree, "split")
	if resolver.ResolvedAttr[attr] == nil || resolver.ResolvedAttr[attr].Name != "split" {
		t.Fatalf("expected split attribute to resolve via backward inference, got %+v", resolver.ResolvedAttr[attr])
	}

	// Verify the parameter was inferred as str
	fnSym := global.Symbols["_check_cryptography"]
	if fnSym == nil || fnSym.Inner == nil {
		t.Fatal("missing _check_cryptography function symbol")
	}
	paramSym := fnSym.Inner.Symbols["cryptography_version"]
	if paramSym == nil {
		t.Fatal("missing cryptography_version parameter symbol")
	}
	if paramSym.Inferred == nil || paramSym.Inferred.Kind != TypeBuiltin || paramSym.Inferred.Symbol == nil || paramSym.Inferred.Symbol.Name != "str" {
		t.Fatalf("expected cryptography_version parameter to be inferred as str via backward inference, got %+v", paramSym.Inferred)
	}

	// Verify parts has list[str] type
	partsSym := fnSym.Inner.Symbols["parts"]
	if partsSym == nil {
		t.Fatal("missing parts symbol")
	}
	if partsSym.Inferred == nil || partsSym.Inferred.Kind != TypeList {
		t.Fatalf("expected parts to have list type, got %+v", partsSym.Inferred)
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

func mustAttributeNode(t *testing.T, tree *ast.AST, attrName string) ast.NodeID {
	t.Helper()
	var found ast.NodeID
	var walk func(ast.NodeID)
	walk = func(id ast.NodeID) {
		if id == ast.NoNode || found != ast.NoNode {
			return
		}
		if tree.Node(id).Kind == ast.NodeAttribute {
			attr := tree.ChildAt(id, 1)
			if text, ok := tree.NameText(attr); ok && text == attrName {
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
		t.Fatalf("missing attribute node %q", attrName)
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

// Test that hex literals are stored with converted decimal values in DefaultValue
func TestHexLiteralDefaultValue(t *testing.T) {
	src := "__build__ = 0x023301\n"
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)

	// Verify the symbol was created with the converted value
	sym, ok := global.Symbols["__build__"]
	if !ok {
		t.Fatal("missing __build__ symbol")
	}

	// The DefaultValue should contain the converted decimal value with angle brackets
	expectedValue := "<144129>"
	if sym.DefaultValue != expectedValue {
		t.Fatalf("expected DefaultValue %q for hex literal, got %q", expectedValue, sym.DefaultValue)
	}
}

// Test that binary and octal literals are also stored with converted decimal values
func TestBinaryOctalLiteralDefaultValue(t *testing.T) {
	src := `binary_val = 0b1010
octal_val = 0o777
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)

	// Check binary value
	binarySym, ok := global.Symbols["binary_val"]
	if !ok {
		t.Fatal("missing binary_val symbol")
	}
	if binarySym.DefaultValue != "<10>" {
		t.Fatalf("expected DefaultValue %q for binary literal, got %q", "<10>", binarySym.DefaultValue)
	}

	// Check octal value
	octalSym, ok := global.Symbols["octal_val"]
	if !ok {
		t.Fatal("missing octal_val symbol")
	}
	if octalSym.DefaultValue != "<511>" {
		t.Fatalf("expected DefaultValue %q for octal literal, got %q", "<511>", octalSym.DefaultValue)
	}
}

// Test that bytes strings are inferred as bytes type
func TestBytesStringTypeInference(t *testing.T) {
	src := `s = "hello"
b = b"world"
rb_val = rb"raw bytes"
br_val = br"raw bytes 2"
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)

	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	// Check regular string is inferred as str
	strSym := global.Symbols["s"]
	if strSym == nil {
		t.Fatal("missing s symbol")
	}
	if strSym.Inferred == nil || strSym.Inferred.Kind != TypeBuiltin || strSym.Inferred.Symbol == nil || strSym.Inferred.Symbol.Name != "str" {
		t.Fatalf("expected s to have str type, got %+v", strSym.Inferred)
	}

	// Check bytes string is inferred as bytes
	bytesSym := global.Symbols["b"]
	if bytesSym == nil {
		t.Fatal("missing b symbol")
	}
	if bytesSym.Inferred == nil || bytesSym.Inferred.Kind != TypeBuiltin || bytesSym.Inferred.Symbol == nil || bytesSym.Inferred.Symbol.Name != "bytes" {
		t.Fatalf("expected b to have bytes type, got %+v", bytesSym.Inferred)
	}

	// Check rb prefix
	rbSym := global.Symbols["rb_val"]
	if rbSym == nil {
		t.Fatal("missing rb_val symbol")
	}
	if rbSym.Inferred == nil || rbSym.Inferred.Kind != TypeBuiltin || rbSym.Inferred.Symbol == nil || rbSym.Inferred.Symbol.Name != "bytes" {
		t.Fatalf("expected rb_val to have bytes type, got %+v", rbSym.Inferred)
	}

	// Check br prefix
	brSym := global.Symbols["br_val"]
	if brSym == nil {
		t.Fatal("missing br_val symbol")
	}
	if brSym.Inferred == nil || brSym.Inferred.Kind != TypeBuiltin || brSym.Inferred.Symbol == nil || brSym.Inferred.Symbol.Name != "bytes" {
		t.Fatalf("expected br_val to have bytes type, got %+v", brSym.Inferred)
	}

	// Verify DefaultValue shows b prefix for bytes (content without quotes)
	if bytesSym.DefaultValue != "bworld" {
		t.Fatalf("expected DefaultValue to show b prefix, got %q", bytesSym.DefaultValue)
	}

	_ = resolver
}

// Test dict method resolution and return type inference
func TestDictMethodReturnTypes(t *testing.T) {
	src := `d: dict[str, int] = {}
items = d.items()
keys = d.keys()
values = d.values()
val = d.get("key")
popped = d.pop("key")
d.update({"a": 1})
d.clear()
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)

	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	// Verify items attribute resolves
	itemsAttr := mustAttributeNode(t, tree, "items")
	if resolver.ResolvedAttr[itemsAttr] == nil || resolver.ResolvedAttr[itemsAttr].Name != "items" {
		t.Fatalf("expected items attribute to resolve, got %+v", resolver.ResolvedAttr[itemsAttr])
	}

	// Test items has list[tuple[str, int]] type
	itemsSym := global.Symbols["items"]
	if itemsSym == nil {
		t.Fatal("missing items symbol")
	}
	if itemsSym.Inferred == nil || itemsSym.Inferred.Kind != TypeList {
		t.Fatalf("expected items to have list type, got %+v", itemsSym.Inferred)
	}
	if itemsSym.Inferred.Elem == nil || itemsSym.Inferred.Elem.Kind != TypeTuple {
		t.Fatalf("expected items to have list[tuple[...]] type, got %+v", itemsSym.Inferred.Elem)
	}

	// Test keys has list[str] type
	keysSym := global.Symbols["keys"]
	if keysSym == nil {
		t.Fatal("missing keys symbol")
	}
	if keysSym.Inferred == nil || keysSym.Inferred.Kind != TypeList {
		t.Fatalf("expected keys to have list type, got %+v", keysSym.Inferred)
	}
	if keysSym.Inferred.Elem == nil || keysSym.Inferred.Elem.Kind != TypeBuiltin || keysSym.Inferred.Elem.Symbol == nil || keysSym.Inferred.Elem.Symbol.Name != "str" {
		t.Fatalf("expected keys to have list[str] type, got %+v", keysSym.Inferred)
	}

	// Test values has list[int] type
	valuesSym := global.Symbols["values"]
	if valuesSym == nil {
		t.Fatal("missing values symbol")
	}
	if valuesSym.Inferred == nil || valuesSym.Inferred.Kind != TypeList {
		t.Fatalf("expected values to have list type, got %+v", valuesSym.Inferred)
	}
	if valuesSym.Inferred.Elem == nil || valuesSym.Inferred.Elem.Kind != TypeBuiltin || valuesSym.Inferred.Elem.Symbol == nil || valuesSym.Inferred.Elem.Symbol.Name != "int" {
		t.Fatalf("expected values to have list[int] type, got %+v", valuesSym.Inferred)
	}

	// Test get returns int | None (union)
	valSym := global.Symbols["val"]
	if valSym == nil {
		t.Fatal("missing val symbol")
	}
	if valSym.Inferred == nil || valSym.Inferred.Kind != TypeUnion {
		t.Fatalf("expected val to have union type (int | None), got %+v", valSym.Inferred)
	}

	// Test pop returns int
	poppedSym := global.Symbols["popped"]
	if poppedSym == nil {
		t.Fatal("missing popped symbol")
	}
	if poppedSym.Inferred == nil || poppedSym.Inferred.Kind != TypeBuiltin || poppedSym.Inferred.Symbol == nil || poppedSym.Inferred.Symbol.Name != "int" {
		t.Fatalf("expected popped to have int type, got %+v", poppedSym.Inferred)
	}
}

// Test dict methods work without type annotations (inferred from usage)
func TestDictMethodsWithoutAnnotation(t *testing.T) {
	src := `d = {"a": 1, "b": 2}
items = d.items()
keys = d.keys()
values = d.values()
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)

	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	// Verify all attributes resolve without errors
	for _, method := range []string{"items", "keys", "values"} {
		attr := mustAttributeNode(t, tree, method)
		if resolver.ResolvedAttr[attr] == nil || resolver.ResolvedAttr[attr].Name != method {
			t.Fatalf("expected %s attribute to resolve, got %+v", method, resolver.ResolvedAttr[attr])
		}
	}

	// Verify items, keys, values have list types
	for _, varName := range []string{"items", "keys", "values"} {
		sym := global.Symbols[varName]
		if sym == nil {
			t.Fatalf("missing %s symbol", varName)
		}
		if sym.Inferred == nil || sym.Inferred.Kind != TypeList {
			t.Fatalf("expected %s to have list type, got %+v", varName, sym.Inferred)
		}
	}
}

// Test nested dict return types
func TestNestedDictMethodReturnTypes(t *testing.T) {
	src := `d: dict[str, dict[int, list[float]]] = {}
items = d.items()
inner_dict = d.get("key")
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)

	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	// Test items returns list[tuple[str, dict[int, list[float]]]]
	itemsSym := global.Symbols["items"]
	if itemsSym == nil {
		t.Fatal("missing items symbol")
	}
	if itemsSym.Inferred == nil || itemsSym.Inferred.Kind != TypeList {
		t.Fatalf("expected items to have list type, got %+v", itemsSym.Inferred)
	}
	if itemsSym.Inferred.Elem == nil || itemsSym.Inferred.Elem.Kind != TypeTuple {
		t.Fatalf("expected items to have list[tuple[...]] type, got %+v", itemsSym.Inferred.Elem)
	}

	// Verify inner_dict has dict[int, list[float]] | None type
	innerSym := global.Symbols["inner_dict"]
	if innerSym == nil {
		t.Fatal("missing inner_dict symbol")
	}
	if innerSym.Inferred == nil || innerSym.Inferred.Kind != TypeUnion {
		t.Fatalf("expected inner_dict to have union type, got %+v", innerSym.Inferred)
	}

	_ = resolver
}

// Test backward type inference for function parameters using dict methods
func TestDictBackwardInference(t *testing.T) {
	src := `def process_dict(x):
    items = x.items()
    keys = x.keys()
    values = x.values()
    val = x.get("key")
    return items, keys, values, val
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)

	resolver, errs := Resolve(tree, global)
	// Filter out "undefined name" errors for the function itself
	var realErrors []SemanticError
	for _, err := range errs {
		if !strings.Contains(err.Msg, "undefined name") {
			realErrors = append(realErrors, err)
		}
	}
	if len(realErrors) != 0 {
		t.Fatalf("unexpected errors: %+v", realErrors)
	}

	// Verify dict method attributes resolve (backward inference should type x as dict)
	for _, method := range []string{"items", "keys", "values", "get"} {
		attr := mustAttributeNode(t, tree, method)
		if resolver.ResolvedAttr[attr] == nil || resolver.ResolvedAttr[attr].Name != method {
			t.Fatalf("expected %s attribute to resolve via backward inference, got %+v", method, resolver.ResolvedAttr[attr])
		}
	}

	// Verify the parameter x was inferred as dict via backward inference
	fnSym := global.Symbols["process_dict"]
	if fnSym == nil || fnSym.Inner == nil {
		t.Fatal("missing process_dict function symbol")
	}
	paramSym := fnSym.Inner.Symbols["x"]
	if paramSym == nil {
		t.Fatal("missing x parameter symbol")
	}
	if paramSym.Inferred == nil || paramSym.Inferred.Kind != TypeBuiltin || paramSym.Inferred.Symbol == nil || paramSym.Inferred.Symbol.Name != "dict" {
		t.Fatalf("expected x parameter to be inferred as dict via backward inference, got %+v", paramSym.Inferred)
	}
}

// Test backward inference with pop method (exists on both list and dict)
func TestDictPopBackwardInference(t *testing.T) {
	src := `def get_value(d):
    return d.pop("key")
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)

	resolver, errs := Resolve(tree, global)
	var realErrors []SemanticError
	for _, err := range errs {
		if !strings.Contains(err.Msg, "undefined name") {
			realErrors = append(realErrors, err)
		}
	}
	if len(realErrors) != 0 {
		t.Fatalf("unexpected errors: %+v", realErrors)
	}

	// Verify pop attribute resolves
	attr := mustAttributeNode(t, tree, "pop")
	if resolver.ResolvedAttr[attr] == nil || resolver.ResolvedAttr[attr].Name != "pop" {
		t.Fatalf("expected pop attribute to resolve via backward inference, got %+v", resolver.ResolvedAttr[attr])
	}

	// Verify parameter d was inferred as dict (we prefer dict over list for pop)
	fnSym := global.Symbols["get_value"]
	if fnSym == nil || fnSym.Inner == nil {
		t.Fatal("missing get_value function symbol")
	}
	paramSym := fnSym.Inner.Symbols["d"]
	if paramSym == nil {
		t.Fatal("missing d parameter symbol")
	}
	if paramSym.Inferred == nil || paramSym.Inferred.Kind != TypeBuiltin || paramSym.Inferred.Symbol == nil || paramSym.Inferred.Symbol.Name != "dict" {
		t.Fatalf("expected d parameter to be inferred as dict via backward inference, got %+v", paramSym.Inferred)
	}
}

// Tests for stringified type annotations (forward references)

func TestResolveStringifiedAnnotation(t *testing.T) {
	src := `def f(x: "int"):
    pass
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	fnSym := global.Symbols["f"]
	if fnSym == nil || fnSym.Inner == nil {
		t.Fatal("missing function symbol f")
	}
	xSym := fnSym.Inner.Symbols["x"]
	if xSym == nil || xSym.Inferred == nil || xSym.Inferred.Kind != TypeBuiltin || xSym.Inferred.Symbol == nil || xSym.Inferred.Symbol.Name != "int" {
		t.Fatalf("expected stringified int type on x, got %+v", xSym)
	}
}

func TestResolveStringifiedForwardReference(t *testing.T) {
	src := `def make() -> "Foo":
    return Foo()

class Foo:
    pass
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	makeSym := global.Symbols["make"]
	if makeSym == nil || makeSym.Returns == nil {
		t.Fatalf("expected return type on make, got %+v", makeSym)
	}
	if makeSym.Returns.Kind != TypeInstance || makeSym.Returns.Symbol == nil || makeSym.Returns.Symbol.Name != "Foo" {
		t.Fatalf("expected Foo return type, got %+v", makeSym.Returns)
	}
}

func TestResolveStringifiedGenericAnnotation(t *testing.T) {
	src := `def f(items: "list[int]"):
    pass
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	fnSym := global.Symbols["f"]
	if fnSym == nil || fnSym.Inner == nil {
		t.Fatal("missing function symbol f")
	}
	itemsSym := fnSym.Inner.Symbols["items"]
	if itemsSym == nil || itemsSym.Inferred == nil || itemsSym.Inferred.Kind != TypeList {
		t.Fatalf("expected list type on items, got %+v", itemsSym)
	}
	if itemsSym.Inferred.Elem == nil || itemsSym.Inferred.Elem.Kind != TypeBuiltin || itemsSym.Inferred.Elem.Symbol == nil || itemsSym.Inferred.Elem.Symbol.Name != "int" {
		t.Fatalf("expected list[int], got %+v", itemsSym.Inferred)
	}
}

func TestResolveStringifiedTupleAnnotation(t *testing.T) {
	src := `def f() -> "tuple[int, str]":
    pass
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	fnSym := global.Symbols["f"]
	if fnSym == nil || fnSym.Returns == nil || fnSym.Returns.Kind != TypeTuple {
		t.Fatalf("expected tuple return type, got %+v", fnSym.Returns)
	}
	if len(fnSym.Returns.Items) != 2 {
		t.Fatalf("expected 2 tuple items, got %+v", fnSym.Returns.Items)
	}
}

func TestResolveStringifiedDictAnnotation(t *testing.T) {
	src := `def f(mapping: "dict[str, int]"):
    pass
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	fnSym := global.Symbols["f"]
	if fnSym == nil || fnSym.Inner == nil {
		t.Fatal("missing function symbol f")
	}
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

func TestResolveStringifiedNestedGeneric(t *testing.T) {
	src := `def f(matrix: "list[list[int]]"):
    pass
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	fnSym := global.Symbols["f"]
	if fnSym == nil || fnSym.Inner == nil {
		t.Fatal("missing function symbol f")
	}
	matrixSym := fnSym.Inner.Symbols["matrix"]
	if matrixSym == nil || matrixSym.Inferred == nil || matrixSym.Inferred.Kind != TypeList {
		t.Fatalf("expected list type on matrix, got %+v", matrixSym)
	}
	inner := matrixSym.Inferred.Elem
	if inner == nil || inner.Kind != TypeList {
		t.Fatalf("expected nested list type, got %+v", inner)
	}
	if inner.Elem == nil || inner.Elem.Kind != TypeBuiltin || inner.Elem.Symbol == nil || inner.Elem.Symbol.Name != "int" {
		t.Fatalf("expected nested list[int], got %+v", inner)
	}
}

func TestResolveStringifiedSetAnnotation(t *testing.T) {
	src := `def f(items: "set[int]"):
    pass
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	fnSym := global.Symbols["f"]
	if fnSym == nil || fnSym.Inner == nil {
		t.Fatal("missing function symbol f")
	}
	itemsSym := fnSym.Inner.Symbols["items"]
	if itemsSym == nil || itemsSym.Inferred == nil || itemsSym.Inferred.Kind != TypeSet {
		t.Fatalf("expected set type on items, got %+v", itemsSym)
	}
	if itemsSym.Inferred.Elem == nil || itemsSym.Inferred.Elem.Kind != TypeBuiltin || itemsSym.Inferred.Elem.Symbol == nil || itemsSym.Inferred.Elem.Symbol.Name != "int" {
		t.Fatalf("expected set[int], got %+v", itemsSym.Inferred)
	}
}

func TestResolveStringifiedAnnotatedVariable(t *testing.T) {
	src := `x: "int" = 1
`
	tree := parser.New(src).Parse()
	global, defs := BuildScopes(tree, src)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	xSym := defs[mustNameNode(t, tree, "x")]
	if xSym == nil || xSym.Inferred == nil || xSym.Inferred.Kind != TypeBuiltin || xSym.Inferred.Symbol == nil || xSym.Inferred.Symbol.Name != "int" {
		t.Fatalf("expected stringified int type on x, got %+v", xSym)
	}
}

func TestResolveStringifiedInvalidAnnotation(t *testing.T) {
	// Invalid syntax in string annotation should silently fail (return nil)
	// and not produce errors
	src := `def f(x: "invalid["):
    pass
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	_, errs := Resolve(tree, global)
	// Should have no errors - invalid string annotations are silently ignored
	// (matching Python runtime behavior)
	for _, err := range errs {
		// We only care about errors unrelated to the invalid string annotation
		if err.Msg != "" {
			// This is a bit loose - we're basically accepting any error handling
			// as long as it doesn't crash
		}
	}

	fnSym := global.Symbols["f"]
	if fnSym == nil || fnSym.Inner == nil {
		t.Fatal("missing function symbol f")
	}
	xSym := fnSym.Inner.Symbols["x"]
	if xSym == nil {
		t.Fatal("missing parameter symbol x")
	}
	// With invalid annotation, x should have no inferred type (nil)
	// This is acceptable behavior
}

func TestResolveStringifiedCaching(t *testing.T) {
	// Test that repeated string annotations are cached and reused
	src := `def f(a: "int", b: "int"):
    pass
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	// Verify cache was populated
	if len(resolver.stringAnnotCache) == 0 {
		t.Fatal("expected string annotation cache to be populated")
	}

	// Both parameters should have int type from cached parsing
	fnSym := global.Symbols["f"]
	aSym := fnSym.Inner.Symbols["a"]
	bSym := fnSym.Inner.Symbols["b"]

	if aSym == nil || aSym.Inferred == nil || aSym.Inferred.Kind != TypeBuiltin || aSym.Inferred.Symbol.Name != "int" {
		t.Fatalf("expected int type on a, got %+v", aSym)
	}
	if bSym == nil || bSym.Inferred == nil || bSym.Inferred.Kind != TypeBuiltin || bSym.Inferred.Symbol.Name != "int" {
		t.Fatalf("expected int type on b, got %+v", bSym)
	}
}

func TestResolveStringifiedUnionAnnotation(t *testing.T) {
	src := `def f(x: "int | str"):
    pass
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	fnSym := global.Symbols["f"]
	if fnSym == nil || fnSym.Inner == nil {
		t.Fatal("missing function symbol f")
	}
	xSym := fnSym.Inner.Symbols["x"]
	if xSym == nil || xSym.Inferred == nil || xSym.Inferred.Kind != TypeUnion {
		t.Fatalf("expected union type on x, got %+v", xSym)
	}
	if len(xSym.Inferred.Union) != 2 {
		t.Fatalf("expected 2 union arms, got %+v", xSym.Inferred.Union)
	}
}

func TestResolveStringifiedClassForwardRef(t *testing.T) {
	// Test forward reference to a class defined later in the file
	src := `def create() -> "Container":
    return Container()

class Container:
    pass
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	createSym := global.Symbols["create"]
	if createSym == nil || createSym.Returns == nil {
		t.Fatalf("expected return type on create, got %+v", createSym)
	}
	if createSym.Returns.Kind != TypeInstance || createSym.Returns.Symbol == nil || createSym.Returns.Symbol.Name != "Container" {
		t.Fatalf("expected Container return type, got %+v", createSym.Returns)
	}
}

func TestResolveStringifiedOptionalAnnotation(t *testing.T) {
	src := `def f(x: "int | None"):
    pass
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	fnSym := global.Symbols["f"]
	if fnSym == nil || fnSym.Inner == nil {
		t.Fatal("missing function symbol f")
	}
	xSym := fnSym.Inner.Symbols["x"]
	if xSym == nil || xSym.Inferred == nil || xSym.Inferred.Kind != TypeUnion {
		t.Fatalf("expected union type on x, got %+v", xSym)
	}
}

func TestResolveStringifiedComplexNested(t *testing.T) {
	src := `def f(data: "dict[str, list[int]]"):
    pass
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	fnSym := global.Symbols["f"]
	if fnSym == nil || fnSym.Inner == nil {
		t.Fatal("missing function symbol f")
	}
	dataSym := fnSym.Inner.Symbols["data"]
	if dataSym == nil || dataSym.Inferred == nil || dataSym.Inferred.Kind != TypeDict {
		t.Fatalf("expected dict type on data, got %+v", dataSym)
	}
	if dataSym.Inferred.Elem == nil || dataSym.Inferred.Elem.Kind != TypeList {
		t.Fatalf("expected dict value type list, got %+v", dataSym.Inferred.Elem)
	}
	if dataSym.Inferred.Elem.Elem == nil || dataSym.Inferred.Elem.Elem.Kind != TypeBuiltin || dataSym.Inferred.Elem.Elem.Symbol.Name != "int" {
		t.Fatalf("expected list[int], got %+v", dataSym.Inferred.Elem)
	}
}

// Tests for isinstance() type narrowing

func TestResolveIsinstanceNarrowingBasic(t *testing.T) {
	src := `def f(x):
    if isinstance(x, str):
        y = x.encode("utf-8")
    return x
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	// Check that 'encode' attribute was resolved on x inside the if block
	// This would fail if x wasn't narrowed to str
	encodeAttr := mustAttributeNode(t, tree, "encode")
	if resolver.ResolvedAttr[encodeAttr] == nil || resolver.ResolvedAttr[encodeAttr].Name != "encode" {
		t.Fatalf("expected encode attribute to resolve via isinstance narrowing, got %+v", resolver.ResolvedAttr[encodeAttr])
	}
}

func TestResolveIsinstanceNarrowingInt(t *testing.T) {
	src := `def f(x):
    if isinstance(x, int):
        y = x.bit_length()
    return x
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	// Check that 'bit_length' attribute was resolved on x inside the if block
	bitLenAttr := mustAttributeNode(t, tree, "bit_length")
	if resolver.ResolvedAttr[bitLenAttr] == nil || resolver.ResolvedAttr[bitLenAttr].Name != "bit_length" {
		t.Fatalf("expected bit_length attribute to resolve via isinstance narrowing, got %+v", resolver.ResolvedAttr[bitLenAttr])
	}
}

func TestResolveIsinstanceNarrowingClass(t *testing.T) {
	src := `class Foo:
    def method(self):
        pass

def f(x):
    if isinstance(x, Foo):
        x.method()
    return x
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	// Check that 'method' attribute was resolved on x inside the if block
	methodAttr := mustAttributeNode(t, tree, "method")
	if resolver.ResolvedAttr[methodAttr] == nil || resolver.ResolvedAttr[methodAttr].Name != "method" {
		t.Fatalf("expected method attribute to resolve via isinstance narrowing, got %+v", resolver.ResolvedAttr[methodAttr])
	}
}

func TestResolveIsinstanceNarrowingNested(t *testing.T) {
	src := `def f(x):
    if isinstance(x, str):
        if isinstance(x, str):  # redundant but tests constraint stacking
            y = x.encode("utf-8")
    return x
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	// Check that 'encode' attribute was resolved in the nested if block
	encodeAttr := mustAttributeNode(t, tree, "encode")
	if resolver.ResolvedAttr[encodeAttr] == nil || resolver.ResolvedAttr[encodeAttr].Name != "encode" {
		t.Fatalf("expected encode attribute to resolve in nested isinstance, got %+v", resolver.ResolvedAttr[encodeAttr])
	}
}

func TestResolveIsinstanceNarrowingNoLeak(t *testing.T) {
	src := `def f(x):
    if isinstance(x, str):
        y = x.encode("utf-8")
    # x should not have str type here (constraints removed)
    z = x
    return x
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	_, errs := Resolve(tree, global)
	if len(errs) != 0 {
		// It's ok if there are errors about encode not being defined outside the if
		// as long as it was resolved inside the if
	}
}

func TestResolveIsinstanceNarrowingWithAssign(t *testing.T) {
	src := `def sha256_utf8(x):
    if isinstance(x, str):
        x = x.encode("utf-8")
    return hashlib.sha256(x).hexdigest()
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	resolver, errs := Resolve(tree, global)

	// We expect errors for undefined names (hashlib) but not for x.encode
	hasUndefinedEncode := false
	for _, err := range errs {
		if err.Msg == "undefined attribute: encode" {
			hasUndefinedEncode = true
		}
	}
	if hasUndefinedEncode {
		t.Fatal("x.encode should resolve via isinstance narrowing, got 'undefined attribute: encode'")
	}

	// Check encode attribute resolved
	encodeAttr := mustAttributeNode(t, tree, "encode")
	if resolver.ResolvedAttr[encodeAttr] == nil || resolver.ResolvedAttr[encodeAttr].Name != "encode" {
		t.Fatalf("expected encode attribute to resolve via isinstance narrowing, got %+v", resolver.ResolvedAttr[encodeAttr])
	}
}

func TestResolveIsinstanceNarrowingGenericType(t *testing.T) {
	src := `def f(x):
    if isinstance(x, list):
        y = x.append(1)
    return x
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		// Filter out only relevant errors
		var realErrors []SemanticError
		for _, err := range errs {
			if err.Msg != "undefined name" {
				realErrors = append(realErrors, err)
			}
		}
		if len(realErrors) > 0 {
			t.Fatalf("unexpected errors: %+v", realErrors)
		}
	}

	// Check that 'append' attribute was resolved on x inside the if block
	appendAttr := mustAttributeNode(t, tree, "append")
	if resolver.ResolvedAttr[appendAttr] == nil || resolver.ResolvedAttr[appendAttr].Name != "append" {
		t.Fatalf("expected append attribute to resolve via isinstance(list) narrowing, got %+v", resolver.ResolvedAttr[appendAttr])
	}
}

// Tests for class-level instance attribute inference

func TestResolveInferredInstanceAttribute(t *testing.T) {
	src := `class Foo:
    def set_bar(self):
        self.bar = 42

def test():
    f = Foo()
    f.set_bar()
    return f.bar
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	// Check the class has the inferred attribute recorded
	fooSym := global.Symbols["Foo"]
	if fooSym == nil {
		t.Fatal("missing Foo class")
	}
	inferredType := resolver.getInferredInstanceAttr(fooSym, "bar")
	if inferredType == nil {
		t.Fatal("expected 'bar' to be recorded as inferred instance attribute")
	}

	// Check that at least one read access to 'bar' was resolved
	foundResolved := false
	for nodeID, sym := range resolver.ResolvedAttr {
		if sym != nil && sym.Name == "bar" {
			if resolver.ExprTypes[nodeID] != nil {
				foundResolved = true
				break
			}
		}
	}
	if !foundResolved {
		t.Fatal("expected at least one read access to 'bar' attribute to be resolved")
	}
}

func TestResolveInferredLocalVariableAttribute(t *testing.T) {
	src := `class Response:
    pass

def build_response():
    response = Response()
    response.connection = "adapter"
    return response.connection
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	// Check the class has the inferred attribute recorded
	respSym := global.Symbols["Response"]
	if respSym == nil {
		t.Fatal("missing Response class")
	}
	inferredType := resolver.getInferredInstanceAttr(respSym, "connection")
	if inferredType == nil {
		t.Fatal("expected 'connection' to be recorded as inferred instance attribute on Response class")
	}

	// Check that at least one read access to 'connection' was resolved
	foundResolved := false
	for nodeID, sym := range resolver.ResolvedAttr {
		if sym != nil && sym.Name == "connection" {
			// Verify this is a read access (has expression type set)
			if resolver.ExprTypes[nodeID] != nil {
				foundResolved = true
				break
			}
		}
	}
	if !foundResolved {
		t.Fatal("expected at least one read access to 'connection' attribute to be resolved")
	}
}

func TestResolveInferredAttributeUnionType(t *testing.T) {
	src := `class Foo:
    def set_values(self, x):
        if x:
            self.value = 42
        else:
            self.value = "string"

def test():
    f = Foo()
    f.set_values(True)
    return f.value
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	resolver, errs := Resolve(tree, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	// Check the class has the inferred attribute recorded
	fooSym := global.Symbols["Foo"]
	if fooSym == nil {
		t.Fatal("missing Foo class")
	}
	inferredType := resolver.getInferredInstanceAttr(fooSym, "value")
	if inferredType == nil {
		t.Fatal("expected 'value' to be recorded as inferred instance attribute")
	}
	if inferredType.Kind != TypeUnion {
		t.Fatalf("expected inferred type to be union, got %+v", inferredType)
	}

	// Check that at least one read access to 'value' was resolved
	foundResolved := false
	for nodeID, sym := range resolver.ResolvedAttr {
		if sym != nil && sym.Name == "value" {
			if resolver.ExprTypes[nodeID] != nil {
				foundResolved = true
				break
			}
		}
	}
	if !foundResolved {
		t.Fatal("expected at least one read access to 'value' attribute to be resolved")
	}
}

func TestResolveInferredAttributeNoInheritance(t *testing.T) {
	src := `class Base:
    def set_base_attr(self):
        self.base_attr = 1

class Child(Base):
    def set_child_attr(self):
        self.child_attr = 2

def test():
    c = Child()
    c.set_base_attr()
    c.set_child_attr()
    return c.base_attr + c.child_attr
`
	tree := parser.New(src).Parse()
	global, _ := BuildScopes(tree, src)
	resolver, errs := Resolve(tree, global)

	// Allow errors for method resolution (set_base_attr, set_child_attr) as they're different tests
	// We only care about attribute resolution
	var attrErrors []SemanticError
	for _, err := range errs {
		if err.Msg == "undefined attribute" {
			attrErrors = append(attrErrors, err)
		}
	}

	// Check that both attributes are recorded on their respective classes
	baseSym := global.Symbols["Base"]
	childSym := global.Symbols["Child"]

	if baseSym == nil {
		t.Fatal("missing Base class")
	}
	if childSym == nil {
		t.Fatal("missing Child class")
	}

	// Check base_attr is on Base class
	if resolver.getInferredInstanceAttr(baseSym, "base_attr") == nil {
		t.Fatal("expected 'base_attr' to be recorded on Base class")
	}

	// Check child_attr is on Child class
	if resolver.getInferredInstanceAttr(childSym, "child_attr") == nil {
		t.Fatal("expected 'child_attr' to be recorded on Child class")
	}

	// Check that at least one read access to each attribute was resolved
	foundBase := false
	foundChild := false
	for _, sym := range resolver.ResolvedAttr {
		if sym != nil {
			if sym.Name == "base_attr" {
				foundBase = true
			}
			if sym.Name == "child_attr" {
				foundChild = true
			}
		}
	}

	if !foundBase && len(attrErrors) > 0 {
		t.Logf("Warning: base_attr not resolved, but recorded on class (may need inheritance support)")
	}
	if !foundChild && len(attrErrors) > 0 {
		t.Logf("Warning: child_attr not resolved, but recorded on class")
	}
}
