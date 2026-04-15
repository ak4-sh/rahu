package parser

import (
	"strings"
	"testing"

	a "rahu/parser/ast"
)

func parseSource(t *testing.T, src string) (*Parser, *a.AST) {
	t.Helper()
	p := New(src)
	tree := p.Parse()
	return p, tree
}

func requireNoParseErrors(t *testing.T, p *Parser) {
	t.Helper()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("unexpected parse errors: %+v", errs)
	}
}

func requireParseErrorContains(t *testing.T, p *Parser, want string) {
	t.Helper()
	for _, err := range p.Errors() {
		if strings.Contains(err.Msg, want) {
			return
		}
	}
	t.Fatalf("expected parse error containing %q, got %+v", want, p.Errors())
}

func moduleStmt(t *testing.T, tree *a.AST, index int) a.NodeID {
	t.Helper()
	stmts := children(tree, tree.Root)
	if index < 0 || index >= len(stmts) {
		t.Fatalf("module stmt index %d out of range, got %d stmts", index, len(stmts))
	}
	return stmts[index]
}

func children(tree *a.AST, id a.NodeID) []a.NodeID {
	out := []a.NodeID{}
	for child := tree.Nodes[id].FirstChild; child != a.NoNode; child = tree.Nodes[child].NextSibling {
		out = append(out, child)
	}
	return out
}

func requireKind(t *testing.T, tree *a.AST, id a.NodeID, want a.NodeKind) {
	t.Helper()
	if got := tree.Nodes[id].Kind; got != want {
		t.Fatalf("unexpected node kind: got %s want %s", got, want)
	}
}

func requireChildCount(t *testing.T, tree *a.AST, id a.NodeID, want int) []a.NodeID {
	t.Helper()
	kids := children(tree, id)
	if len(kids) != want {
		t.Fatalf("unexpected child count for %s: got %d want %d", tree.Nodes[id].Kind, len(kids), want)
	}
	return kids
}

func nameText(t *testing.T, tree *a.AST, id a.NodeID) string {
	t.Helper()
	requireKind(t, tree, id, a.NodeName)
	idx := tree.Nodes[id].Data
	if int(idx) >= len(tree.Names) {
		t.Fatalf("name index %d out of range", idx)
	}
	return tree.Names[idx]
}

func stringValue(t *testing.T, tree *a.AST, id a.NodeID) string {
	t.Helper()
	requireKind(t, tree, id, a.NodeString)
	idx := tree.Nodes[id].Data
	if int(idx) >= len(tree.Strings) {
		t.Fatalf("string index %d out of range", idx)
	}
	return tree.Strings[idx]
}

func numberValue(t *testing.T, tree *a.AST, id a.NodeID) string {
	t.Helper()
	requireKind(t, tree, id, a.NodeNumber)
	idx := tree.Nodes[id].Data
	if int(idx) >= len(tree.Numbers) {
		t.Fatalf("number index %d out of range", idx)
	}
	return tree.Numbers[idx]
}

func compareOpValue(t *testing.T, tree *a.AST, id a.NodeID) a.CompareOp {
	t.Helper()
	requireKind(t, tree, id, a.NodeCompareOp)
	return a.CompareOp(tree.Nodes[id].Data)
}

func TestParseFuncParamsAndDefaults(t *testing.T) {
	p, tree := parseSource(t, "def f(a, b=1):\n    x\n")
	requireNoParseErrors(t, p)

	fn := moduleStmt(t, tree, 0)
	requireKind(t, tree, fn, a.NodeFunctionDef)

	fnKids := requireChildCount(t, tree, fn, 3)
	if got := nameText(t, tree, fnKids[0]); got != "f" {
		t.Fatalf("unexpected function name: got %q", got)
	}

	args := fnKids[1]
	requireKind(t, tree, args, a.NodeArgs)
	params := requireChildCount(t, tree, args, 2)

	firstParamKids := requireChildCount(t, tree, params[0], 1)
	if got := nameText(t, tree, firstParamKids[0]); got != "a" {
		t.Fatalf("unexpected first param name: got %q", got)
	}

	secondParamKids := requireChildCount(t, tree, params[1], 2)
	if got := nameText(t, tree, secondParamKids[0]); got != "b" {
		t.Fatalf("unexpected second param name: got %q", got)
	}
	if got := numberValue(t, tree, secondParamKids[1]); got != "1" {
		t.Fatalf("unexpected default value: got %q", got)
	}

	body := fnKids[2]
	requireKind(t, tree, body, a.NodeBlock)
	bodyKids := requireChildCount(t, tree, body, 1)
	requireKind(t, tree, bodyKids[0], a.NodeExprStmt)
}

func TestParseFuncParamAnnotation(t *testing.T) {
	p, tree := parseSource(t, "def f(x: int):\n    x\n")
	requireNoParseErrors(t, p)

	fn := moduleStmt(t, tree, 0)
	name, args, returnAnnotation, _ := tree.FunctionPartsWithReturn(fn)
	if got := nameText(t, tree, name); got != "f" {
		t.Fatalf("unexpected function name: got %q", got)
	}
	if returnAnnotation != a.NoNode {
		t.Fatalf("unexpected return annotation: got %s", tree.Node(returnAnnotation).Kind)
	}

	params := requireChildCount(t, tree, args, 1)
	paramName, annotation, defaultExpr := tree.ParamParts(params[0])
	if got := nameText(t, tree, paramName); got != "x" {
		t.Fatalf("unexpected param name: got %q", got)
	}
	if got := nameText(t, tree, annotation); got != "int" {
		t.Fatalf("unexpected annotation: got %q", got)
	}
	if defaultExpr != a.NoNode {
		t.Fatalf("unexpected default expression: got %s", tree.Node(defaultExpr).Kind)
	}
}

func TestParseFuncParamAnnotationWithDefault(t *testing.T) {
	p, tree := parseSource(t, "def f(x: int = 1):\n    x\n")
	requireNoParseErrors(t, p)

	fn := moduleStmt(t, tree, 0)
	_, args, _, _ := tree.FunctionPartsWithReturn(fn)
	params := requireChildCount(t, tree, args, 1)
	_, annotation, defaultExpr := tree.ParamParts(params[0])
	if got := nameText(t, tree, annotation); got != "int" {
		t.Fatalf("unexpected annotation: got %q", got)
	}
	if got := numberValue(t, tree, defaultExpr); got != "1" {
		t.Fatalf("unexpected default value: got %q", got)
	}
}

func TestParseFuncVarArgsAndKwArgs(t *testing.T) {
	p, tree := parseSource(t, "def f(a, *args: tuple, **kwargs: dict):\n    x\n")
	requireNoParseErrors(t, p)

	fn := moduleStmt(t, tree, 0)
	_, args, _, _ := tree.FunctionPartsWithReturn(fn)
	params := requireChildCount(t, tree, args, 3)
	if tree.ParamIsVarArg(params[0]) || tree.ParamIsKwArg(params[0]) {
		t.Fatal("first param should be plain")
	}
	if !tree.ParamIsVarArg(params[1]) || tree.ParamIsKwArg(params[1]) {
		t.Fatal("second param should be *args")
	}
	if tree.ParamIsVarArg(params[2]) || !tree.ParamIsKwArg(params[2]) {
		t.Fatal("third param should be **kwargs")
	}
	if got := nameText(t, tree, tree.ChildAt(params[1], 0)); got != "args" {
		t.Fatalf("unexpected vararg name: got %q", got)
	}
	if got := nameText(t, tree, tree.ChildAt(params[2], 0)); got != "kwargs" {
		t.Fatalf("unexpected kwarg name: got %q", got)
	}
}

func TestParseFuncVarArgErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{name: "missing vararg name", src: "def f(*):\n    pass\n", want: "expected parameter name after '*'"},
		{name: "missing kwarg name", src: "def f(**):\n    pass\n", want: "expected parameter name after '**'"},
		{name: "vararg default", src: "def f(*args=1):\n    pass\n", want: "*args cannot have a default value"},
		{name: "kwarg default", src: "def f(**kwargs=1):\n    pass\n", want: "**kwargs cannot have a default value"},
		{name: "duplicate vararg", src: "def f(*a, *b):\n    pass\n", want: "duplicate *args parameter"},
		{name: "duplicate kwarg", src: "def f(**a, **b):\n    pass\n", want: "duplicate **kwargs parameter"},
		{name: "after kwarg", src: "def f(**kwargs, a):\n    pass\n", want: "parameter after **kwargs is not allowed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := parseSource(t, tt.src)
			requireParseErrorContains(t, p, tt.want)
		})
	}
}

func TestParseFuncReturnAnnotation(t *testing.T) {
	p, tree := parseSource(t, "def f() -> int:\n    x\n")
	requireNoParseErrors(t, p)

	fn := moduleStmt(t, tree, 0)
	_, _, returnAnnotation, body := tree.FunctionPartsWithReturn(fn)
	if got := nameText(t, tree, returnAnnotation); got != "int" {
		t.Fatalf("unexpected return annotation: got %q", got)
	}
	requireKind(t, tree, body, a.NodeBlock)
}

func TestParseFuncParamUnionAnnotation(t *testing.T) {
	p, tree := parseSource(t, "def f(x: int | None):\n    x\n")
	requireNoParseErrors(t, p)

	fn := moduleStmt(t, tree, 0)
	_, args, _, _ := tree.FunctionPartsWithReturn(fn)
	params := requireChildCount(t, tree, args, 1)
	_, annotation, _ := tree.ParamParts(params[0])
	requireKind(t, tree, annotation, a.NodeBinOp)
	parts := requireChildCount(t, tree, annotation, 2)
	if got := nameText(t, tree, parts[0]); got != "int" {
		t.Fatalf("unexpected left union annotation: got %q", got)
	}
	requireKind(t, tree, parts[1], a.NodeNone)
}

func TestParseFuncReturnUnionAnnotation(t *testing.T) {
	p, tree := parseSource(t, "def f() -> str | None:\n    x\n")
	requireNoParseErrors(t, p)

	fn := moduleStmt(t, tree, 0)
	_, _, returnAnnotation, _ := tree.FunctionPartsWithReturn(fn)
	requireKind(t, tree, returnAnnotation, a.NodeBinOp)
	parts := requireChildCount(t, tree, returnAnnotation, 2)
	if got := nameText(t, tree, parts[0]); got != "str" {
		t.Fatalf("unexpected left return annotation: got %q", got)
	}
	requireKind(t, tree, parts[1], a.NodeNone)
}

