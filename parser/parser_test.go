package parser

import (
	"os"
	"testing"

	"rahu/parser/ast"
)

// ===== Assignment Tests =====

func TestSimpleAssignment(t *testing.T) {
	input := "x = 5"
	p := New(input)
	module := p.Parse()

	if len(module.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(module.Body))
	}

	assign, ok := module.Body[0].(*ast.Assign)
	if !ok {
		t.Fatalf("expected Assign, got %T", module.Body[0])
	}

	if len(assign.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(assign.Targets))
	}

	target, ok := assign.Targets[0].(*ast.Name)
	if !ok || target.Text != "x" {
		t.Fatalf("expected target 'x', got %v", target)
	}

	value, ok := assign.Value.(*ast.Number)
	if !ok || value.Value != "5" {
		t.Fatalf("expected value '5', got %v", value)
	}
}

func TestExpressionAssignment(t *testing.T) {
	input := "result = 1 + 2 * 3"
	p := New(input)
	module := p.Parse()

	if len(module.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(module.Body))
	}

	assign := module.Body[0].(*ast.Assign)

	// Should be BinOp(1, +, BinOp(2, *, 3))
	binop, ok := assign.Value.(*ast.BinOp)
	if !ok {
		t.Fatalf("expected BinOp for value, got %T", assign.Value)
	}

	if binop.Op != ast.Add {
		t.Fatalf("expected Add operator, got %v", binop.Op)
	}

	// Right side should be multiplication
	rightBinOp, ok := binop.Right.(*ast.BinOp)
	if !ok || rightBinOp.Op != ast.Mult {
		t.Fatalf("expected multiplication on right, got %T", binop.Right)
	}
}

// ===== Expression Tests =====

func TestBinaryOperations(t *testing.T) {
	tests := []struct {
		input    string
		expected ast.Operator
	}{
		{"x = 1 + 2", ast.Add},
		{"x = 1 - 2", ast.Sub},
		{"x = 1 * 2", ast.Mult},
		{"x = 1 / 2", ast.Div},
		{"x = 1 // 2", ast.FloorDiv},
		{"x = 1 % 2", ast.Mod},
	}

	for _, tt := range tests {
		p := New(tt.input)
		module := p.Parse()
		assign := module.Body[0].(*ast.Assign)
		binop := assign.Value.(*ast.BinOp)

		if binop.Op != tt.expected {
			t.Errorf("for %q: expected %v, got %v", tt.input, tt.expected, binop.Op)
		}
	}
}

func TestOperatorPrecedence(t *testing.T) {
	input := "x = 1 + 2 * 3"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*ast.Assign)
	binop := assign.Value.(*ast.BinOp)

	// Top level should be addition
	if binop.Op != ast.Add {
		t.Fatalf("expected top-level Add, got %v", binop.Op)
	}

	// Left should be Number(1)
	if _, ok := binop.Left.(*ast.Number); !ok {
		t.Fatalf("expected Number on left, got %T", binop.Left)
	}

	// Right should be BinOp(2, *, 3)
	rightBinOp, ok := binop.Right.(*ast.BinOp)
	if !ok {
		t.Fatalf("expected BinOp on right, got %T", binop.Right)
	}

	if rightBinOp.Op != ast.Mult {
		t.Fatalf("expected Mult on right, got %v", rightBinOp.Op)
	}
}

func TestParentheses(t *testing.T) {
	input := "x = (1 + 2) * 3"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*ast.Assign)
	binop := assign.Value.(*ast.BinOp)

	// Top level should be multiplication
	if binop.Op != ast.Mult {
		t.Fatalf("expected top-level Mult, got %v", binop.Op)
	}

	// Left should be BinOp(1, +, 2)
	leftBinOp, ok := binop.Left.(*ast.BinOp)
	if !ok || leftBinOp.Op != ast.Add {
		t.Fatalf("expected Add on left, got %T", binop.Left)
	}
}

// ===== Comparison Tests =====

func TestComparison(t *testing.T) {
	tests := []struct {
		input    string
		expected ast.CompareOp
	}{
		{"x = 1 == 2", ast.Eq},
		{"x = 1 != 2", ast.NotEq},
		{"x = 1 < 2", ast.Lt},
		{"x = 1 <= 2", ast.LtE},
		{"x = 1 > 2", ast.Gt},
		{"x = 1 >= 2", ast.GtE},
	}

	for _, tt := range tests {
		p := New(tt.input)
		module := p.Parse()
		assign := module.Body[0].(*ast.Assign)
		compare := assign.Value.(*ast.Compare)

		if len(compare.Ops) != 1 || compare.Ops[0] != tt.expected {
			t.Errorf("for %q: expected %v, got %v", tt.input, tt.expected, compare.Ops)
		}
	}
}

