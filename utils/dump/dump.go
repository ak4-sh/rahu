package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"rahu/analyser"
	"rahu/parser"
	"rahu/parser/ast"
)

var (
	bold    string
	red     string
	reset   string
	blue    string
	cyan    string
	green   string
	yellow  string
	magenta string
)

func initColors(noColor bool) {
	if noColor {
		bold = ""
		red = ""
		reset = ""
		blue = ""
		cyan = ""
		green = ""
		yellow = ""
		magenta = ""
	} else {
		bold = "\033[1m"
		red = "\033[31m"
		reset = "\033[0m"
		blue = "\033[34m"
		cyan = "\033[36m"
		green = "\033[32m"
		yellow = "\033[33m"
		magenta = "\033[35m"
	}
}

func header(s string) {
	fmt.Println(bold + cyan + s + reset)
}

func errLine(s string) {
	fmt.Println(red + s + reset)
}

func okLine(s string) {
	fmt.Println(green + s + reset)
}

func warnLine(s string) {
	fmt.Println(yellow + s + reset)
}

// ------------------------------------------------------

func main() {
	noColor := flag.Bool("no-color", false, "Disable colored output")
	flag.Parse()

	initColors(*noColor)

	src, err := os.ReadFile("temp.py")
	if err != nil {
		panic(err)
	}

	header("=== SOURCE ===")
	fmt.Println(string(src))
	fmt.Println()

	p := parser.New(string(src))
	module := p.Parse()

	if errs := p.Errors(); len(errs) > 0 {
		header("=== PARSER ERRORS ===")
		for _, e := range errs {
			errLine(fmt.Sprintf("%v: %s", e.Span, e.Msg))
		}
		return
	}

	global := analyser.BuildScopes(module)

	header("=== SCOPES ===")
	dumpScope(global, 0)
	fmt.Println()

	semErrors, resolved, resolvedAttrs, pendingAttrs := analyser.ResolveWithAttrs(module, global)

	header("=== RESOLVER STATS ===")
	fmt.Printf("names=%d attrs=%d pending=%d semErrs=%d\n\n",
		len(resolved),
		len(resolvedAttrs),
		len(pendingAttrs),
		len(semErrors),
	)

	if len(semErrors) > 0 {
		header("=== SEMANTIC ERRORS ===")
		for _, e := range semErrors {
			errLine(fmt.Sprintf("%v: %s", e.Span, e.Msg))
		}
		fmt.Println()
	}

	analyser.PromoteClassMembers(global)

	header("=== RESOLVED NAMES ===")
	for name, sym := range resolved {
		okLine(fmt.Sprintf(
			"%s @ [%d,%d] -> %s (%s)",
			name.ID,
			name.Pos.Start,
			name.Pos.End,
			sym.Name,
			sym.Kind,
		))
	}

	fmt.Println()
	header("=== ATTRIBUTE BINDINGS ===")
	dumpAttrBindings(module, resolvedAttrs)

	fmt.Println()
	header("=== ATTRIBUTES DISCOVERED (INSTANCE) ===")
	dumpClassAttrs(global)

	fmt.Println()
	header("=== PROMOTED CLASS MEMBERS ===")
	dumpClassMembers(global)
}

// ------------------------------------------------------------

func dumpScope(s *analyser.Scope, indent int) {
	prefix := strings.Repeat("  ", indent)
	fmt.Printf("%sScope(%s)\n", prefix, s.Kind)

	for _, sym := range s.Symbols {
		fmt.Printf("%s  %s%s%s : %s\n",
			prefix,
			yellow,
			sym.Name,
			reset,
			sym.Kind,
		)
		if sym.Inner != nil {
			dumpScope(sym.Inner, indent+2)
		}
	}
}

func dumpClassAttrs(s *analyser.Scope) {
	for _, sym := range s.Symbols {
		if sym.Kind == analyser.SymClass {
			fmt.Printf("%sClass %s%s\n", magenta, sym.Name, reset)
			if sym.Attrs != nil {
				for _, a := range sym.Attrs.Symbols {
					fmt.Printf("  %sattr%s %s\n", yellow, reset, a.Name)
				}
			}
			if sym.Inner != nil {
				dumpClassAttrs(sym.Inner)
			}
		}
	}
}

// ------------------------------------------------------------

func dumpAttrBindings(module *ast.Module, attrs map[*ast.Attribute]*analyser.Symbol) {
	walkStmt := func(st ast.Statement, visitExpr func(ast.Expression)) {}
	var walkExpr func(ast.Expression)

	walkExpr = func(e ast.Expression) {
		switch v := e.(type) {
		case *ast.Attribute:
			sym := attrs[v]
			if sym == nil {
				errLine(fmt.Sprintf(
					"UNBOUND attr %s at [%d,%d]",
					v.Attr.ID,
					v.Attr.Pos.Start,
					v.Attr.Pos.End,
				))
			} else {
				okLine(fmt.Sprintf(
					"BOUND   attr %s -> %s (%s) at [%d,%d]",
					v.Attr.ID,
					sym.Name,
					sym.Kind,
					v.Attr.Pos.Start,
					v.Attr.Pos.End,
				))
			}
			walkExpr(v.Value)

		case *ast.BinOp:
			walkExpr(v.Left)
			walkExpr(v.Right)

		case *ast.UnaryOp:
			walkExpr(v.Operand)

		case *ast.BooleanOp:
			for _, x := range v.Values {
				walkExpr(x)
			}

		case *ast.Compare:
			walkExpr(v.Left)
			for _, x := range v.Right {
				walkExpr(x)
			}

		case *ast.Call:
			walkExpr(v.Func)
			for _, x := range v.Args {
				walkExpr(x)
			}

		case *ast.Tuple:
			for _, x := range v.Elts {
				walkExpr(x)
			}

		case *ast.List:
			for _, x := range v.Elts {
				walkExpr(x)
			}
		}
	}

	walkStmt = func(st ast.Statement, visitExpr func(ast.Expression)) {
		switch s := st.(type) {
		case *ast.Assign:
			for _, t := range s.Targets {
				visitExpr(t)
			}
			visitExpr(s.Value)

		case *ast.ExprStmt:
			visitExpr(s.Value)

		case *ast.Return:
			if s.Value != nil {
				visitExpr(s.Value)
			}

		case *ast.FunctionDef, *ast.ClassDef:
			switch x := s.(type) {
			case *ast.FunctionDef:
				for _, b := range x.Body {
					walkStmt(b, visitExpr)
				}
			case *ast.ClassDef:
				for _, b := range x.Body {
					walkStmt(b, visitExpr)
				}
			}
		}
	}

	for _, st := range module.Body {
		walkStmt(st, walkExpr)
	}
}

func dumpClassMembers(s *analyser.Scope) {
	for _, sym := range s.Symbols {
		if sym.Kind == analyser.SymClass {
			fmt.Printf("%sClass %s%s\n", magenta, sym.Name, reset)

			if sym.Members != nil {
				for _, m := range sym.Members.Symbols {
					fmt.Printf("  %smember%s %s : %s\n",
						yellow, reset,
						m.Name,
						m.Kind,
					)
				}
			} else {
				warnLine("  (no members)")
			}

			if sym.Inner != nil {
				dumpClassMembers(sym.Inner)
			}
		}
	}
}