func TestParseYieldExpr(t *testing.T) {
	p, tree := parseSource(t, "def f():\n    yield x\n")
	requireNoParseErrors(t, p)

	fn := moduleStmt(t, tree, 0)
	_, _, _, body := tree.FunctionPartsWithReturn(fn)
	stmt := requireChildCount(t, tree, body, 1)[0]
	requireKind(t, tree, stmt, a.NodeExprStmt)
	yieldExpr := requireChildCount(t, tree, stmt, 1)[0]
	requireKind(t, tree, yieldExpr, a.NodeYield)
	child := requireChildCount(t, tree, yieldExpr, 1)[0]
	if got := nameText(t, tree, child); got != "x" {
		t.Fatalf("unexpected yield expression: got %q", got)
	}
}

func TestParseYieldFromExpr(t *testing.T) {
	p, tree := parseSource(t, "def f():\n    yield from xs\n")
	requireNoParseErrors(t, p)

	fn := moduleStmt(t, tree, 0)
	_, _, _, body := tree.FunctionPartsWithReturn(fn)
	stmt := requireChildCount(t, tree, body, 1)[0]
	yieldExpr := requireChildCount(t, tree, stmt, 1)[0]
	requireKind(t, tree, yieldExpr, a.NodeYield)
	if tree.Node(yieldExpr).Data != 1 {
		t.Fatalf("expected yield-from flag, got %d", tree.Node(yieldExpr).Data)
	}
	child := requireChildCount(t, tree, yieldExpr, 1)[0]
	if got := nameText(t, tree, child); got != "xs" {
		t.Fatalf("unexpected yield-from expression: got %q", got)
	}
}

func TestParseFuncPositionalOnlyParams(t *testing.T) {
	p, tree := parseSource(t, "def f(x, /, y):\n    x\n")
	requireNoParseErrors(t, p)

	fn := moduleStmt(t, tree, 0)
	_, args, _, _ := tree.FunctionPartsWithReturn(fn)
	params := requireChildCount(t, tree, args, 2)
	if got := nameText(t, tree, tree.ChildAt(params[0], 0)); got != "x" {
		t.Fatalf("unexpected first param: got %q", got)
	}
	if got := nameText(t, tree, tree.ChildAt(params[1], 0)); got != "y" {
		t.Fatalf("unexpected second param: got %q", got)
	}
}

func TestParseFuncAnnotationErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{name: "missing param annotation", src: "def f(x:):\n    x\n", want: "expected type annotation after ':'"},
		{name: "missing return annotation", src: "def f() ->:\n    x\n", want: "expected return type after '->'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := parseSource(t, tt.src)
			requireParseErrorContains(t, p, tt.want)
		})
	}
}

func TestParseFuncDocstringStoredOnFunction(t *testing.T) {
	p, tree := parseSource(t, "def f():\n    \"doc\"\n    x\n")
	requireNoParseErrors(t, p)

	fn := moduleStmt(t, tree, 0)
	requireKind(t, tree, fn, a.NodeFunctionDef)
	if got := tree.Strings[tree.Nodes[fn].Data]; got != "doc" {
		t.Fatalf("unexpected function docstring: got %q", got)
	}

	fnKids := children(tree, fn)
	body := fnKids[len(fnKids)-1]
	requireKind(t, tree, body, a.NodeBlock)
	bodyKids := requireChildCount(t, tree, body, 1)
	exprStmtKids := requireChildCount(t, tree, bodyKids[0], 1)
	if got := nameText(t, tree, exprStmtKids[0]); got != "x" {
		t.Fatalf("unexpected body expression name: got %q", got)
	}
}

func TestParseFuncNonDefaultAfterDefaultError(t *testing.T) {
	p, _ := parseSource(t, "def f(a=1, b):\n    x\n")
	requireParseErrorContains(t, p, "non-default argument follows default argument")
}

func TestParseDecoratedFunctionShape(t *testing.T) {
	p, tree := parseSource(t, "@dec\n@pkg.wrap(x)\ndef f():\n    y\n")
	requireNoParseErrors(t, p)

	fn := moduleStmt(t, tree, 0)
	requireKind(t, tree, fn, a.NodeFunctionDef)
	decorators := tree.Decorators(fn)
	if len(decorators) != 2 {
		t.Fatalf("unexpected decorator count: %d", len(decorators))
	}
	if got := nameText(t, tree, tree.DecoratorExpr(decorators[0])); got != "dec" {
		t.Fatalf("unexpected first decorator expr: got %q", got)
	}
	if tree.Node(tree.DecoratorExpr(decorators[1])).Kind != a.NodeCall {
		t.Fatalf("expected second decorator to be call expr, got %s", tree.Node(tree.DecoratorExpr(decorators[1])).Kind)
	}
	name, _, body := tree.FunctionParts(fn)
	if got := nameText(t, tree, name); got != "f" {
		t.Fatalf("unexpected function name: got %q", got)
	}
	bodyStmt := requireChildCount(t, tree, body, 1)[0]
	bodyExpr := requireChildCount(t, tree, bodyStmt, 1)[0]
	if got := nameText(t, tree, bodyExpr); got != "y" {
		t.Fatalf("unexpected function body expr: got %q", got)
	}
}

func TestParseFStringShape(t *testing.T) {
	p, tree := parseSource(t, "f\"hello {name}!\"\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	fstring := requireChildCount(t, tree, exprStmt, 1)[0]
	requireKind(t, tree, fstring, a.NodeFString)
	kids := requireChildCount(t, tree, fstring, 3)
	requireKind(t, tree, kids[0], a.NodeFStringText)
	if got, _ := tree.StringText(kids[0]); got != "hello " {
		t.Fatalf("unexpected first f-string text: got %q", got)
	}
	requireKind(t, tree, kids[1], a.NodeFStringExpr)
	if got := nameText(t, tree, tree.ChildAt(kids[1], 0)); got != "name" {
		t.Fatalf("unexpected f-string expr: got %q", got)
	}
	requireKind(t, tree, kids[2], a.NodeFStringText)
	if got, _ := tree.StringText(kids[2]); got != "!" {
		t.Fatalf("unexpected trailing f-string text: got %q", got)
	}
}

func TestParseFStringEscapedBraces(t *testing.T) {
	p, tree := parseSource(t, "f\"{{value}}\"\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	fstring := requireChildCount(t, tree, exprStmt, 1)[0]
	kids := requireChildCount(t, tree, fstring, 1)
	if got, _ := tree.StringText(kids[0]); got != "{value}" {
		t.Fatalf("unexpected escaped brace text: got %q", got)
	}
}

func TestParseFStringConversionAndFormatSpec(t *testing.T) {
	tests := []string{
		"f\"{username!r}\"\n",
		"f\"{value:.2f}\"\n",
		"f\"{value!r:>10}\"\n",
	}

	for _, src := range tests {
		p, tree := parseSource(t, src)
		requireNoParseErrors(t, p)

		exprStmt := moduleStmt(t, tree, 0)
		fstring := requireChildCount(t, tree, exprStmt, 1)[0]
		kids := tree.Children(fstring)
		if len(kids) == 0 {
			t.Fatalf("expected f-string children for %q", src)
		}
		requireKind(t, tree, kids[0], a.NodeFStringExpr)
	}
}

func TestParseFStringErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{name: "unterminated expr", src: "f\"{name\"\n", want: "unterminated f-string expression"},
		{name: "empty expr", src: "f\"{}\"\n", want: "empty f-string expression"},
		{name: "single closing brace", src: "f\"}\"\n", want: "single '}' is not allowed in f-string"},
		{name: "invalid conversion", src: "f\"{value!x}\"\n", want: "invalid f-string conversion"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := parseSource(t, tt.src)
			requireParseErrorContains(t, p, tt.want)
		})
	}
}

func TestParseAdjacentFStringsInsideParens(t *testing.T) {
	src := "base = (\n    f'username=\"{self.username}\", realm=\"{realm}\", nonce=\"{nonce}\", ' \n    f'uri=\"{path}\", response=\"{respdig}\"'\n)\n"
	p, tree := parseSource(t, src)
	requireNoParseErrors(t, p)

	assign := moduleStmt(t, tree, 0)
	requireKind(t, tree, assign, a.NodeAssign)
	value := requireChildCount(t, tree, assign, 2)[0]
	requireKind(t, tree, value, a.NodeFString)
	if len(tree.Children(value)) < 4 {
		t.Fatalf("expected merged f-string children, got %d", len(tree.Children(value)))
	}
}

func TestParseRawStringLiteral(t *testing.T) {
	p, tree := parseSource(t, "x = r\"\\w+\\s+\"\n")
	requireNoParseErrors(t, p)

	assign := moduleStmt(t, tree, 0)
	value := requireChildCount(t, tree, assign, 2)[0]
	requireKind(t, tree, value, a.NodeString)
	if got, _ := tree.StringText(value); got != `\w+\s+` {
		t.Fatalf("unexpected raw string text: got %q", got)
	}
}

func TestParseRawFStringLiteral(t *testing.T) {
	p, tree := parseSource(t, "x = rf\"{name}\\n\"\n")
	requireNoParseErrors(t, p)

	assign := moduleStmt(t, tree, 0)
	value := requireChildCount(t, tree, assign, 2)[0]
	requireKind(t, tree, value, a.NodeFString)
	kids := tree.Children(value)
	if len(kids) != 2 {
		t.Fatalf("unexpected raw f-string child count: %d", len(kids))
	}
	if got, _ := tree.StringText(kids[1]); got != `\n` {
		t.Fatalf("unexpected raw f-string trailing text: got %q", got)
	}
}

func TestParseNestedForWithBlankLineBeforeReturn(t *testing.T) {
	src := "def f():\n    if header:\n        for link in links:\n            key = link.get(\"rel\")\n            resolved_links[key] = link\n\n    return resolved_links\n"
	p, tree := parseSource(t, src)
	requireNoParseErrors(t, p)

	fn := moduleStmt(t, tree, 0)
	_, _, body := tree.FunctionParts(fn)
	bodyKids := requireChildCount(t, tree, body, 2)
	requireKind(t, tree, bodyKids[1], a.NodeReturn)
}

