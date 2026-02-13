// Package utils contains small, reusable helpers that sit outside the core
// compiler pipeline.
//
// It currently provides:
//   - AST inspection utilities (pretty-printing and colored debugging output)
//   - File parsing helpers for loading source files into memory
//
// The utilities in this package are intentionally side-effect free (except for
// I/O) and do not participate in parsing, semantic analysis, or code generation.
// They exist to support tooling, debugging, and developer ergonomics.
package utils

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"rahu/parser"
)

const (
	reset  = "\033[0m"
	blue   = "\033[34m" // node types
	cyan   = "\033[36m" // fields
	green  = "\033[32m" // literals
	yellow = "\033[33m" // operators / keywords
)

type PrintOptions struct {
	UseColor bool
}

func c(opt PrintOptions, code string) string {
	if !opt.UseColor {
		return ""
	}
	return code
}

func nodeLabel(opt PrintOptions, s string) string {
	return c(opt, blue) + s + c(opt, reset)
}

func field(opt PrintOptions, s string) string {
	return c(opt, cyan) + s + c(opt, reset)
}

func literal(opt PrintOptions, s string) string {
	return c(opt, green) + s + c(opt, reset)
}

func keyword(opt PrintOptions, s string) string {
	return c(opt, yellow) + s + c(opt, reset)
}

func PrintAST(w io.Writer, node any) {
	useColor := false
	if f, ok := w.(*os.File); ok {
		useColor = term.IsTerminal(int(f.Fd()))
	}

	opts := PrintOptions{UseColor: useColor}

	printAST(w, node, 0, opts)
}

func printAST(w io.Writer, node any, indent int, opts PrintOptions) {
	prefix := strings.Repeat(" ", indent)

	switch n := node.(type) {

	case *parser.Module:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "Module:"))
		for _, stmt := range n.Body {
			printAST(w, stmt, indent+2, opts)
		}

	case *parser.Assign:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "Assign:"))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Targets:"))
		for _, t := range n.Targets {
			printAST(w, t, indent+4, opts)
		}
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Value:"))
		printAST(w, n.Value, indent+4, opts)

	case *parser.BinOp:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "BinOp:"))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Left:"))
		printAST(w, n.Left, indent+4, opts)
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Op:"))
		printAST(w, n.Op, indent+4, opts)
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Right:"))
		printAST(w, n.Right, indent+4, opts)

	case *parser.Name:
		fmt.Fprintf(w, "%s%s\n", prefix, literal(opts, "Name("+n.ID+")"))

	case *parser.Number:
		fmt.Fprintf(w, "%s%s\n", prefix, literal(opts, "Number("+n.Value+")"))

	case parser.Operator:
		fmt.Fprintf(w, "%s%s\n", prefix, keyword(opts, operatorString(n)))

	case *parser.String:
		fmt.Fprintf(w, "%s%s\n", prefix, literal(opts, `String("`+n.Value+`")`))

	case *parser.Boolean:
		fmt.Fprintf(w, "%s%s\n", prefix, literal(opts, fmt.Sprintf("Boolean(%t)", n.Value)))

	case *parser.Call:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "Call:"))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Func:"))
		printAST(w, n.Func, indent+4, opts)
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Args:"))
		for _, a := range n.Args {
			printAST(w, a, indent+4, opts)
		}

	case *parser.ExprStmt:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "ExprStmt:"))
		printAST(w, n.Value, indent+2, opts)

	case parser.FuncArg:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "FuncArg("+n.Name+")"))
		if n.Default != nil {
			fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Default:"))
			printAST(w, n.Default, indent+4, opts)
		}

	case *parser.FunctionDef:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "FunctionDef("+n.Name+"):"))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Args:"))
		for _, a := range n.Args {
			printAST(w, a, indent+4, opts)
		}
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Body:"))
		for _, s := range n.Body {
			printAST(w, s, indent+4, opts)
		}

	case *parser.Return:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "Return:"))
		printAST(w, n.Value, indent+2, opts)

	case *parser.If:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "If:"))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Test:"))
		printAST(w, n.Test, indent+4, opts)
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Body:"))
		for _, s := range n.Body {
			printAST(w, s, indent+4, opts)
		}
		if len(n.Orelse) > 0 {
			fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Else:"))
			for _, s := range n.Orelse {
				printAST(w, s, indent+4, opts)
			}
		}

	case *parser.WhileLoop:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "While:"))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Test:"))
		printAST(w, n.Test, indent+4, opts)
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Body:"))
		for _, s := range n.Body {
			printAST(w, s, indent+4, opts)
		}

	case *parser.For:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "For:"))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Target:"))
		printAST(w, n.Target, indent+4, opts)
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Iter:"))
		printAST(w, n.Iter, indent+4, opts)
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Body:"))
		for _, s := range n.Body {
			printAST(w, s, indent+4, opts)
		}

	case *parser.List:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "List:"))
		for _, e := range n.Elts {
			printAST(w, e, indent+2, opts)
		}

	case *parser.Tuple:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "Tuple:"))
		for _, e := range n.Elts {
			printAST(w, e, indent+2, opts)
		}

	case *parser.BooleanOp:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "BooleanOp:"))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Operator:"))
		switch n.Operator {
		case parser.And:
			fmt.Fprintf(w, "%s    %s\n", prefix, keyword(opts, "And"))
		case parser.Or:
			fmt.Fprintf(w, "%s    %s\n", prefix, keyword(opts, "Or"))
		}
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Values:"))
		for _, v := range n.Values {
			printAST(w, v, indent+4, opts)
		}

	case parser.CompareOp:
		fmt.Fprintf(w, "%s%s\n", prefix, keyword(opts, "CompareOp("+compareOpString(n)+")"))

	case *parser.Compare:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "Compare:"))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Left:"))
		printAST(w, n.Left, indent+4, opts)
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Ops:"))
		for _, op := range n.Ops {
			printAST(w, op, indent+4, opts)
		}
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Right:"))
		for _, r := range n.Right {
			printAST(w, r, indent+4, opts)
		}

	case *parser.Break:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "Break"))

	case *parser.Continue:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "Continue"))

	default:
		fmt.Fprintf(w, "%sUnknown(%T)\n", prefix, node)
	}
}

func operatorString(op parser.Operator) string {
	switch op {
	case parser.Add:
		return "+"
	case parser.Sub:
		return "-"
	case parser.Mult:
		return "*"
	case parser.Div:
		return "/"
	case parser.FloorDiv:
		return "//"
	case parser.Mod:
		return "%"
	default:
		return "<?>"
	}
}

func compareOpString(op parser.CompareOp) string {
	switch op {
	case parser.Eq:
		return "=="
	case parser.NotEq:
		return "!="
	case parser.Lt:
		return "<"
	case parser.LtE:
		return "<="
	case parser.Gt:
		return ">"
	case parser.GtE:
		return ">="
	default:
		return "<?>"
	}
}
