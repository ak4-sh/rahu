package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
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
		bold, red, reset, blue, cyan, green, yellow, magenta = "", "", "", "", "", "", "", ""
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

func header(s string)   { fmt.Println(bold + cyan + s + reset) }
func errLine(s string)  { fmt.Println(red + s + reset) }
func okLine(s string)   { fmt.Println(green + s + reset) }
func warnLine(s string) { fmt.Println(yellow + s + reset) }

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

	global, _ := analyser.BuildScopes(module)

	header("=== SCOPES ===")
	dumpScope(global, 0)
	fmt.Println()

	analyser.PromoteClassMembers(global)

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

	header("=== RESOLVED NAMES ===")
	nameByID := collectNameNodes(module)

	type nameEntry struct {
		id   ast.NodeID
		name *ast.Name
		sym  *analyser.Symbol
	}

	var names []nameEntry
	for id, s := range resolved {
		n := nameByID[id]
		if n == nil {
			continue
		}
		names = append(names, nameEntry{id: id, name: n, sym: s})
	}

	sort.Slice(names, func(i, j int) bool {
		return names[i].name.Pos.Start < names[j].name.Pos.Start
	})

	for _, e := range names {
		okLine(fmt.Sprintf(
			"%s @ [%d,%d] -> %s (%s)",
			e.name.Text,
			e.name.Pos.Start,
			e.name.Pos.End,
			e.sym.Name,
			e.sym.Kind,
		))
	}

	fmt.Println()
	header("=== ATTRIBUTE BINDINGS ===")
	dumpAttrBindings(module, resolvedAttrs)

	fmt.Println()
	header("=== INSTANCE ATTRIBUTES PER CLASS ===")
	dumpClassAttrs(global)

	fmt.Println()
	header("=== PROMOTED CLASS MEMBERS ===")
	dumpClassMembers(global)
}

// ------------------------------------------------------------

func dumpScope(s *analyser.Scope, indent int) {
	prefix := strings.Repeat("  ", indent)

	fmt.Printf("%sScope(%s)\n", prefix, s.Kind)

	var names []string
	for n := range s.Symbols {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, n := range names {
		sym := s.Symbols[n]

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
				var names []string
				for n := range sym.Attrs.Symbols {
					names = append(names, n)
				}
				sort.Strings(names)

				for _, n := range names {
					fmt.Printf("  %sattr%s %s\n", yellow, reset, n)
				}
			}

			if sym.Inner != nil {
				dumpClassAttrs(sym.Inner)
			}
		}
	}
}

// ------------------------------------------------------------

func dumpAttrBindings(module *ast.Module, attrs map[ast.NodeID]*analyser.Symbol) {
	var walkExpr func(ast.Expression)

	walkExpr = func(e ast.Expression) {
		switch v := e.(type) {

		case *ast.Attribute:

			base := "<expr>"
			if n, ok := v.Value.(*ast.Name); ok {
				base = n.Text
			}

			sym := attrs[v.ID]

			if sym == nil {
				errLine(fmt.Sprintf(
					"UNBOUND %s.%s at [%d,%d]",
					base,
					v.Attr.Text,
					v.Attr.Pos.Start,
					v.Attr.Pos.End,
				))
			} else {
				okLine(fmt.Sprintf(
					"BOUND   %s.%s -> %s (%s) at [%d,%d]",
					base,
					v.Attr.Text,
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

	var walkStmt func(ast.Statement)

	walkStmt = func(st ast.Statement) {
		switch s := st.(type) {

		case *ast.Assign:
			for _, t := range s.Targets {
				walkExpr(t)
			}
			walkExpr(s.Value)

		case *ast.ExprStmt:
			walkExpr(s.Value)

		case *ast.Return:
			if s.Value != nil {
				walkExpr(s.Value)
			}

		case *ast.FunctionDef:
			for _, b := range s.Body {
				walkStmt(b)
			}

		case *ast.ClassDef:
			for _, b := range s.Body {
				walkStmt(b)
			}
		}
	}

	for _, st := range module.Body {
		walkStmt(st)
	}
}

func collectNameNodes(module *ast.Module) map[ast.NodeID]*ast.Name {
	result := make(map[ast.NodeID]*ast.Name)

	var walkExpr func(ast.Expression)
	walkExpr = func(e ast.Expression) {
		switch v := e.(type) {
		case *ast.Name:
			result[v.ID] = v
		case *ast.Attribute:
			if v.Attr != nil {
				result[v.Attr.ID] = v.Attr
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

	var walkStmt func(ast.Statement)
	walkStmt = func(st ast.Statement) {
		switch s := st.(type) {
		case *ast.Assign:
			for _, t := range s.Targets {
				walkExpr(t)
			}
			walkExpr(s.Value)
		case *ast.AugAssign:
			walkExpr(s.Target)
			walkExpr(s.Value)
		case *ast.ExprStmt:
			walkExpr(s.Value)
		case *ast.Return:
			if s.Value != nil {
				walkExpr(s.Value)
			}
		case *ast.FunctionDef:
			if s.Name != nil {
				result[s.Name.ID] = s.Name
			}
			for _, arg := range s.Args {
				if arg.Name != nil {
					result[arg.Name.ID] = arg.Name
				}
				if arg.Default != nil {
					walkExpr(arg.Default)
				}
			}
			for _, b := range s.Body {
				walkStmt(b)
			}
		case *ast.ClassDef:
			if s.Name != nil {
				result[s.Name.ID] = s.Name
			}
			for _, base := range s.Bases {
				if base != nil {
					walkExpr(base)
				}
			}
			for _, b := range s.Body {
				walkStmt(b)
			}
		case *ast.If:
			walkExpr(s.Test)
			for _, b := range s.Body {
				walkStmt(b)
			}
			for _, b := range s.Orelse {
				walkStmt(b)
			}
		case *ast.For:
			walkExpr(s.Target)
			walkExpr(s.Iter)
			for _, b := range s.Body {
				walkStmt(b)
			}
			for _, b := range s.Orelse {
				walkStmt(b)
			}
		case *ast.WhileLoop:
			walkExpr(s.Test)
			for _, b := range s.Body {
				walkStmt(b)
			}
		}
	}

	for _, st := range module.Body {
		walkStmt(st)
	}

	return result
}

func dumpClassMembers(s *analyser.Scope) {
	for _, sym := range s.Symbols {
		if sym.Kind == analyser.SymClass {

			fmt.Printf("%sClass %s%s\n", magenta, sym.Name, reset)

			if sym.Members != nil {

				var names []string
				for n := range sym.Members.Symbols {
					names = append(names, n)
				}
				sort.Strings(names)

				for _, n := range names {
					m := sym.Members.Symbols[n]

					fmt.Printf("  %smember%s %s : %s\n",
						yellow,
						reset,
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