func TestParseAdjacentWarningMessageWithFStringConversion(t *testing.T) {
	src := "msg = (\n    \"Non-string usernames will no longer be supported in Requests \"\n    f\"3.0.0. Please convert the object you've passed in ({username!r}) to \"\n    \"a string or bytes object in the near future to avoid \"\n    \"problems.\"\n)\n"
	p, tree := parseSource(t, src)
	requireNoParseErrors(t, p)

	assign := moduleStmt(t, tree, 0)
	value := requireChildCount(t, tree, assign, 2)[0]
	requireKind(t, tree, value, a.NodeFString)
}

func TestParseDictComprehensionShape(t *testing.T) {
	p, tree := parseSource(t, "{event: [] for event in HOOKS}\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	comp := requireChildCount(t, tree, exprStmt, 1)[0]
	requireKind(t, tree, comp, a.NodeDictComp)
	keyExpr, valueExpr, clauses := tree.DictCompParts(comp)
	if got := nameText(t, tree, keyExpr); got != "event" {
		t.Fatalf("unexpected dict comp key: got %q", got)
	}
	requireKind(t, tree, valueExpr, a.NodeList)
	if len(clauses) != 1 {
		t.Fatalf("unexpected dict comp clauses: %d", len(clauses))
	}
	target, iter, filters := tree.ComprehensionParts(clauses[0])
	if got := nameText(t, tree, target); got != "event" {
		t.Fatalf("unexpected dict comp target: got %q", got)
	}
	if got := nameText(t, tree, iter); got != "HOOKS" {
		t.Fatalf("unexpected dict comp iter: got %q", got)
	}
	if len(filters) != 0 {
		t.Fatalf("unexpected dict comp filters: %d", len(filters))
	}
}

func TestParseDictLiteralStillUsesNodeDict(t *testing.T) {
	p, tree := parseSource(t, "{a: b}\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	dict := requireChildCount(t, tree, exprStmt, 1)[0]
	requireKind(t, tree, dict, a.NodeDict)
}

func TestParseDecoratedClassShape(t *testing.T) {
	p, tree := parseSource(t, "@dec\nclass C:\n    pass\n")
	requireNoParseErrors(t, p)

	classNode := moduleStmt(t, tree, 0)
	requireKind(t, tree, classNode, a.NodeClassDef)
	decorators := tree.Decorators(classNode)
	if len(decorators) != 1 {
		t.Fatalf("unexpected decorator count: %d", len(decorators))
	}
	if got := nameText(t, tree, tree.DecoratorExpr(decorators[0])); got != "dec" {
		t.Fatalf("unexpected class decorator expr: got %q", got)
	}
	name, _, _ := tree.ClassParts(classNode)
	if got := nameText(t, tree, name); got != "C" {
		t.Fatalf("unexpected class name: got %q", got)
	}
}

func TestParseDecoratorErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{name: "missing expr", src: "@\ndef f():\n    pass\n", want: "expected decorator expression after '@'"},
		{name: "missing newline", src: "@dec def f():\n    pass\n", want: "expected newline after decorator"},
		{name: "missing def or class", src: "@dec\nx = 1\n", want: "expected function or class definition after decorator"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := parseSource(t, tt.src)
			requireParseErrorContains(t, p, tt.want)
		})
	}
}

func TestParseIfElseShape(t *testing.T) {
	p, tree := parseSource(t, "if x:\n    y\nelse:\n    z\n")
	requireNoParseErrors(t, p)

	ifNode := moduleStmt(t, tree, 0)
	requireKind(t, tree, ifNode, a.NodeIf)
	ifKids := requireChildCount(t, tree, ifNode, 3)

	if got := nameText(t, tree, ifKids[0]); got != "x" {
		t.Fatalf("unexpected if test name: got %q", got)
	}

	body := ifKids[1]
	requireKind(t, tree, body, a.NodeBlock)
	bodyStmt := requireChildCount(t, tree, body, 1)[0]
	bodyExpr := requireChildCount(t, tree, bodyStmt, 1)[0]
	if got := nameText(t, tree, bodyExpr); got != "y" {
		t.Fatalf("unexpected body expr: got %q", got)
	}

	orElse := ifKids[2]
	requireKind(t, tree, orElse, a.NodeBlock)
	orElseStmt := requireChildCount(t, tree, orElse, 1)[0]
	orElseExpr := requireChildCount(t, tree, orElseStmt, 1)[0]
	if got := nameText(t, tree, orElseExpr); got != "z" {
		t.Fatalf("unexpected else expr: got %q", got)
	}
}

func TestParseIfElifAsNestedIfBlock(t *testing.T) {
	p, tree := parseSource(t, "if x:\n    y\nelif z:\n    w\n")
	requireNoParseErrors(t, p)

	ifNode := moduleStmt(t, tree, 0)
	requireKind(t, tree, ifNode, a.NodeIf)
	ifKids := requireChildCount(t, tree, ifNode, 3)

	orElse := ifKids[2]
	requireKind(t, tree, orElse, a.NodeBlock)
	orElseKids := requireChildCount(t, tree, orElse, 1)
	requireKind(t, tree, orElseKids[0], a.NodeIf)

	nestedKids := requireChildCount(t, tree, orElseKids[0], 2)
	if got := nameText(t, tree, nestedKids[0]); got != "z" {
		t.Fatalf("unexpected elif test: got %q", got)
	}
}

func TestParseTryExceptElseFinallyShape(t *testing.T) {
	p, tree := parseSource(t, "try:\n    risky\nexcept ValueError as err:\n    handle\nelse:\n    ok\nfinally:\n    cleanup\n")
	requireNoParseErrors(t, p)

	tryNode := moduleStmt(t, tree, 0)
	requireKind(t, tree, tryNode, a.NodeTry)
	body, excepts, elseBlock, finallyBlock := tree.TryParts(tryNode)
	requireKind(t, tree, body, a.NodeBlock)
	if len(excepts) != 1 {
		t.Fatalf("unexpected except count: %d", len(excepts))
	}
	requireKind(t, tree, excepts[0], a.NodeExcept)
	excType, asName, exceptBody := tree.ExceptParts(excepts[0])
	if got := nameText(t, tree, excType); got != "ValueError" {
		t.Fatalf("unexpected except type: got %q", got)
	}
	if got := nameText(t, tree, asName); got != "err" {
		t.Fatalf("unexpected except alias: got %q", got)
	}
	requireKind(t, tree, exceptBody, a.NodeBlock)
	requireKind(t, tree, elseBlock, a.NodeBlock)
	requireKind(t, tree, finallyBlock, a.NodeBlock)
}

func TestParseTryFinallyShape(t *testing.T) {
	p, tree := parseSource(t, "try:\n    risky\nfinally:\n    cleanup\n")
	requireNoParseErrors(t, p)

	tryNode := moduleStmt(t, tree, 0)
	requireKind(t, tree, tryNode, a.NodeTry)
	body, excepts, elseBlock, finallyBlock := tree.TryParts(tryNode)
	requireKind(t, tree, body, a.NodeBlock)
	if len(excepts) != 0 {
		t.Fatalf("unexpected except count: %d", len(excepts))
	}
	if elseBlock != a.NoNode {
		t.Fatalf("unexpected else block: %v", elseBlock)
	}
	requireKind(t, tree, finallyBlock, a.NodeBlock)
}

func TestParseWithStatementShape(t *testing.T) {
	p, tree := parseSource(t, "with resource as handle:\n    handle\n")
	requireNoParseErrors(t, p)

	withNode := moduleStmt(t, tree, 0)
	requireKind(t, tree, withNode, a.NodeWith)
	items, body := tree.WithParts(withNode)
	if len(items) != 1 {
		t.Fatalf("unexpected with item count: %d", len(items))
	}
	contextExpr, asTarget := tree.WithItemParts(items[0])
	if got := nameText(t, tree, contextExpr); got != "resource" {
		t.Fatalf("unexpected with context expr: got %q", got)
	}
	if got := nameText(t, tree, asTarget); got != "handle" {
		t.Fatalf("unexpected with as target: got %q", got)
	}
	bodyStmt := requireChildCount(t, tree, body, 1)[0]
	bodyExpr := requireChildCount(t, tree, bodyStmt, 1)[0]
	if got := nameText(t, tree, bodyExpr); got != "handle" {
		t.Fatalf("unexpected with body expr: got %q", got)
	}
}

func TestParseWithStatementMultipleItems(t *testing.T) {
	p, tree := parseSource(t, "with a as x, b as y:\n    y\n")
	requireNoParseErrors(t, p)

	withNode := moduleStmt(t, tree, 0)
	requireKind(t, tree, withNode, a.NodeWith)
	items, body := tree.WithParts(withNode)
	if len(items) != 2 {
		t.Fatalf("unexpected with item count: %d", len(items))
	}
	firstContext, firstTarget := tree.WithItemParts(items[0])
	if got := nameText(t, tree, firstContext); got != "a" {
		t.Fatalf("unexpected first with context: got %q", got)
	}
	if got := nameText(t, tree, firstTarget); got != "x" {
		t.Fatalf("unexpected first with target: got %q", got)
	}
	secondContext, secondTarget := tree.WithItemParts(items[1])
	if got := nameText(t, tree, secondContext); got != "b" {
		t.Fatalf("unexpected second with context: got %q", got)
	}
	if got := nameText(t, tree, secondTarget); got != "y" {
		t.Fatalf("unexpected second with target: got %q", got)
	}
	if body == a.NoNode {
		t.Fatal("expected with body")
	}
}

func TestParseWithStatementBodyWithLeadingBlankLines(t *testing.T) {
	p, tree := parseSource(t, "with resource:\n\n\n    use\n")
	requireNoParseErrors(t, p)

	withNode := moduleStmt(t, tree, 0)
	requireKind(t, tree, withNode, a.NodeWith)
	_, body := tree.WithParts(withNode)
	bodyStmt := requireChildCount(t, tree, body, 1)[0]
	bodyExpr := requireChildCount(t, tree, bodyStmt, 1)[0]
	if got := nameText(t, tree, bodyExpr); got != "use" {
		t.Fatalf("unexpected with body expr: got %q", got)
	}
}

func TestParseWithStatementErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{name: "missing expr", src: "with:\n    x\n", want: "expected expression after 'with'"},
		{name: "missing target after as", src: "with resource as:\n    x\n", want: "expected target after 'as' in with item"},
		{name: "missing colon", src: "with resource\n    x\n", want: "expected ':' after with"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := parseSource(t, tt.src)
			requireParseErrorContains(t, p, tt.want)
		})
	}
}

func TestParsePassStatement(t *testing.T) {
	p, tree := parseSource(t, "pass\n")
	requireNoParseErrors(t, p)

	stmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, stmt, a.NodePass)
}

func TestParseIfBodyPassStatement(t *testing.T) {
	p, tree := parseSource(t, "if x:\n    pass\n")
	requireNoParseErrors(t, p)

	ifNode := moduleStmt(t, tree, 0)
	requireKind(t, tree, ifNode, a.NodeIf)
	ifKids := requireChildCount(t, tree, ifNode, 2)
	body := ifKids[1]
	requireKind(t, tree, body, a.NodeBlock)
	bodyStmt := requireChildCount(t, tree, body, 1)[0]
	requireKind(t, tree, bodyStmt, a.NodePass)
}

func TestParseIfBodyWithLeadingBlankLines(t *testing.T) {
	p, tree := parseSource(t, "if x:\n\n\n    y\n")
	requireNoParseErrors(t, p)

	ifNode := moduleStmt(t, tree, 0)
	requireKind(t, tree, ifNode, a.NodeIf)
	body := requireChildCount(t, tree, ifNode, 2)[1]
	bodyStmt := requireChildCount(t, tree, body, 1)[0]
	bodyExpr := requireChildCount(t, tree, bodyStmt, 1)[0]
	if got := nameText(t, tree, bodyExpr); got != "y" {
		t.Fatalf("unexpected if body expr: got %q", got)
	}
}

func TestParseIfElseBodyWithLeadingBlankLines(t *testing.T) {
	p, tree := parseSource(t, "if x:\n    y\nelse:\n\n\n    z\n")
	requireNoParseErrors(t, p)

	ifNode := moduleStmt(t, tree, 0)
	requireKind(t, tree, ifNode, a.NodeIf)
	orElse := requireChildCount(t, tree, ifNode, 3)[2]
	orElseStmt := requireChildCount(t, tree, orElse, 1)[0]
	orElseExpr := requireChildCount(t, tree, orElseStmt, 1)[0]
	if got := nameText(t, tree, orElseExpr); got != "z" {
		t.Fatalf("unexpected else body expr: got %q", got)
	}
}

func TestParseTryExceptPassStatement(t *testing.T) {
	p, tree := parseSource(t, "try:\n    risky\nexcept TypeError:\n    pass\n")
	requireNoParseErrors(t, p)

	tryNode := moduleStmt(t, tree, 0)
	requireKind(t, tree, tryNode, a.NodeTry)
	_, excepts, _, _ := tree.TryParts(tryNode)
	if len(excepts) != 1 {
		t.Fatalf("unexpected except count: %d", len(excepts))
	}
	_, _, body := tree.ExceptParts(excepts[0])
	requireKind(t, tree, body, a.NodeBlock)
	bodyStmt := requireChildCount(t, tree, body, 1)[0]
	requireKind(t, tree, bodyStmt, a.NodePass)
}

func TestParseTryExceptFinallyBodiesWithLeadingBlankLines(t *testing.T) {
	p, tree := parseSource(t, "try:\n\n\n    risky\nexcept TypeError:\n\n\n    handle\nfinally:\n\n\n    cleanup\n")
	requireNoParseErrors(t, p)

	tryNode := moduleStmt(t, tree, 0)
	requireKind(t, tree, tryNode, a.NodeTry)
	body, excepts, _, finallyBlock := tree.TryParts(tryNode)
	bodyStmt := requireChildCount(t, tree, body, 1)[0]
	bodyExpr := requireChildCount(t, tree, bodyStmt, 1)[0]
	if got := nameText(t, tree, bodyExpr); got != "risky" {
		t.Fatalf("unexpected try body expr: got %q", got)
	}
	_, _, exceptBody := tree.ExceptParts(excepts[0])
	exceptStmt := requireChildCount(t, tree, exceptBody, 1)[0]
	exceptExpr := requireChildCount(t, tree, exceptStmt, 1)[0]
	if got := nameText(t, tree, exceptExpr); got != "handle" {
		t.Fatalf("unexpected except body expr: got %q", got)
	}
	finallyStmt := requireChildCount(t, tree, finallyBlock, 1)[0]
	finallyExpr := requireChildCount(t, tree, finallyStmt, 1)[0]
	if got := nameText(t, tree, finallyExpr); got != "cleanup" {
		t.Fatalf("unexpected finally body expr: got %q", got)
	}
}

func TestParseForBodyWithLeadingBlankLines(t *testing.T) {
	p, tree := parseSource(t, "for x in xs:\n\n\n    y\n")
	requireNoParseErrors(t, p)

	forNode := moduleStmt(t, tree, 0)
	requireKind(t, tree, forNode, a.NodeFor)
	forKids := requireChildCount(t, tree, forNode, 3)
	bodyStmt := requireChildCount(t, tree, forKids[2], 1)[0]
	bodyExpr := requireChildCount(t, tree, bodyStmt, 1)[0]
	if got := nameText(t, tree, bodyExpr); got != "y" {
		t.Fatalf("unexpected for body expr: got %q", got)
	}
}

func TestParseForElseBodyWithLeadingBlankLines(t *testing.T) {
	p, tree := parseSource(t, "for x in xs:\n    y\nelse:\n\n\n    z\n")
	requireNoParseErrors(t, p)

	forNode := moduleStmt(t, tree, 0)
	requireKind(t, tree, forNode, a.NodeFor)
	forKids := requireChildCount(t, tree, forNode, 4)
	elseStmt := requireChildCount(t, tree, forKids[3], 1)[0]
	elseExpr := requireChildCount(t, tree, elseStmt, 1)[0]
	if got := nameText(t, tree, elseExpr); got != "z" {
		t.Fatalf("unexpected for else body expr: got %q", got)
	}
}

func TestParseWhileBodyWithLeadingBlankLines(t *testing.T) {
	p, tree := parseSource(t, "while x:\n\n\n    y\n")
	requireNoParseErrors(t, p)

	whileNode := moduleStmt(t, tree, 0)
	requireKind(t, tree, whileNode, a.NodeWhile)
	whileKids := requireChildCount(t, tree, whileNode, 2)
	bodyStmt := requireChildCount(t, tree, whileKids[1], 1)[0]
	bodyExpr := requireChildCount(t, tree, bodyStmt, 1)[0]
	if got := nameText(t, tree, bodyExpr); got != "y" {
		t.Fatalf("unexpected while body expr: got %q", got)
	}
}

func TestParseListComprehensionShape(t *testing.T) {
	p, tree := parseSource(t, "[x for x in xs if x]\n")
	requireNoParseErrors(t, p)

	stmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, stmt, a.NodeExprStmt)
	comp := requireChildCount(t, tree, stmt, 1)[0]
	requireKind(t, tree, comp, a.NodeListComp)
	expr, clauses := tree.ListCompParts(comp)
	if got := nameText(t, tree, expr); got != "x" {
		t.Fatalf("unexpected list comp expr: got %q", got)
	}
	if len(clauses) != 1 {
		t.Fatalf("unexpected clause count: %d", len(clauses))
	}
	target, iter, filters := tree.ComprehensionParts(clauses[0])
	if got := nameText(t, tree, target); got != "x" {
		t.Fatalf("unexpected target: got %q", got)
	}
	if got := nameText(t, tree, iter); got != "xs" {
		t.Fatalf("unexpected iter: got %q", got)
	}
	if len(filters) != 1 || nameText(t, tree, filters[0]) != "x" {
		t.Fatalf("unexpected filters: %+v", filters)
	}
}

func TestParseListComprehensionMultipleClauses(t *testing.T) {
	p, tree := parseSource(t, "[(x, y) for x in xs for y in ys]\n")
	requireNoParseErrors(t, p)

	stmt := moduleStmt(t, tree, 0)
	comp := requireChildCount(t, tree, stmt, 1)[0]
	requireKind(t, tree, comp, a.NodeListComp)
	_, clauses := tree.ListCompParts(comp)
	if len(clauses) != 2 {
		t.Fatalf("unexpected clause count: %d", len(clauses))
	}
}

func TestParseClassBasesAndDocstring(t *testing.T) {
	p, tree := parseSource(t, "class C(A, B):\n    \"doc\"\n    x\n")
	requireNoParseErrors(t, p)

	classNode := moduleStmt(t, tree, 0)
	requireKind(t, tree, classNode, a.NodeClassDef)
	classKids := requireChildCount(t, tree, classNode, 3)

	if got := nameText(t, tree, classKids[0]); got != "C" {
		t.Fatalf("unexpected class name: got %q", got)
	}

	bases := classKids[1]
	requireKind(t, tree, bases, a.NodeBaseList)
	baseKids := requireChildCount(t, tree, bases, 2)
	if got := nameText(t, tree, baseKids[0]); got != "A" {
		t.Fatalf("unexpected first base: got %q", got)
	}
	if got := nameText(t, tree, baseKids[1]); got != "B" {
		t.Fatalf("unexpected second base: got %q", got)
	}

	if got := tree.Strings[tree.Nodes[classNode].Data]; got != "doc" {
		t.Fatalf("unexpected class docstring: got %q", got)
	}

	body := classKids[2]
	requireKind(t, tree, body, a.NodeBlock)
	bodyStmt := requireChildCount(t, tree, body, 1)[0]
	bodyExpr := requireChildCount(t, tree, bodyStmt, 1)[0]
	if got := nameText(t, tree, bodyExpr); got != "x" {
		t.Fatalf("unexpected class body expr: got %q", got)
	}
}