func TestComparisonChaining(t *testing.T) {
	input := "x = 1 < y < 10"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*ast.Assign)
	compare, ok := assign.Value.(*ast.Compare)
	if !ok {
		t.Fatalf("expected Compare, got %T", assign.Value)
	}

	if len(compare.Ops) != 2 {
		t.Fatalf("expected 2 operators, got %d", len(compare.Ops))
	}

	if compare.Ops[0] != ast.Lt || compare.Ops[1] != ast.Lt {
		t.Fatalf("expected both Lt, got %v", compare.Ops)
	}
}

// ===== Boolean Operator Tests =====

func TestBooleanAnd(t *testing.T) {
	input := "x = a and b and c"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*ast.Assign)
	boolOp, ok := assign.Value.(*ast.BooleanOp)
	if !ok {
		t.Fatalf("expected BooleanOp, got %T", assign.Value)
	}

	if boolOp.Operator != ast.And {
		t.Fatalf("expected And, got %v", boolOp.Operator)
	}

	if len(boolOp.Values) != 3 {
		t.Fatalf("expected 3 values, got %d", len(boolOp.Values))
	}
}

func TestBooleanOr(t *testing.T) {
	input := "x = a or b or c"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*ast.Assign)
	boolOp := assign.Value.(*ast.BooleanOp)

	if boolOp.Operator != ast.Or {
		t.Fatalf("expected Or, got %v", boolOp.Operator)
	}
}

func TestBooleanPrecedence(t *testing.T) {
	input := "x = a or b and c"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*ast.Assign)
	orOp := assign.Value.(*ast.BooleanOp)

	// Top level should be Or
	if orOp.Operator != ast.Or {
		t.Fatalf("expected Or at top, got %v", orOp.Operator)
	}

	// Second value should be And(b, c)
	andOp, ok := orOp.Values[1].(*ast.BooleanOp)
	if !ok || andOp.Operator != ast.And {
		t.Fatalf("expected And as second value, got %T", orOp.Values[1])
	}
}

func TestComplexBooleanExpression(t *testing.T) {
	input := "x = a > 5 and b < 10 or c == 0"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*ast.Assign)

	// Should be Or(And(a>5, b<10), c==0)
	orOp, ok := assign.Value.(*ast.BooleanOp)
	if !ok || orOp.Operator != ast.Or {
		t.Fatalf("expected Or at top level, got %T", assign.Value)
	}

	// First value should be And
	andOp, ok := orOp.Values[0].(*ast.BooleanOp)
	if !ok || andOp.Operator != ast.And {
		t.Fatalf("expected And in first value, got %T", orOp.Values[0])
	}

	// And should contain two comparisons
	if len(andOp.Values) != 2 {
		t.Fatalf("expected 2 values in And, got %d", len(andOp.Values))
	}
}

// ===== Unary Operator Tests =====

func TestUnaryMinus(t *testing.T) {
	input := "x = -5"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*ast.Assign)
	unary, ok := assign.Value.(*ast.UnaryOp)
	if !ok {
		t.Fatalf("expected UnaryOp, got %T", assign.Value)
	}

	if unary.Op != ast.USub {
		t.Fatalf("expected USub, got %v", unary.Op)
	}
}

func TestUnaryNot(t *testing.T) {
	input := "x = not y"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*ast.Assign)
	unary := assign.Value.(*ast.UnaryOp)

	if unary.Op != ast.Not {
		t.Fatalf("expected Not, got %v", unary.Op)
	}
}

// ===== Function Tests =====

func TestFunctionDefinition(t *testing.T) {
	input := `def add(x, y):
    return x + y`
	p := New(input)
	module := p.Parse()

	if len(module.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(module.Body))
	}

	funcDef, ok := module.Body[0].(*ast.FunctionDef)
	if !ok {
		t.Fatalf("expected FunctionDef, got %T", module.Body[0])
	}

	if funcDef.Name.Text != "add" {
		t.Fatalf("expected name 'add', got %q", funcDef.Name.Text)
	}

	if len(funcDef.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(funcDef.Args))
	}

	if funcDef.Args[0].Name.Text != "x" || funcDef.Args[1].Name.Text != "y" {
		t.Fatalf("unexpected arg names: %v", funcDef.Args)
	}

	if len(funcDef.Body) != 1 {
		t.Fatalf("expected 1 body statement, got %d", len(funcDef.Body))
	}
}

