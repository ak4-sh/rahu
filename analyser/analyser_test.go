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
	module := p.Parse()

	global := BuildScopes(module)

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
	module := p.Parse()
	global := BuildScopes(module)

	errs, resolved := Resolve(module, global)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %+v", errs)
	}

	var xUses []*ast.Name
	collectNames(module, &xUses)

	if resolved[xUses[0]].Name != "x" {
		t.Fatal("x did not resolve to symbol x")
	}
}

func collectNames(n ast.Node, out *[]*ast.Name) {
	switch v := n.(type) {

	// ----- statements -----

	case *ast.Module:
		for _, s := range v.Body {
			collectNames(s, out)
		}

	case *ast.Assign:
		for _, t := range v.Targets {
			collectNames(t, out)
		}
		collectNames(v.Value, out)

	case *ast.FunctionDef:
		for _, arg := range v.Args {
			if arg.Default != nil {
				collectNames(arg.Default, out)
			}
		}
		for _, s := range v.Body {
			collectNames(s, out)
		}

	case *ast.ExprStmt:
		collectNames(v.Value, out)

	case *ast.If:
		collectNames(v.Test, out)
		for _, s := range v.Body {
			collectNames(s, out)
		}
		for _, s := range v.Orelse {
			collectNames(s, out)
		}

	case *ast.For:
		collectNames(v.Target, out)
		collectNames(v.Iter, out)
		for _, s := range v.Body {
			collectNames(s, out)
		}

	case *ast.WhileLoop:
		collectNames(v.Test, out)
		for _, s := range v.Body {
			collectNames(s, out)
		}

	case *ast.Return:
		if v.Value != nil {
			collectNames(v.Value, out)
		}

	// ----- expressions -----

	case *ast.Name:
		*out = append(*out, v)

	case *ast.BinOp:
		collectNames(v.Left, out)
		collectNames(v.Right, out)

	case *ast.UnaryOp:
		collectNames(v.Operand, out)

	case *ast.Call:
		collectNames(v.Func, out)
		for _, a := range v.Args {
			collectNames(a, out)
		}

	case *ast.Compare:
		collectNames(v.Left, out)
		for _, r := range v.Right {
			collectNames(r, out)
		}

	case *ast.Tuple:
		for _, e := range v.Elts {
			collectNames(e, out)
		}

	case *ast.List:
		for _, e := range v.Elts {
			collectNames(e, out)
		}

	case *ast.BooleanOp:
		for _, e := range v.Values {
			collectNames(e, out)
		}

	// literals: nothing to do
	case *ast.Number, *ast.String, *ast.Boolean:
		return
	}
}