func TestParseAssignShape(t *testing.T) {
	p, tree := parseSource(t, "x, y = z\n")
	requireNoParseErrors(t, p)

	assign := moduleStmt(t, tree, 0)
	requireKind(t, tree, assign, a.NodeAssign)
	assignKids := requireChildCount(t, tree, assign, 3)

	if got := nameText(t, tree, assignKids[0]); got != "z" {
		t.Fatalf("unexpected assign value: got %q", got)
	}
	if got := nameText(t, tree, assignKids[1]); got != "x" {
		t.Fatalf("unexpected first assign target: got %q", got)
	}
	if got := nameText(t, tree, assignKids[2]); got != "y" {
		t.Fatalf("unexpected second assign target: got %q", got)
	}
}

func TestParseSubscriptAssignmentShape(t *testing.T) {
	p, tree := parseSource(t, "a[0] = x\n")
	requireNoParseErrors(t, p)

	assign := moduleStmt(t, tree, 0)
	requireKind(t, tree, assign, a.NodeAssign)
	assignKids := requireChildCount(t, tree, assign, 2)
	if got := nameText(t, tree, assignKids[0]); got != "x" {
		t.Fatalf("unexpected assign value: got %q", got)
	}
	requireKind(t, tree, assignKids[1], a.NodeSubScript)
	targetKids := requireChildCount(t, tree, assignKids[1], 2)
	if got := nameText(t, tree, targetKids[0]); got != "a" {
		t.Fatalf("unexpected subscript target base: got %q", got)
	}
	if got := numberValue(t, tree, targetKids[1]); got != "0" {
		t.Fatalf("unexpected subscript target index: got %q", got)
	}
}

func TestParseSliceAssignmentShape(t *testing.T) {
	p, tree := parseSource(t, "a[1:3] = x\n")
	requireNoParseErrors(t, p)

	assign := moduleStmt(t, tree, 0)
	requireKind(t, tree, assign, a.NodeAssign)
	assignKids := requireChildCount(t, tree, assign, 2)
	requireKind(t, tree, assignKids[1], a.NodeSubScript)
	targetKids := requireChildCount(t, tree, assignKids[1], 2)
	requireKind(t, tree, targetKids[1], a.NodeSlice)
}

func TestParseAnnotatedAssignmentShape(t *testing.T) {
	p, tree := parseSource(t, "x: int = 1\n")
	requireNoParseErrors(t, p)

	stmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, stmt, a.NodeAnnAssign)
	target, annotation, value := tree.AnnAssignParts(stmt)
	if got := nameText(t, tree, target); got != "x" {
		t.Fatalf("unexpected annotated assign target: got %q", got)
	}
	if got := nameText(t, tree, annotation); got != "int" {
		t.Fatalf("unexpected annotation: got %q", got)
	}
	if got := numberValue(t, tree, value); got != "1" {
		t.Fatalf("unexpected annotated assign value: got %q", got)
	}
}

func TestParseAnnotatedAssignmentWithoutValue(t *testing.T) {
	p, tree := parseSource(t, "x: int\n")
	requireNoParseErrors(t, p)

	stmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, stmt, a.NodeAnnAssign)
	target, annotation, value := tree.AnnAssignParts(stmt)
	if got := nameText(t, tree, target); got != "x" {
		t.Fatalf("unexpected annotated assign target: got %q", got)
	}
	if got := nameText(t, tree, annotation); got != "int" {
		t.Fatalf("unexpected annotation: got %q", got)
	}
	if value != a.NoNode {
		t.Fatalf("unexpected annotated assign value: got %s", tree.Node(value).Kind)
	}
}

func TestParseAnnotatedAssignmentUnionShape(t *testing.T) {
	p, tree := parseSource(t, "x: int | None = 1\n")
	requireNoParseErrors(t, p)

	stmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, stmt, a.NodeAnnAssign)
	target, annotation, value := tree.AnnAssignParts(stmt)
	if got := nameText(t, tree, target); got != "x" {
		t.Fatalf("unexpected annotated assign target: got %q", got)
	}
	requireKind(t, tree, annotation, a.NodeBinOp)
	parts := requireChildCount(t, tree, annotation, 2)
	if got := nameText(t, tree, parts[0]); got != "int" {
		t.Fatalf("unexpected left annotation: got %q", got)
	}
	requireKind(t, tree, parts[1], a.NodeNone)
	if got := numberValue(t, tree, value); got != "1" {
		t.Fatalf("unexpected annotated assign value: got %q", got)
	}
}

func TestParseSubscriptAugAssignShape(t *testing.T) {
	p, tree := parseSource(t, "a[0] += 1\n")
	requireNoParseErrors(t, p)

	assign := moduleStmt(t, tree, 0)
	requireKind(t, tree, assign, a.NodeAugAssign)
	kids := requireChildCount(t, tree, assign, 2)
	requireKind(t, tree, kids[0], a.NodeSubScript)
	if got := numberValue(t, tree, kids[1]); got != "1" {
		t.Fatalf("unexpected aug assign value: got %q", got)
	}
}

func TestParseCallShape(t *testing.T) {
	p, tree := parseSource(t, "f(x, y)\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, exprStmt, a.NodeExprStmt)
	stmtKids := requireChildCount(t, tree, exprStmt, 1)

	call := stmtKids[0]
	requireKind(t, tree, call, a.NodeCall)
	callKids := requireChildCount(t, tree, call, 3)
	if got := nameText(t, tree, callKids[0]); got != "f" {
		t.Fatalf("unexpected callee: got %q", got)
	}
	if got := nameText(t, tree, callKids[1]); got != "x" {
		t.Fatalf("unexpected first arg: got %q", got)
	}
	if got := nameText(t, tree, callKids[2]); got != "y" {
		t.Fatalf("unexpected second arg: got %q", got)
	}
}

func TestParseCallKeywordArgument(t *testing.T) {
	p, tree := parseSource(t, "foo(x=myList[3])\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	call := requireChildCount(t, tree, exprStmt, 1)[0]
	requireKind(t, tree, call, a.NodeCall)
	callKids := requireChildCount(t, tree, call, 2)
	requireKind(t, tree, callKids[1], a.NodeKeywordArg)
	kwKids := requireChildCount(t, tree, callKids[1], 2)
	if got := nameText(t, tree, kwKids[0]); got != "x" {
		t.Fatalf("unexpected keyword name: got %q", got)
	}
	requireKind(t, tree, kwKids[1], a.NodeSubScript)
	subKids := requireChildCount(t, tree, kwKids[1], 2)
	if got := nameText(t, tree, subKids[0]); got != "myList" {
		t.Fatalf("unexpected keyword value base: got %q", got)
	}
	if got := numberValue(t, tree, subKids[1]); got != "3" {
		t.Fatalf("unexpected keyword value index: got %q", got)
	}
}

func TestParseCallMixedPositionalAndKeywordArgs(t *testing.T) {
	p, tree := parseSource(t, "foo(a, x=1)\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	call := requireChildCount(t, tree, exprStmt, 1)[0]
	requireKind(t, tree, call, a.NodeCall)
	callKids := requireChildCount(t, tree, call, 3)
	if got := nameText(t, tree, callKids[1]); got != "a" {
		t.Fatalf("unexpected positional arg: got %q", got)
	}
	requireKind(t, tree, callKids[2], a.NodeKeywordArg)
}

func TestParseCallKeywordArgErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{name: "missing keyword value", src: "foo(x=)\n", want: "expected expression after '=' in keyword argument"},
		{name: "positional after keyword", src: "foo(x=1, y)\n", want: "positional argument follows keyword argument"},
		{name: "missing star expr", src: "foo(*)\n", want: "expected expression after '*' in call argument"},
		{name: "missing kwstar expr", src: "foo(**)\n", want: "expected expression after '**' in call argument"},
		{name: "positional after kwstar", src: "foo(**kw, x)\n", want: "positional argument follows **kwargs"},
		{name: "keyword after kwstar", src: "foo(**kw, x=1)\n", want: "keyword argument follows **kwargs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := parseSource(t, tt.src)
			requireParseErrorContains(t, p, tt.want)
		})
	}
}

func TestParseCallStarAndKwStarArguments(t *testing.T) {
	p, tree := parseSource(t, "foo(a, *xs, y=1, **kw)\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	call := requireChildCount(t, tree, exprStmt, 1)[0]
	callKids := requireChildCount(t, tree, call, 5)
	if got := nameText(t, tree, callKids[1]); got != "a" {
		t.Fatalf("unexpected positional arg: got %q", got)
	}
	requireKind(t, tree, callKids[2], a.NodeStarArg)
	if got := nameText(t, tree, tree.ChildAt(callKids[2], 0)); got != "xs" {
		t.Fatalf("unexpected star arg expr: got %q", got)
	}
	requireKind(t, tree, callKids[3], a.NodeKeywordArg)
	requireKind(t, tree, callKids[4], a.NodeKwStarArg)
	if got := nameText(t, tree, tree.ChildAt(callKids[4], 0)); got != "kw" {
		t.Fatalf("unexpected kwstar arg expr: got %q", got)
	}
}

func TestParseComparisonOperators(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		wantOp    a.CompareOp
		wantLeft  string
		wantRight string
	}{
		{name: "in", src: "x in y\n", wantOp: a.In, wantLeft: "x", wantRight: "y"},
		{name: "not in", src: "x not in y\n", wantOp: a.NotIn, wantLeft: "x", wantRight: "y"},
		{name: "is", src: "x is y\n", wantOp: a.Is, wantLeft: "x", wantRight: "y"},
		{name: "is not", src: "x is not y\n", wantOp: a.IsNot, wantLeft: "x", wantRight: "y"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, tree := parseSource(t, tt.src)
			requireNoParseErrors(t, p)

			exprStmt := moduleStmt(t, tree, 0)
			compare := requireChildCount(t, tree, exprStmt, 1)[0]
			requireKind(t, tree, compare, a.NodeCompare)

			kids := requireChildCount(t, tree, compare, 2)
			if got := nameText(t, tree, kids[0]); got != tt.wantLeft {
				t.Fatalf("unexpected comparison left: got %q", got)
			}
			if got := compareOpValue(t, tree, kids[1]); got != tt.wantOp {
				t.Fatalf("unexpected comparison op: got %v want %v", got, tt.wantOp)
			}
			if got := nameText(t, tree, tree.ChildAt(kids[1], 0)); got != tt.wantRight {
				t.Fatalf("unexpected comparison right: got %q", got)
			}
		})
	}
}