func TestFunctionWithDefaults(t *testing.T) {
	input := `def greet(name, greeting="Hello"):
    print(greeting)`
	p := New(input)
	module := p.Parse()

	funcDef := module.Body[0].(*ast.FunctionDef)

	if len(funcDef.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(funcDef.Args))
	}

	// First arg has no default
	if funcDef.Args[0].Default != nil {
		t.Fatalf("expected no default for first arg")
	}

	// Second arg has default
	if funcDef.Args[1].Default == nil {
		t.Fatalf("expected default for second arg")
	}

	defaultVal, ok := funcDef.Args[1].Default.(*ast.String)
	if !ok || defaultVal.Value != "Hello" {
		t.Fatalf("expected default 'Hello', got %v", funcDef.Args[1].Default)
	}
}

func TestFunctionCall(t *testing.T) {
	input := "result = add(1, 2)"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*ast.Assign)
	call, ok := assign.Value.(*ast.Call)
	if !ok {
		t.Fatalf("expected Call, got %T", assign.Value)
	}

	funcName, ok := call.Func.(*ast.Name)
	if !ok || funcName.Text != "add" {
		t.Fatalf("expected function 'add', got %v", call.Func)
	}

	if len(call.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(call.Args))
	}
}

func TestFunctionCallNoArgs(t *testing.T) {
	input := "result = foo()"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*ast.Assign)
	call := assign.Value.(*ast.Call)

	if len(call.Args) != 0 {
		t.Fatalf("expected 0 args, got %d", len(call.Args))
	}
}

func TestFunctionCallMultipleArgs(t *testing.T) {
	input := "result = foo(1, 2, 3)"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*ast.Assign)
	call := assign.Value.(*ast.Call)

	if len(call.Args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(call.Args))
	}

	for i, expected := range []string{"1", "2", "3"} {
		num, ok := call.Args[i].(*ast.Number)
		if !ok {
			t.Fatalf("arg %d: expected Number, got %T", i, call.Args[i])
		}
		if num.Value != expected {
			t.Fatalf("arg %d: expected %q, got %q", i, expected, num.Value)
		}
	}
}

func TestFunctionCallTrailingComma(t *testing.T) {
	input := "result = foo(1, 2,)"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*ast.Assign)
	call := assign.Value.(*ast.Call)

	if len(call.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(call.Args))
	}
}

func TestFunctionCallNestedCalls(t *testing.T) {
	input := "result = foo(bar(1), 2)"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*ast.Assign)
	call := assign.Value.(*ast.Call)

	if len(call.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(call.Args))
	}

	innerCall, ok := call.Args[0].(*ast.Call)
	if !ok {
		t.Fatalf("expected first arg to be Call, got %T", call.Args[0])
	}

	innerFunc, ok := innerCall.Func.(*ast.Name)
	if !ok || innerFunc.Text != "bar" {
		t.Fatalf("expected inner call to 'bar', got %v", innerCall.Func)
	}

	if len(innerCall.Args) != 1 {
		t.Fatalf("expected 1 inner arg, got %d", len(innerCall.Args))
	}
}

func TestFunctionCallExpressionArgs(t *testing.T) {
	input := "result = foo(1 + 2, x * y)"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*ast.Assign)
	call := assign.Value.(*ast.Call)

	if len(call.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(call.Args))
	}

	// First arg should be BinOp(1, +, 2)
	binop1, ok := call.Args[0].(*ast.BinOp)
	if !ok {
		t.Fatalf("expected first arg to be BinOp, got %T", call.Args[0])
	}
	if binop1.Op != ast.Add {
		t.Fatalf("expected Add, got %v", binop1.Op)
	}

	// Second arg should be BinOp(x, *, y)
	binop2, ok := call.Args[1].(*ast.BinOp)
	if !ok {
		t.Fatalf("expected second arg to be BinOp, got %T", call.Args[1])
	}
	if binop2.Op != ast.Mult {
		t.Fatalf("expected Mult, got %v", binop2.Op)
	}
}

// ===== Control Flow Tests =====

func TestIfStatement(t *testing.T) {
	input := `if x > 5:
    print(x)`
	p := New(input)
	module := p.Parse()

	ifStmt, ok := module.Body[0].(*ast.If)
	if !ok {
		t.Fatalf("expected If, got %T", module.Body[0])
	}

	if _, ok := ifStmt.Test.(*ast.Compare); !ok {
		t.Fatalf("expected Compare in test, got %T", ifStmt.Test)
	}

	if len(ifStmt.Body) != 1 {
		t.Fatalf("expected 1 body statement, got %d", len(ifStmt.Body))
	}

	if len(ifStmt.Orelse) != 0 {
		t.Fatalf("expected no else, got %d statements", len(ifStmt.Orelse))
	}
}

