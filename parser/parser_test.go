package parser

import (
	"testing"
)

// ===== Assignment Tests =====

func TestSimpleAssignment(t *testing.T) {
	input := "x = 5"
	p := New(input)
	module := p.Parse()

	if len(module.Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(module.Body))
	}

	assign, ok := module.Body[0].(*Assign)
	if !ok {
		t.Fatalf("expected Assign, got %T", module.Body[0])
	}

	if len(assign.Targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(assign.Targets))
	}

	target, ok := assign.Targets[0].(*Name)
	if !ok || target.ID != "x" {
		t.Fatalf("expected target 'x', got %v", target)
	}

	value, ok := assign.Value.(*Number)
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

	assign := module.Body[0].(*Assign)

	// Should be BinOp(1, +, BinOp(2, *, 3))
	binop, ok := assign.Value.(*BinOp)
	if !ok {
		t.Fatalf("expected BinOp for value, got %T", assign.Value)
	}

	if binop.Op != Add {
		t.Fatalf("expected Add operator, got %v", binop.Op)
	}

	// Right side should be multiplication
	rightBinOp, ok := binop.Right.(*BinOp)
	if !ok || rightBinOp.Op != Mult {
		t.Fatalf("expected multiplication on right, got %T", binop.Right)
	}
}

// ===== Expression Tests =====

func TestBinaryOperations(t *testing.T) {
	tests := []struct {
		input    string
		expected Operator
	}{
		{"x = 1 + 2", Add},
		{"x = 1 - 2", Sub},
		{"x = 1 * 2", Mult},
		{"x = 1 / 2", Div},
		{"x = 1 // 2", FloorDiv},
		{"x = 1 % 2", Mod},
	}

	for _, tt := range tests {
		p := New(tt.input)
		module := p.Parse()
		assign := module.Body[0].(*Assign)
		binop := assign.Value.(*BinOp)

		if binop.Op != tt.expected {
			t.Errorf("for %q: expected %v, got %v", tt.input, tt.expected, binop.Op)
		}
	}
}

func TestOperatorPrecedence(t *testing.T) {
	input := "x = 1 + 2 * 3"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*Assign)
	binop := assign.Value.(*BinOp)

	// Top level should be addition
	if binop.Op != Add {
		t.Fatalf("expected top-level Add, got %v", binop.Op)
	}

	// Left should be Number(1)
	if _, ok := binop.Left.(*Number); !ok {
		t.Fatalf("expected Number on left, got %T", binop.Left)
	}

	// Right should be BinOp(2, *, 3)
	rightBinOp, ok := binop.Right.(*BinOp)
	if !ok {
		t.Fatalf("expected BinOp on right, got %T", binop.Right)
	}

	if rightBinOp.Op != Mult {
		t.Fatalf("expected Mult on right, got %v", rightBinOp.Op)
	}
}

func TestParentheses(t *testing.T) {
	input := "x = (1 + 2) * 3"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*Assign)
	binop := assign.Value.(*BinOp)

	// Top level should be multiplication
	if binop.Op != Mult {
		t.Fatalf("expected top-level Mult, got %v", binop.Op)
	}

	// Left should be BinOp(1, +, 2)
	leftBinOp, ok := binop.Left.(*BinOp)
	if !ok || leftBinOp.Op != Add {
		t.Fatalf("expected Add on left, got %T", binop.Left)
	}
}

// ===== Comparison Tests =====

func TestComparison(t *testing.T) {
	tests := []struct {
		input    string
		expected CompareOp
	}{
		{"x = 1 == 2", Eq},
		{"x = 1 != 2", NotEq},
		{"x = 1 < 2", Lt},
		{"x = 1 <= 2", LtE},
		{"x = 1 > 2", Gt},
		{"x = 1 >= 2", GtE},
	}

	for _, tt := range tests {
		p := New(tt.input)
		module := p.Parse()
		assign := module.Body[0].(*Assign)
		compare := assign.Value.(*Compare)

		if len(compare.Ops) != 1 || compare.Ops[0] != tt.expected {
			t.Errorf("for %q: expected %v, got %v", tt.input, tt.expected, compare.Ops)
		}
	}
}