func TestParseComparisonChainingWithIdentityAndMembership(t *testing.T) {
	p, tree := parseSource(t, "a is not b in c\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	compare := requireChildCount(t, tree, exprStmt, 1)[0]
	requireKind(t, tree, compare, a.NodeCompare)

	kids := requireChildCount(t, tree, compare, 3)
	if got := nameText(t, tree, kids[0]); got != "a" {
		t.Fatalf("unexpected chained comparison left: got %q", got)
	}
	if got := compareOpValue(t, tree, kids[1]); got != a.IsNot {
		t.Fatalf("unexpected first comparison op: got %v", got)
	}
	if got := nameText(t, tree, tree.ChildAt(kids[1], 0)); got != "b" {
		t.Fatalf("unexpected first comparison right: got %q", got)
	}
	if got := compareOpValue(t, tree, kids[2]); got != a.In {
		t.Fatalf("unexpected second comparison op: got %v", got)
	}
	if got := nameText(t, tree, tree.ChildAt(kids[2], 0)); got != "c" {
		t.Fatalf("unexpected second comparison right: got %q", got)
	}
}

func TestParseComparisonOperatorErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{name: "missing rhs after is", src: "x is\n", want: "expected expression after comparison operator"},
		{name: "missing rhs after is not", src: "x is not\n", want: "expected expression after comparison operator"},
		{name: "not without in is not a comparison", src: "x not y\n", want: "expected newline after expression"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := parseSource(t, tt.src)
			requireParseErrorContains(t, p, tt.want)
		})
	}
}

func TestInvariantFunctionDefChildOrder(t *testing.T) {
	p, tree := parseSource(t, "def f(a=1):\n    x\n")
	requireNoParseErrors(t, p)

	fn := moduleStmt(t, tree, 0)
	requireKind(t, tree, fn, a.NodeFunctionDef)
	fnKids := requireChildCount(t, tree, fn, 3)

	requireKind(t, tree, fnKids[0], a.NodeName)
	requireKind(t, tree, fnKids[1], a.NodeArgs)
	requireKind(t, tree, fnKids[2], a.NodeBlock)

	params := requireChildCount(t, tree, fnKids[1], 1)
	paramKids := requireChildCount(t, tree, params[0], 2)
	requireKind(t, tree, paramKids[0], a.NodeName)
	requireKind(t, tree, paramKids[1], a.NodeNumber)
}

func TestParseAttributeShape(t *testing.T) {
	p, tree := parseSource(t, "a.b\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, exprStmt, a.NodeExprStmt)
	attr := requireChildCount(t, tree, exprStmt, 1)[0]
	requireKind(t, tree, attr, a.NodeAttribute)
	attrKids := requireChildCount(t, tree, attr, 2)
	if got := nameText(t, tree, attrKids[0]); got != "a" {
		t.Fatalf("unexpected attribute base: got %q", got)
	}
	if got := nameText(t, tree, attrKids[1]); got != "b" {
		t.Fatalf("unexpected attribute name: got %q", got)
	}
}

func TestParseImportSimple(t *testing.T) {
	p, tree := parseSource(t, "import pkg\n")
	requireNoParseErrors(t, p)

	stmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, stmt, a.NodeImport)
	aliases := requireChildCount(t, tree, stmt, 1)
	requireKind(t, tree, aliases[0], a.NodeAlias)
	target, alias := tree.AliasParts(aliases[0])
	if got := nameText(t, tree, target); got != "pkg" {
		t.Fatalf("unexpected import target: got %q", got)
	}
	if alias != a.NoNode {
		t.Fatal("did not expect alias for simple import")
	}
}

func TestParseImportDottedAndAlias(t *testing.T) {
	p, tree := parseSource(t, "import pkg.mod as m\n")
	requireNoParseErrors(t, p)

	stmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, stmt, a.NodeImport)
	aliasNode := requireChildCount(t, tree, stmt, 1)[0]
	target, alias := tree.AliasParts(aliasNode)
	requireKind(t, tree, target, a.NodeAttribute)
	targetKids := requireChildCount(t, tree, target, 2)
	if got := nameText(t, tree, targetKids[0]); got != "pkg" {
		t.Fatalf("unexpected import base: got %q", got)
	}
	if got := nameText(t, tree, targetKids[1]); got != "mod" {
		t.Fatalf("unexpected import attr: got %q", got)
	}
	if got := nameText(t, tree, alias); got != "m" {
		t.Fatalf("unexpected alias: got %q", got)
	}
}

func TestParseImportMultiple(t *testing.T) {
	p, tree := parseSource(t, "import a, b as c\n")
	requireNoParseErrors(t, p)

	stmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, stmt, a.NodeImport)
	aliases := requireChildCount(t, tree, stmt, 2)

	firstTarget, firstAlias := tree.AliasParts(aliases[0])
	if got := nameText(t, tree, firstTarget); got != "a" {
		t.Fatalf("unexpected first target: got %q", got)
	}
	if firstAlias != a.NoNode {
		t.Fatal("did not expect alias for first import")
	}

	secondTarget, secondAlias := tree.AliasParts(aliases[1])
	if got := nameText(t, tree, secondTarget); got != "b" {
		t.Fatalf("unexpected second target: got %q", got)
	}
	if got := nameText(t, tree, secondAlias); got != "c" {
		t.Fatalf("unexpected second alias: got %q", got)
	}
}

func TestParseFromImportSimple(t *testing.T) {
	p, tree := parseSource(t, "from pkg import name\n")
	requireNoParseErrors(t, p)

	stmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, stmt, a.NodeFromImport)
	parts := children(tree, stmt)
	if got := nameText(t, tree, parts[0]); got != "pkg" {
		t.Fatalf("unexpected module path: got %q", got)
	}
	requireKind(t, tree, parts[1], a.NodeAlias)
	target, alias := tree.AliasParts(parts[1])
	if got := nameText(t, tree, target); got != "name" {
		t.Fatalf("unexpected imported name: got %q", got)
	}
	if alias != a.NoNode {
		t.Fatal("did not expect alias for simple from-import")
	}
}

func TestParseFromImportDottedModuleAndAlias(t *testing.T) {
	p, tree := parseSource(t, "from pkg.mod import name as alias\n")
	requireNoParseErrors(t, p)

	stmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, stmt, a.NodeFromImport)
	parts := children(tree, stmt)
	requireKind(t, tree, parts[0], a.NodeAttribute)
	moduleKids := requireChildCount(t, tree, parts[0], 2)
	if got := nameText(t, tree, moduleKids[0]); got != "pkg" {
		t.Fatalf("unexpected module base: got %q", got)
	}
	if got := nameText(t, tree, moduleKids[1]); got != "mod" {
		t.Fatalf("unexpected module attr: got %q", got)
	}
	target, alias := tree.AliasParts(parts[1])
	if got := nameText(t, tree, target); got != "name" {
		t.Fatalf("unexpected imported target: got %q", got)
	}
	if got := nameText(t, tree, alias); got != "alias" {
		t.Fatalf("unexpected imported alias: got %q", got)
	}
}

func TestParseFromImportMultiple(t *testing.T) {
	p, tree := parseSource(t, "from pkg import x, y as z\n")
	requireNoParseErrors(t, p)

	stmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, stmt, a.NodeFromImport)
	parts := children(tree, stmt)
	if len(parts) != 3 {
		t.Fatalf("unexpected child count: got %d want 3", len(parts))
	}
	firstTarget, firstAlias := tree.AliasParts(parts[1])
	if got := nameText(t, tree, firstTarget); got != "x" {
		t.Fatalf("unexpected first imported target: got %q", got)
	}
	if firstAlias != a.NoNode {
		t.Fatal("did not expect alias for first imported name")
	}
	secondTarget, secondAlias := tree.AliasParts(parts[2])
	if got := nameText(t, tree, secondTarget); got != "y" {
		t.Fatalf("unexpected second imported target: got %q", got)
	}
	if got := nameText(t, tree, secondAlias); got != "z" {
		t.Fatalf("unexpected second imported alias: got %q", got)
	}
}

func TestParseFromImportStar(t *testing.T) {
	p, tree := parseSource(t, "from pkg import *\n")
	requireNoParseErrors(t, p)

	stmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, stmt, a.NodeFromImport)
	parts := children(tree, stmt)
	if len(parts) != 2 {
		t.Fatalf("unexpected child count: got %d want 2", len(parts))
	}
	if got := nameText(t, tree, parts[0]); got != "pkg" {
		t.Fatalf("unexpected module path: got %q", got)
	}
	target, alias := tree.AliasParts(parts[1])
	if got := nameText(t, tree, target); got != "*" {
		t.Fatalf("unexpected star import target: got %q", got)
	}
	if alias != a.NoNode {
		t.Fatal("did not expect alias for star import")
	}
}

func TestParseFromImportParenthesized(t *testing.T) {
	p, tree := parseSource(t, "from pkg import (x, y as z)\n")
	requireNoParseErrors(t, p)

	stmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, stmt, a.NodeFromImport)
	parts := children(tree, stmt)
	if len(parts) != 3 {
		t.Fatalf("unexpected child count: got %d want 3", len(parts))
	}
	if got := nameText(t, tree, parts[0]); got != "pkg" {
		t.Fatalf("unexpected module path: got %q", got)
	}
	firstTarget, firstAlias := tree.AliasParts(parts[1])
	if got := nameText(t, tree, firstTarget); got != "x" {
		t.Fatalf("unexpected first imported target: got %q", got)
	}
	if firstAlias != a.NoNode {
		t.Fatal("did not expect alias for first imported name")
	}
	secondTarget, secondAlias := tree.AliasParts(parts[2])
	if got := nameText(t, tree, secondTarget); got != "y" {
		t.Fatalf("unexpected second imported target: got %q", got)
	}
	if got := nameText(t, tree, secondAlias); got != "z" {
		t.Fatalf("unexpected second imported alias: got %q", got)
	}
}