func TestIfElse(t *testing.T) {
	input := `if x > 5:
    print("big")
else:
    print("small")`
	p := New(input)
	module := p.Parse()

	ifStmt := module.Body[0].(*ast.If)

	if len(ifStmt.Orelse) != 1 {
		t.Fatalf("expected 1 else statement, got %d", len(ifStmt.Orelse))
	}
}

func TestIfElifElse(t *testing.T) {
	input := `if x > 10:
    print("big")
elif x > 5:
    print("medium")
else:
    print("small")`
	p := New(input)
	module := p.Parse()

	ifStmt := module.Body[0].(*ast.If)

	if len(ifStmt.Orelse) != 1 {
		t.Fatalf("expected 1 orelse statement, got %d", len(ifStmt.Orelse))
	}

	// Elif is another If in orelse
	elifStmt, ok := ifStmt.Orelse[0].(*ast.If)
	if !ok {
		t.Fatalf("expected elif to be If, got %T", ifStmt.Orelse[0])
	}

	// Elif should have an else
	if len(elifStmt.Orelse) != 1 {
		t.Fatalf("expected else in elif, got %d", len(elifStmt.Orelse))
	}
}

func TestForLoop(t *testing.T) {
	input := `for i in range(10):
    print(i)`
	p := New(input)
	module := p.Parse()

	forStmt, ok := module.Body[0].(*ast.For)
	if !ok {
		t.Fatalf("expected For, got %T", module.Body[0])
	}

	target, ok := forStmt.Target.(*ast.Name)
	if !ok || target.Text != "i" {
		t.Fatalf("expected target 'i', got %v", forStmt.Target)
	}

	if _, ok := forStmt.Iter.(*ast.Call); !ok {
		t.Fatalf("expected Call in iter, got %T", forStmt.Iter)
	}

	if len(forStmt.Body) != 1 {
		t.Fatalf("expected 1 body statement, got %d", len(forStmt.Body))
	}
}

func TestForLoopWithTupleUnpacking(t *testing.T) {
	input := `for x, y in pairs:
    print(x)`
	p := New(input)
	module := p.Parse()

	forStmt := module.Body[0].(*ast.For)

	tuple, ok := forStmt.Target.(*ast.Tuple)
	if !ok {
		t.Fatalf("expected Tuple target, got %T", forStmt.Target)
	}

	if len(tuple.Elts) != 2 {
		t.Fatalf("expected 2 elements in tuple, got %d", len(tuple.Elts))
	}
}

func TestWhileLoop(t *testing.T) {
	input := `while x > 0:
    x = x - 1`
	p := New(input)
	module := p.Parse()

	whileStmt, ok := module.Body[0].(*ast.WhileLoop)
	if !ok {
		t.Fatalf("expected WhileLoop, got %T", module.Body[0])
	}

	if _, ok := whileStmt.Test.(*ast.Compare); !ok {
		t.Fatalf("expected Compare in test, got %T", whileStmt.Test)
	}

	if len(whileStmt.Body) != 1 {
		t.Fatalf("expected 1 body statement, got %d", len(whileStmt.Body))
	}
}

func TestBreakStatement(t *testing.T) {
	input := `while True:
    break`
	p := New(input)
	module := p.Parse()

	whileStmt := module.Body[0].(*ast.WhileLoop)

	_, ok := whileStmt.Body[0].(*ast.Break)
	if !ok {
		t.Fatalf("expected Break, got %T", whileStmt.Body[0])
	}
}

func TestContinueStatement(t *testing.T) {
	input := `for i in range(10):
    continue`
	p := New(input)
	module := p.Parse()

	forStmt := module.Body[0].(*ast.For)

	_, ok := forStmt.Body[0].(*ast.Continue)
	if !ok {
		t.Fatalf("expected Continue, got %T", forStmt.Body[0])
	}
}

// ===== List Tests =====

func TestEmptyList(t *testing.T) {
	input := "x = []"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*ast.Assign)
	list, ok := assign.Value.(*ast.List)
	if !ok {
		t.Fatalf("expected List, got %T", assign.Value)
	}

	if len(list.Elts) != 0 {
		t.Fatalf("expected empty list, got %d elements", len(list.Elts))
	}
}

func TestSimpleList(t *testing.T) {
	input := "x = [1, 2, 3]"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*ast.Assign)
	list := assign.Value.(*ast.List)

	if len(list.Elts) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(list.Elts))
	}
}