func TestComparisonChaining(t *testing.T) {
	input := "x = 1 < y < 10"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*Assign)
	compare, ok := assign.Value.(*Compare)
	if !ok {
		t.Fatalf("expected Compare, got %T", assign.Value)
	}

	if len(compare.Ops) != 2 {
		t.Fatalf("expected 2 operators, got %d", len(compare.Ops))
	}

	if compare.Ops[0] != Lt || compare.Ops[1] != Lt {
		t.Fatalf("expected both Lt, got %v", compare.Ops)
	}
}

// ===== Boolean Operator Tests =====

func TestBooleanAnd(t *testing.T) {
	input := "x = a and b and c"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*Assign)
	boolOp, ok := assign.Value.(*BooleanOp)
	if !ok {
		t.Fatalf("expected BooleanOp, got %T", assign.Value)
	}

	if boolOp.Operator != And {
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

	assign := module.Body[0].(*Assign)
	boolOp := assign.Value.(*BooleanOp)

	if boolOp.Operator != Or {
		t.Fatalf("expected Or, got %v", boolOp.Operator)
	}
}

func TestBooleanPrecedence(t *testing.T) {
	input := "x = a or b and c"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*Assign)
	orOp := assign.Value.(*BooleanOp)

	// Top level should be Or
	if orOp.Operator != Or {
		t.Fatalf("expected Or at top, got %v", orOp.Operator)
	}

	// Second value should be And(b, c)
	andOp, ok := orOp.Values[1].(*BooleanOp)
	if !ok || andOp.Operator != And {
		t.Fatalf("expected And as second value, got %T", orOp.Values[1])
	}
}

func TestComplexBooleanExpression(t *testing.T) {
	input := "x = a > 5 and b < 10 or c == 0"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*Assign)

	// Should be Or(And(a>5, b<10), c==0)
	orOp, ok := assign.Value.(*BooleanOp)
	if !ok || orOp.Operator != Or {
		t.Fatalf("expected Or at top level, got %T", assign.Value)
	}

	// First value should be And
	andOp, ok := orOp.Values[0].(*BooleanOp)
	if !ok || andOp.Operator != And {
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

	assign := module.Body[0].(*Assign)
	unary, ok := assign.Value.(*UnaryOp)
	if !ok {
		t.Fatalf("expected UnaryOp, got %T", assign.Value)
	}

	if unary.Op != USub {
		t.Fatalf("expected USub, got %v", unary.Op)
	}
}

func TestUnaryNot(t *testing.T) {
	input := "x = not y"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*Assign)
	unary := assign.Value.(*UnaryOp)

	if unary.Op != Not {
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

	funcDef, ok := module.Body[0].(*FunctionDef)
	if !ok {
		t.Fatalf("expected FunctionDef, got %T", module.Body[0])
	}

	if funcDef.Name != "add" {
		t.Fatalf("expected name 'add', got %q", funcDef.Name)
	}

	if len(funcDef.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(funcDef.Args))
	}

	if funcDef.Args[0].Name != "x" || funcDef.Args[1].Name != "y" {
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

	funcDef := module.Body[0].(*FunctionDef)

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

	defaultVal, ok := funcDef.Args[1].Default.(*String)
	if !ok || defaultVal.Value != "Hello" {
		t.Fatalf("expected default 'Hello', got %v", funcDef.Args[1].Default)
	}
}

func TestFunctionCall(t *testing.T) {
	input := "result = add(1, 2)"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*Assign)
	call, ok := assign.Value.(*Call)
	if !ok {
		t.Fatalf("expected Call, got %T", assign.Value)
	}

	funcName, ok := call.Func.(*Name)
	if !ok || funcName.ID != "add" {
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

	assign := module.Body[0].(*Assign)
	call := assign.Value.(*Call)

	if len(call.Args) != 0 {
		t.Fatalf("expected 0 args, got %d", len(call.Args))
	}
}

// ===== Control Flow Tests =====