func TestParseFromImportParenthesizedMultiline(t *testing.T) {
	p, tree := parseSource(t, "from urllib3.exceptions import (\n    ClosedPoolError,\n    ConnectTimeoutError,\n    MaxRetryError,\n)\n")
	requireNoParseErrors(t, p)

	stmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, stmt, a.NodeFromImport)
	module, aliases := tree.FromImportParts(stmt)
	requireKind(t, tree, module, a.NodeAttribute)
	if len(aliases) != 3 {
		t.Fatalf("unexpected alias count: got %d want 3", len(aliases))
	}
	if got := nameText(t, tree, tree.ChildAt(aliases[0], 0)); got != "ClosedPoolError" {
		t.Fatalf("unexpected first imported target: got %q", got)
	}
	if got := nameText(t, tree, tree.ChildAt(aliases[1], 0)); got != "ConnectTimeoutError" {
		t.Fatalf("unexpected second imported target: got %q", got)
	}
	if got := nameText(t, tree, tree.ChildAt(aliases[2], 0)); got != "MaxRetryError" {
		t.Fatalf("unexpected third imported target: got %q", got)
	}
}

func TestParseRelativeFromImportForms(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		wantDepth uint32
		wantMod   string
	}{
		{name: "dot import", src: "from . import x\n", wantDepth: 1, wantMod: ""},
		{name: "dot module import", src: "from .mod import x\n", wantDepth: 1, wantMod: "mod"},
		{name: "parent module import", src: "from ..pkg import x\n", wantDepth: 2, wantMod: "pkg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, tree := parseSource(t, tt.src)
			requireNoParseErrors(t, p)
			stmt := moduleStmt(t, tree, 0)
			requireKind(t, tree, stmt, a.NodeFromImport)
			if got := tree.Nodes[stmt].Data; got != tt.wantDepth {
				t.Fatalf("unexpected relative depth: got %d want %d", got, tt.wantDepth)
			}
			module, aliases := tree.FromImportParts(stmt)
			if tt.wantMod == "" {
				if module != a.NoNode {
					t.Fatalf("expected no module node, got %s", tree.Nodes[module].Kind)
				}
			} else {
				if got, ok := tree.NameText(module); !ok || got != tt.wantMod {
					t.Fatalf("unexpected relative module: got %q want %q", got, tt.wantMod)
				}
			}
			if len(aliases) != 1 {
				t.Fatalf("unexpected alias count: got %d", len(aliases))
			}
		})
	}
}

func TestParseImportErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{name: "missing import target", src: "import\n", want: "expected import path"},
		{name: "missing alias name", src: "import pkg as\n", want: "expected alias name after 'as'"},
		{name: "missing import keyword", src: "from pkg\n", want: "expected 'import' after module path"},
		{name: "missing imported name", src: "from pkg import\n", want: "expected imported name"},
		{name: "missing imported name in parens", src: "from pkg import ()\n", want: "expected imported name"},
		{name: "missing closing paren", src: "from pkg import (x, y\n", want: "expected ')' after imported names"},
		{name: "missing imported alias name", src: "from pkg import x as\n", want: "expected alias name after 'as'"},
		{name: "invalid positional-only separator", src: "def f(/):\n    pass\n", want: "invalid positional-only parameter separator"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := parseSource(t, tt.src)
			requireParseErrorContains(t, p, tt.want)
		})
	}
}

func TestParseSubscriptShape(t *testing.T) {
	p, tree := parseSource(t, "a[0]\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, exprStmt, a.NodeExprStmt)
	sub := requireChildCount(t, tree, exprStmt, 1)[0]
	requireKind(t, tree, sub, a.NodeSubScript)
	subKids := requireChildCount(t, tree, sub, 2)
	if got := nameText(t, tree, subKids[0]); got != "a" {
		t.Fatalf("unexpected subscript base: got %q", got)
	}
	if got := numberValue(t, tree, subKids[1]); got != "0" {
		t.Fatalf("unexpected subscript index: got %q", got)
	}
}

func TestParseSubscriptTupleIndexShape(t *testing.T) {
	p, tree := parseSource(t, "a[x, y]\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, exprStmt, a.NodeExprStmt)
	sub := requireChildCount(t, tree, exprStmt, 1)[0]
	requireKind(t, tree, sub, a.NodeSubScript)
	subKids := requireChildCount(t, tree, sub, 2)
	requireKind(t, tree, subKids[1], a.NodeTuple)
	tupleKids := requireChildCount(t, tree, subKids[1], 2)
	if got := nameText(t, tree, tupleKids[0]); got != "x" {
		t.Fatalf("unexpected first tuple index: got %q", got)
	}
	if got := nameText(t, tree, tupleKids[1]); got != "y" {
		t.Fatalf("unexpected second tuple index: got %q", got)
	}
}

func TestParseCallArgumentSubscript(t *testing.T) {
	p, tree := parseSource(t, "Foo(bar[0])\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, exprStmt, a.NodeExprStmt)
	call := requireChildCount(t, tree, exprStmt, 1)[0]
	requireKind(t, tree, call, a.NodeCall)
	callKids := requireChildCount(t, tree, call, 2)
	if got := nameText(t, tree, callKids[0]); got != "Foo" {
		t.Fatalf("unexpected callee: got %q", got)
	}
	requireKind(t, tree, callKids[1], a.NodeSubScript)
	argKids := requireChildCount(t, tree, callKids[1], 2)
	if got := nameText(t, tree, argKids[0]); got != "bar" {
		t.Fatalf("unexpected arg base: got %q", got)
	}
	if got := numberValue(t, tree, argKids[1]); got != "0" {
		t.Fatalf("unexpected arg index: got %q", got)
	}
}

func TestParseChainedSubscriptAttributeCall(t *testing.T) {
	p, tree := parseSource(t, "foo.bar[0](x)\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, exprStmt, a.NodeExprStmt)
	call := requireChildCount(t, tree, exprStmt, 1)[0]
	requireKind(t, tree, call, a.NodeCall)
	callKids := requireChildCount(t, tree, call, 2)
	requireKind(t, tree, callKids[0], a.NodeSubScript)
	subKids := requireChildCount(t, tree, callKids[0], 2)
	requireKind(t, tree, subKids[0], a.NodeAttribute)
	attrKids := requireChildCount(t, tree, subKids[0], 2)
	if got := nameText(t, tree, attrKids[0]); got != "foo" {
		t.Fatalf("unexpected attr base: got %q", got)
	}
	if got := nameText(t, tree, attrKids[1]); got != "bar" {
		t.Fatalf("unexpected attr name: got %q", got)
	}
	if got := numberValue(t, tree, subKids[1]); got != "0" {
		t.Fatalf("unexpected chained subscript index: got %q", got)
	}
	if got := nameText(t, tree, callKids[1]); got != "x" {
		t.Fatalf("unexpected call arg: got %q", got)
	}
}

func TestParseSubscriptErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{name: "missing subscript expr", src: "a[]\n", want: "expected expression in subscript"},
		{name: "missing closing bracket", src: "a[0\n", want: "expected ']' after subscript"},
		{name: "newline after open bracket", src: "a[\n", want: "expected expression in subscript"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := parseSource(t, tt.src)
			requireParseErrorContains(t, p, tt.want)
		})
	}
}

func TestParseEmptyDict(t *testing.T) {
	p, tree := parseSource(t, "{}\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, exprStmt, a.NodeExprStmt)
	dict := requireChildCount(t, tree, exprStmt, 1)[0]
	requireKind(t, tree, dict, a.NodeDict)
	requireChildCount(t, tree, dict, 0)
}

func TestParseDictLiteralShape(t *testing.T) {
	p, tree := parseSource(t, "{\"name\": base, \"root\": sq(16)}\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	dict := requireChildCount(t, tree, exprStmt, 1)[0]
	requireKind(t, tree, dict, a.NodeDict)
	kids := requireChildCount(t, tree, dict, 4)
	if got := stringValue(t, tree, kids[0]); got != "name" {
		t.Fatalf("unexpected first key: got %q", got)
	}
	if got := nameText(t, tree, kids[1]); got != "base" {
		t.Fatalf("unexpected first value: got %q", got)
	}
	if got := stringValue(t, tree, kids[2]); got != "root" {
		t.Fatalf("unexpected second key: got %q", got)
	}
	requireKind(t, tree, kids[3], a.NodeCall)
}

func TestParseDictLiteralTrailingComma(t *testing.T) {
	p, tree := parseSource(t, "{\"a\": 1,}\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	dict := requireChildCount(t, tree, exprStmt, 1)[0]
	requireKind(t, tree, dict, a.NodeDict)
	requireChildCount(t, tree, dict, 2)
}

func TestParseDictLiteralErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{name: "missing colon", src: "{\"a\"}\n", want: "expected ':' after dict key"},
		{name: "missing value", src: "{\"a\":}\n", want: "expected expression for dict value"},
		{name: "missing close", src: "{\"a\": 1\n", want: "expected '}' after dict literal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := parseSource(t, tt.src)
			requireParseErrorContains(t, p, tt.want)
		})
	}
}

func TestParseReturnBareTupleValues(t *testing.T) {
	p, tree := parseSource(t, "def f():\n    return a, b\n")
	requireNoParseErrors(t, p)

	fn := moduleStmt(t, tree, 0)
	requireKind(t, tree, fn, a.NodeFunctionDef)
	_, _, body := tree.FunctionParts(fn)
	ret := requireChildCount(t, tree, body, 1)[0]
	requireKind(t, tree, ret, a.NodeReturn)
	tuple := requireChildCount(t, tree, ret, 1)[0]
	requireKind(t, tree, tuple, a.NodeTuple)
	tupleKids := requireChildCount(t, tree, tuple, 2)
	if got := nameText(t, tree, tupleKids[0]); got != "a" {
		t.Fatalf("unexpected first return tuple item: got %q", got)
	}
	if got := nameText(t, tree, tupleKids[1]); got != "b" {
		t.Fatalf("unexpected second return tuple item: got %q", got)
	}
}

func TestParseReturnBareTupleTrailingComma(t *testing.T) {
	p, tree := parseSource(t, "def f():\n    return a,\n")
	requireNoParseErrors(t, p)

	fn := moduleStmt(t, tree, 0)
	_, _, body := tree.FunctionParts(fn)
	ret := requireChildCount(t, tree, body, 1)[0]
	tuple := requireChildCount(t, tree, ret, 1)[0]
	requireKind(t, tree, tuple, a.NodeTuple)
	tupleKids := requireChildCount(t, tree, tuple, 1)
	if got := nameText(t, tree, tupleKids[0]); got != "a" {
		t.Fatalf("unexpected single return tuple item: got %q", got)
	}
}

func TestParseReturnParenthesizedTupleRegression(t *testing.T) {
	p, tree := parseSource(t, "def f():\n    return (a, b)\n")
	requireNoParseErrors(t, p)

	fn := moduleStmt(t, tree, 0)
	_, _, body := tree.FunctionParts(fn)
	ret := requireChildCount(t, tree, body, 1)[0]
	tuple := requireChildCount(t, tree, ret, 1)[0]
	requireKind(t, tree, tuple, a.NodeTuple)
	requireChildCount(t, tree, tuple, 2)
}

func TestParseRaiseBare(t *testing.T) {
	p, tree := parseSource(t, "raise\n")
	requireNoParseErrors(t, p)

	stmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, stmt, a.NodeRaise)
	exc, cause := tree.RaiseParts(stmt)
	if exc != a.NoNode || cause != a.NoNode {
		t.Fatalf("unexpected bare raise parts: exc=%v cause=%v", exc, cause)
	}
}