func TestNestedList(t *testing.T) {
	input := "x = [1, [2, 3], 4]"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*ast.Assign)
	list := assign.Value.(*ast.List)

	if len(list.Elts) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(list.Elts))
	}

	// Second element should be a list
	_, ok := list.Elts[1].(*ast.List)
	if !ok {
		t.Fatalf("expected nested List, got %T", list.Elts[1])
	}
}

func TestListWithTrailingComma(t *testing.T) {
	input := "x = [1, 2, 3,]"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*ast.Assign)
	list := assign.Value.(*ast.List)

	if len(list.Elts) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(list.Elts))
	}
}

// ===== Literal Tests =====

func TestStringLiteral(t *testing.T) {
	input := `x = "hello"`
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*ast.Assign)
	str, ok := assign.Value.(*ast.String)
	if !ok {
		t.Fatalf("expected String, got %T", assign.Value)
	}

	if str.Value != "hello" {
		t.Fatalf("expected 'hello', got %q", str.Value)
	}
}

func TestBooleanLiterals(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"x = True", true},
		{"x = False", false},
	}

	for _, tt := range tests {
		p := New(tt.input)
		module := p.Parse()
		assign := module.Body[0].(*ast.Assign)
		boolean, ok := assign.Value.(*ast.Boolean)
		if !ok {
			t.Fatalf("expected Boolean, got %T", assign.Value)
		}

		if boolean.Value != tt.expected {
			t.Errorf("for %q: expected %v, got %v", tt.input, tt.expected, boolean.Value)
		}
	}
}

// ===== Return Statement Tests =====

func TestReturnWithValue(t *testing.T) {
	input := `def foo():
    return 42`
	p := New(input)
	module := p.Parse()

	funcDef := module.Body[0].(*ast.FunctionDef)
	returnStmt, ok := funcDef.Body[0].(*ast.Return)
	if !ok {
		t.Fatalf("expected Return, got %T", funcDef.Body[0])
	}

	if returnStmt.Value == nil {
		t.Fatalf("expected return value, got nil")
	}
}

func TestReturnEmpty(t *testing.T) {
	input := `def foo():
    return`
	p := New(input)
	module := p.Parse()

	funcDef := module.Body[0].(*ast.FunctionDef)
	returnStmt := funcDef.Body[0].(*ast.Return)

	if returnStmt.Value != nil {
		t.Fatalf("expected nil return value, got %v", returnStmt.Value)
	}
}

// ===== Complex Integration Tests =====

func TestComplexProgram(t *testing.T) {
	input := `def fibonacci(n):
    if n <= 1:
        return n
    return fibonacci(n - 1) + fibonacci(n - 2)

result = fibonacci(10)
print(result)`

	p := New(input)
	module := p.Parse()

	if len(module.Body) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(module.Body))
	}

	// First should be function def
	_, ok := module.Body[0].(*ast.FunctionDef)
	if !ok {
		t.Fatalf("expected FunctionDef first, got %T", module.Body[0])
	}

	// Second should be assignment
	_, ok = module.Body[1].(*ast.Assign)
	if !ok {
		t.Fatalf("expected Assign second, got %T", module.Body[1])
	}

	// Third should be expression statement
	_, ok = module.Body[2].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("expected ExprStmt third, got %T", module.Body[2])
	}
}

func TestNestedControlFlow(t *testing.T) {
	input := `for i in range(10):
    if i % 2 == 0:
        continue
    print(i)`

	p := New(input)
	module := p.Parse()

	forStmt := module.Body[0].(*ast.For)

	if len(forStmt.Body) != 2 {
		t.Fatalf("expected 2 statements in for body, got %d", len(forStmt.Body))
	}

	// First should be if
	ifStmt, ok := forStmt.Body[0].(*ast.If)
	if !ok {
		t.Fatalf("expected If, got %T", forStmt.Body[0])
	}

	// If body should contain continue
	_, ok = ifStmt.Body[0].(*ast.Continue)
	if !ok {
		t.Fatalf("expected Continue in if body, got %T", ifStmt.Body[0])
	}
}

func BenchmarkParseMega(b *testing.B) {
	src, err := os.ReadFile("../mega.py")
	if err != nil {
		b.Fatal(err)
	}

	input := string(src)

	for b.Loop() {
		p := New(input)
		_ = p.Parse()
	}
}

func BenchmarkParseSmall(b *testing.B) {
	src, err := os.ReadFile("../longerScript.py")
	if err != nil {
		b.Fatal(err)
	}

	input := string(src)

	for b.Loop() {
		p := New(input)
		_ = p.Parse()
	}
}