func TestIfStatement(t *testing.T) {
	input := `if x > 5:
    print(x)`
	p := New(input)
	module := p.Parse()

	ifStmt, ok := module.Body[0].(*If)
	if !ok {
		t.Fatalf("expected If, got %T", module.Body[0])
	}

	if _, ok := ifStmt.Test.(*Compare); !ok {
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

	ifStmt := module.Body[0].(*If)

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

	ifStmt := module.Body[0].(*If)

	if len(ifStmt.Orelse) != 1 {
		t.Fatalf("expected 1 orelse statement, got %d", len(ifStmt.Orelse))
	}

	// Elif is another If in orelse
	elifStmt, ok := ifStmt.Orelse[0].(*If)
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

	forStmt, ok := module.Body[0].(*For)
	if !ok {
		t.Fatalf("expected For, got %T", module.Body[0])
	}

	target, ok := forStmt.Target.(*Name)
	if !ok || target.ID != "i" {
		t.Fatalf("expected target 'i', got %v", forStmt.Target)
	}

	if _, ok := forStmt.Iter.(*Call); !ok {
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

	forStmt := module.Body[0].(*For)

	tuple, ok := forStmt.Target.(*Tuple)
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

	whileStmt, ok := module.Body[0].(*WhileLoop)
	if !ok {
		t.Fatalf("expected WhileLoop, got %T", module.Body[0])
	}

	if _, ok := whileStmt.Test.(*Compare); !ok {
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

	whileStmt := module.Body[0].(*WhileLoop)

	_, ok := whileStmt.Body[0].(*Break)
	if !ok {
		t.Fatalf("expected Break, got %T", whileStmt.Body[0])
	}
}

func TestContinueStatement(t *testing.T) {
	input := `for i in range(10):
    continue`
	p := New(input)
	module := p.Parse()

	forStmt := module.Body[0].(*For)

	_, ok := forStmt.Body[0].(*Continue)
	if !ok {
		t.Fatalf("expected Continue, got %T", forStmt.Body[0])
	}
}

// ===== List Tests =====

func TestEmptyList(t *testing.T) {
	input := "x = []"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*Assign)
	list, ok := assign.Value.(*List)
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

	assign := module.Body[0].(*Assign)
	list := assign.Value.(*List)

	if len(list.Elts) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(list.Elts))
	}
}

func TestNestedList(t *testing.T) {
	input := "x = [1, [2, 3], 4]"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*Assign)
	list := assign.Value.(*List)

	if len(list.Elts) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(list.Elts))
	}

	// Second element should be a list
	_, ok := list.Elts[1].(*List)
	if !ok {
		t.Fatalf("expected nested List, got %T", list.Elts[1])
	}
}

func TestListWithTrailingComma(t *testing.T) {
	input := "x = [1, 2, 3,]"
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*Assign)
	list := assign.Value.(*List)

	if len(list.Elts) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(list.Elts))
	}
}

// ===== Literal Tests =====

func TestStringLiteral(t *testing.T) {
	input := `x = "hello"`
	p := New(input)
	module := p.Parse()

	assign := module.Body[0].(*Assign)
	str, ok := assign.Value.(*String)
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
		assign := module.Body[0].(*Assign)
		boolean, ok := assign.Value.(*Boolean)
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

	funcDef := module.Body[0].(*FunctionDef)
	returnStmt, ok := funcDef.Body[0].(*Return)
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

	funcDef := module.Body[0].(*FunctionDef)
	returnStmt := funcDef.Body[0].(*Return)

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
	_, ok := module.Body[0].(*FunctionDef)
	if !ok {
		t.Fatalf("expected FunctionDef first, got %T", module.Body[0])
	}

	// Second should be assignment
	_, ok = module.Body[1].(*Assign)
	if !ok {
		t.Fatalf("expected Assign second, got %T", module.Body[1])
	}

	// Third should be expression statement
	_, ok = module.Body[2].(*ExprStmt)
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

	forStmt := module.Body[0].(*For)

	if len(forStmt.Body) != 2 {
		t.Fatalf("expected 2 statements in for body, got %d", len(forStmt.Body))
	}

	// First should be if
	ifStmt, ok := forStmt.Body[0].(*If)
	if !ok {
		t.Fatalf("expected If, got %T", forStmt.Body[0])
	}

	// If body should contain continue
	_, ok = ifStmt.Body[0].(*Continue)
	if !ok {
		t.Fatalf("expected Continue in if body, got %T", ifStmt.Body[0])
	}
}