func TestParseRaiseExpression(t *testing.T) {
	p, tree := parseSource(t, "raise ValueError\n")
	requireNoParseErrors(t, p)

	stmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, stmt, a.NodeRaise)
	exc, cause := tree.RaiseParts(stmt)
	if got := nameText(t, tree, exc); got != "ValueError" {
		t.Fatalf("unexpected raised expr: got %q", got)
	}
	if cause != a.NoNode {
		t.Fatalf("unexpected raise cause: %v", cause)
	}
}

func TestParseRaiseWithCause(t *testing.T) {
	p, tree := parseSource(t, "raise ValueError from cause\n")
	requireNoParseErrors(t, p)

	stmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, stmt, a.NodeRaise)
	exc, cause := tree.RaiseParts(stmt)
	if got := nameText(t, tree, exc); got != "ValueError" {
		t.Fatalf("unexpected raised expr: got %q", got)
	}
	if got := nameText(t, tree, cause); got != "cause" {
		t.Fatalf("unexpected raise cause: got %q", got)
	}
}

func TestParseRaiseCallExpression(t *testing.T) {
	p, tree := parseSource(t, "raise make_error()\n")
	requireNoParseErrors(t, p)

	stmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, stmt, a.NodeRaise)
	exc, _ := tree.RaiseParts(stmt)
	requireKind(t, tree, exc, a.NodeCall)
}

func TestParseAssertStatements(t *testing.T) {
	p, tree := parseSource(t, "assert x\nassert x, 'bad'\n")
	requireNoParseErrors(t, p)

	stmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, stmt, a.NodeAssert)
	test, msg := tree.AssertParts(stmt)
	if got := nameText(t, tree, test); got != "x" {
		t.Fatalf("unexpected assert test: got %q", got)
	}
	if msg != a.NoNode {
		t.Fatal("did not expect assert message for first statement")
	}

	stmt = moduleStmt(t, tree, 1)
	test, msg = tree.AssertParts(stmt)
	if got := nameText(t, tree, test); got != "x" {
		t.Fatalf("unexpected second assert test: got %q", got)
	}
	requireKind(t, tree, msg, a.NodeString)
}

func TestParseDelStatement(t *testing.T) {
	p, tree := parseSource(t, "del x, obj.attr, items[0]\n")
	requireNoParseErrors(t, p)

	stmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, stmt, a.NodeDel)
	targets := tree.DelTargets(stmt)
	if len(targets) != 3 {
		t.Fatalf("unexpected delete target count: got %d want 3", len(targets))
	}
	if got := nameText(t, tree, targets[0]); got != "x" {
		t.Fatalf("unexpected first delete target: got %q", got)
	}
	requireKind(t, tree, targets[1], a.NodeAttribute)
	requireKind(t, tree, targets[2], a.NodeSubScript)
}

func TestParseGlobalAndNonlocalStatements(t *testing.T) {
	p, tree := parseSource(t, "global x, y\nnonlocal inner\n")
	requireNoParseErrors(t, p)

	stmt := moduleStmt(t, tree, 0)
	requireKind(t, tree, stmt, a.NodeGlobal)
	names := tree.NameList(stmt)
	if len(names) != 2 {
		t.Fatalf("unexpected global name count: got %d want 2", len(names))
	}
	if got := nameText(t, tree, names[0]); got != "x" {
		t.Fatalf("unexpected first global name: got %q", got)
	}
	if got := nameText(t, tree, names[1]); got != "y" {
		t.Fatalf("unexpected second global name: got %q", got)
	}

	stmt = moduleStmt(t, tree, 1)
	requireKind(t, tree, stmt, a.NodeNonlocal)
	names = tree.NameList(stmt)
	if len(names) != 1 {
		t.Fatalf("unexpected nonlocal name count: got %d want 1", len(names))
	}
	if got := nameText(t, tree, names[0]); got != "inner" {
		t.Fatalf("unexpected nonlocal name: got %q", got)
	}
}

func TestParseRaiseErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{name: "missing expr", src: "raise )\n", want: "expected expression after 'raise'"},
		{name: "missing cause", src: "raise err from\n", want: "expected expression after 'from' in raise"},
		{name: "bad cause expr", src: "raise err from )\n", want: "expected expression after 'from' in raise"},
		{name: "missing assert expr", src: "assert\n", want: "expected expression after 'assert'"},
		{name: "missing del target", src: "del\n", want: "expected delete target after 'del'"},
		{name: "invalid del target", src: "del 1\n", want: "invalid delete target"},
		{name: "missing global name", src: "global\n", want: "expected name after 'global'"},
		{name: "missing nonlocal name", src: "nonlocal\n", want: "expected name after 'nonlocal'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := parseSource(t, tt.src)
			requireParseErrorContains(t, p, tt.want)
		})
	}
}

func TestParseSliceRange(t *testing.T) {
	p, tree := parseSource(t, "a[1:3]\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	sub := requireChildCount(t, tree, exprStmt, 1)[0]
	requireKind(t, tree, sub, a.NodeSubScript)
	subKids := requireChildCount(t, tree, sub, 2)
	if got := nameText(t, tree, subKids[0]); got != "a" {
		t.Fatalf("unexpected slice base: got %q", got)
	}
	requireKind(t, tree, subKids[1], a.NodeSlice)
	sliceKids := requireChildCount(t, tree, subKids[1], 2)
	if got := numberValue(t, tree, sliceKids[0]); got != "1" {
		t.Fatalf("unexpected slice start: got %q", got)
	}
	if got := numberValue(t, tree, sliceKids[1]); got != "3" {
		t.Fatalf("unexpected slice end: got %q", got)
	}
}

func TestParseSliceMissingStart(t *testing.T) {
	p, tree := parseSource(t, "a[:3]\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	sub := requireChildCount(t, tree, exprStmt, 1)[0]
	subKids := requireChildCount(t, tree, sub, 2)
	requireKind(t, tree, subKids[1], a.NodeSlice)
	sliceKids := requireChildCount(t, tree, subKids[1], 1)
	if got := numberValue(t, tree, sliceKids[0]); got != "3" {
		t.Fatalf("unexpected slice end: got %q", got)
	}
}

func TestParseSliceMissingEnd(t *testing.T) {
	p, tree := parseSource(t, "a[1:]\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	sub := requireChildCount(t, tree, exprStmt, 1)[0]
	subKids := requireChildCount(t, tree, sub, 2)
	requireKind(t, tree, subKids[1], a.NodeSlice)
	sliceKids := requireChildCount(t, tree, subKids[1], 1)
	if got := numberValue(t, tree, sliceKids[0]); got != "1" {
		t.Fatalf("unexpected slice start: got %q", got)
	}
}

func TestParseSliceFull(t *testing.T) {
	p, tree := parseSource(t, "a[:]\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	sub := requireChildCount(t, tree, exprStmt, 1)[0]
	subKids := requireChildCount(t, tree, sub, 2)
	requireKind(t, tree, subKids[1], a.NodeSlice)
	requireChildCount(t, tree, subKids[1], 0)
}

func TestParseCallArgumentSlice(t *testing.T) {
	p, tree := parseSource(t, "Foo(bar[1:3])\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	call := requireChildCount(t, tree, exprStmt, 1)[0]
	requireKind(t, tree, call, a.NodeCall)
	callKids := requireChildCount(t, tree, call, 2)
	requireKind(t, tree, callKids[1], a.NodeSubScript)
	subKids := requireChildCount(t, tree, callKids[1], 2)
	if got := nameText(t, tree, subKids[0]); got != "bar" {
		t.Fatalf("unexpected slice arg base: got %q", got)
	}
	requireKind(t, tree, subKids[1], a.NodeSlice)
}

func TestParseSliceChainedPostfix(t *testing.T) {
	p, tree := parseSource(t, "foo.bar[1:3](x)\n")
	requireNoParseErrors(t, p)

	exprStmt := moduleStmt(t, tree, 0)
	call := requireChildCount(t, tree, exprStmt, 1)[0]
	requireKind(t, tree, call, a.NodeCall)
	callKids := requireChildCount(t, tree, call, 2)
	requireKind(t, tree, callKids[0], a.NodeSubScript)
	subKids := requireChildCount(t, tree, callKids[0], 2)
	requireKind(t, tree, subKids[0], a.NodeAttribute)
	requireKind(t, tree, subKids[1], a.NodeSlice)
	if got := nameText(t, tree, callKids[1]); got != "x" {
		t.Fatalf("unexpected chained call arg: got %q", got)
	}
}

func TestParseSliceErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{name: "missing end expression", src: "a[1:\n", want: "expected expression after ':' in slice"},
		{name: "missing both with newline", src: "a[:\n", want: "expected expression after ':' in slice"},
		{name: "step slice unsupported", src: "a[1:2:3]\n", want: "slice step not supported"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := parseSource(t, tt.src)
			requireParseErrorContains(t, p, tt.want)
		})
	}
}
