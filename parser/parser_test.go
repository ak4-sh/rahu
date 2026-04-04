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
