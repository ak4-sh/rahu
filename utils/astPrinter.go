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

	"rahu/parser/ast"
)

const (
	reset  = "\033[0m"
	blue   = "\033[34m"
	cyan   = "\033[36m"
	green  = "\033[32m"
	yellow = "\033[33m"
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

func PrintAST(w io.Writer, tree *ast.AST) {
	useColor := false
	if f, ok := w.(*os.File); ok {
		useColor = term.IsTerminal(int(f.Fd()))
	}

	printNode(w, tree, tree.Root, 0, PrintOptions{UseColor: useColor})
}

func printNode(w io.Writer, tree *ast.AST, id ast.NodeID, indent int, opts PrintOptions) {
	if tree == nil || id == ast.NoNode {
		return
	}

	prefix := strings.Repeat(" ", indent)
	node := tree.Node(id)

	switch node.Kind {
	case ast.NodeModule:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "Module:"))
		for _, child := range tree.Children(id) {
			printNode(w, tree, child, indent+2, opts)
		}

	case ast.NodeAssign:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "Assign:"))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Targets:"))
		kids := tree.Children(id)
		for _, target := range kids[1:] {
			printNode(w, tree, target, indent+4, opts)
		}
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Value:"))
		if len(kids) > 0 {
			printNode(w, tree, kids[0], indent+4, opts)
		}

	case ast.NodeAnnAssign:
		target, annotation, value := tree.AnnAssignParts(id)
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "AnnAssign:"))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Target:"))
		printNode(w, tree, target, indent+4, opts)
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Annotation:"))
		printNode(w, tree, annotation, indent+4, opts)
		if value != ast.NoNode {
			fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Value:"))
			printNode(w, tree, value, indent+4, opts)
		}

	case ast.NodeAugAssign:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "AugAssign:"))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Target:"))
		printNode(w, tree, tree.ChildAt(id, 0), indent+4, opts)
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Op:"))
		fmt.Fprintf(w, "%s    %s\n", prefix, keyword(opts, augAssignString(ast.AugAssignOp(node.Data))))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Value:"))
		printNode(w, tree, tree.ChildAt(id, 1), indent+4, opts)

	case ast.NodeBinOp:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "BinOp:"))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Left:"))
		printNode(w, tree, tree.ChildAt(id, 0), indent+4, opts)
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Op:"))
		fmt.Fprintf(w, "%s    %s\n", prefix, keyword(opts, operatorString(ast.Operator(node.Data))))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Right:"))
		printNode(w, tree, tree.ChildAt(id, 1), indent+4, opts)

	case ast.NodeUnaryOp:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "UnaryOp:"))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Op:"))
		fmt.Fprintf(w, "%s    %s\n", prefix, keyword(opts, unaryOpString(ast.UnaryOperator(node.Data))))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Operand:"))
		printNode(w, tree, tree.ChildAt(id, 0), indent+4, opts)

	case ast.NodeName:
		name, _ := tree.NameText(id)
		fmt.Fprintf(w, "%s%s\n", prefix, literal(opts, "Name("+name+")"))

	case ast.NodeNumber:
		value, _ := tree.NumberText(id)
		fmt.Fprintf(w, "%s%s\n", prefix, literal(opts, "Number("+value+")"))

	case ast.NodeString:
		value, _ := tree.StringText(id)
		fmt.Fprintf(w, "%s%s\n", prefix, literal(opts, `String("`+value+`")`))

	case ast.NodeBoolean:
		fmt.Fprintf(w, "%s%s\n", prefix, literal(opts, "Boolean("+boolString(ast.BooleanVal(node.Data))+")"))

	case ast.NodeNone:
		fmt.Fprintf(w, "%s%s\n", prefix, literal(opts, "None"))

	case ast.NodeCall:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "Call:"))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Func:"))
		printNode(w, tree, tree.ChildAt(id, 0), indent+4, opts)
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Args:"))
		for _, arg := range tree.Children(id)[1:] {
			printNode(w, tree, arg, indent+4, opts)
		}

	case ast.NodeAttribute:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "Attribute:"))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Value:"))
		printNode(w, tree, tree.ChildAt(id, 0), indent+4, opts)
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Attr:"))
		printNode(w, tree, tree.ChildAt(id, 1), indent+4, opts)

	case ast.NodeExprStmt:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "ExprStmt:"))
		printNode(w, tree, tree.ChildAt(id, 0), indent+2, opts)

	case ast.NodeParam:
		nameID, annotation, def := tree.ParamParts(id)
		name, _ := tree.NameText(nameID)
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "Param("+name+")"))
		if annotation != ast.NoNode {
			fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Annotation:"))
			printNode(w, tree, annotation, indent+4, opts)
		}
		if def != ast.NoNode {
			fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Default:"))
			printNode(w, tree, def, indent+4, opts)
		}

	case ast.NodeFunctionDef:
		nameID, args, returnAnnotation, body := tree.FunctionPartsWithReturn(id)
		name, _ := tree.NameText(nameID)
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "FunctionDef("+name+"):"))
		if doc, ok := tree.DocString(id); ok {
			fmt.Fprintf(w, "%s  %s %s\n", prefix, field(opts, "Doc:"), literal(opts, doc))
		}
		if args != ast.NoNode {
			fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Args:"))
			for _, arg := range tree.Children(args) {
				printNode(w, tree, arg, indent+4, opts)
			}
		}
		if returnAnnotation != ast.NoNode {
			fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Returns:"))
			printNode(w, tree, returnAnnotation, indent+4, opts)
		}
		if body != ast.NoNode {
			fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Body:"))
			printNode(w, tree, body, indent+4, opts)
		}

	case ast.NodeReturn:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "Return:"))
		printNode(w, tree, tree.ChildAt(id, 0), indent+2, opts)

	case ast.NodeIf:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "If:"))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Test:"))
		printNode(w, tree, tree.ChildAt(id, 0), indent+4, opts)
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Body:"))
		printNode(w, tree, tree.ChildAt(id, 1), indent+4, opts)
		if elseID := tree.ChildAt(id, 2); elseID != ast.NoNode {
			fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Else:"))
			printNode(w, tree, elseID, indent+4, opts)
		}

	case ast.NodeWhile:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "While:"))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Test:"))
		printNode(w, tree, tree.ChildAt(id, 0), indent+4, opts)
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Body:"))
		printNode(w, tree, tree.ChildAt(id, 1), indent+4, opts)

	case ast.NodeFor:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "For:"))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Target:"))
		printNode(w, tree, tree.ChildAt(id, 0), indent+4, opts)
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Iter:"))
		printNode(w, tree, tree.ChildAt(id, 1), indent+4, opts)
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Body:"))
		printNode(w, tree, tree.ChildAt(id, 2), indent+4, opts)
		if elseID := tree.ChildAt(id, 3); elseID != ast.NoNode {
			fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Else:"))
			printNode(w, tree, elseID, indent+4, opts)
		}

	case ast.NodeList:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "List:"))
		for _, child := range tree.Children(id) {
			printNode(w, tree, child, indent+2, opts)
		}

	case ast.NodeTuple:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "Tuple:"))
		for _, child := range tree.Children(id) {
			printNode(w, tree, child, indent+2, opts)
		}

	case ast.NodeBooleanOp:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "BooleanOp:"))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Operator:"))
		fmt.Fprintf(w, "%s    %s\n", prefix, keyword(opts, booleanOpString(ast.BooleanOperator(node.Data))))
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Values:"))
		for _, child := range tree.Children(id) {
			printNode(w, tree, child, indent+4, opts)
		}

	case ast.NodeCompare:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "Compare:"))
		kids := tree.Children(id)
		if len(kids) == 0 {
			return
		}
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Left:"))
		printNode(w, tree, kids[0], indent+4, opts)
		fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Ops:"))
		for _, op := range kids[1:] {
			fmt.Fprintf(w, "%s    %s\n", prefix, keyword(opts, compareOpString(ast.CompareOp(tree.Node(op).Data))))
			printNode(w, tree, tree.ChildAt(op, 0), indent+6, opts)
		}

	case ast.NodeBlock:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "Block:"))
		for _, child := range tree.Children(id) {
			printNode(w, tree, child, indent+2, opts)
		}

	case ast.NodeArgs:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "Args:"))
		for _, child := range tree.Children(id) {
			printNode(w, tree, child, indent+2, opts)
		}

	case ast.NodeBaseList:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "Bases:"))
		for _, child := range tree.Children(id) {
			printNode(w, tree, child, indent+2, opts)
		}

	case ast.NodeClassDef:
		nameID, bases, body := tree.ClassParts(id)
		name, _ := tree.NameText(nameID)
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "ClassDef("+name+"):"))
		if doc, ok := tree.DocString(id); ok {
			fmt.Fprintf(w, "%s  %s %s\n", prefix, field(opts, "Doc:"), literal(opts, doc))
		}
		if bases != ast.NoNode {
			fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Bases:"))
			printNode(w, tree, bases, indent+4, opts)
		}
		if body != ast.NoNode {
			fmt.Fprintf(w, "%s  %s\n", prefix, field(opts, "Body:"))
			printNode(w, tree, body, indent+4, opts)
		}

	case ast.NodeBreak:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "Break"))

	case ast.NodeContinue:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "Continue"))

	case ast.NodeErrExp:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "ErrExp"))

	case ast.NodeErrStmt:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "ErrStmt"))

	case ast.NodeSubScript:
		fmt.Fprintf(w, "%s%s\n", prefix, nodeLabel(opts, "SubScript:"))
		for _, child := range tree.Children(id) {
			printNode(w, tree, child, indent+2, opts)
		}

	case ast.NodeCompareOp:
		fmt.Fprintf(w, "%s%s\n", prefix, keyword(opts, "CompareOp("+compareOpString(ast.CompareOp(node.Data))+")"))

	default:
		fmt.Fprintf(w, "%sUnknown(%s)\n", prefix, node.Kind)
	}
}

func operatorString(op ast.Operator) string {
	switch op {
	case ast.Add:
		return "+"
	case ast.Sub:
		return "-"
	case ast.Mult:
		return "*"
	case ast.Div:
		return "/"
	case ast.FloorDiv:
		return "//"
	case ast.Mod:
		return "%"
	case ast.Pow:
		return "**"
	default:
		return "<?>"
	}
}

func compareOpString(op ast.CompareOp) string {
	switch op {
	case ast.Eq:
		return "=="
	case ast.NotEq:
		return "!="
	case ast.Lt:
		return "<"
	case ast.LtE:
		return "<="
	case ast.Gt:
		return ">"
	case ast.GtE:
		return ">="
	default:
		return "<?>"
	}
}

func booleanOpString(op ast.BooleanOperator) string {
	switch op {
	case ast.And:
		return "And"
	case ast.Or:
		return "Or"
	default:
		return "<?>"
	}
}

func boolString(v ast.BooleanVal) string {
	switch v {
	case ast.TRUE:
		return "true"
	case ast.FALSE:
		return "false"
	default:
		return "<?>"
	}
}

func unaryOpString(op ast.UnaryOperator) string {
	switch op {
	case ast.UAdd:
		return "+"
	case ast.USub:
		return "-"
	case ast.Not:
		return "not"
	case ast.Increment:
		return "++"
	case ast.Decrement:
		return "--"
	default:
		return "<?>"
	}
}

func augAssignString(op ast.AugAssignOp) string {
	switch op {
	case ast.AugAdd:
		return "+="
	case ast.AugSub:
		return "-="
	case ast.AugMul:
		return "*="
	case ast.AugDiv:
		return "/="
	case ast.AugFloorDiv:
		return "//="
	case ast.AugPow:
		return "**="
	case ast.AugAnd:
		return "&="
	case ast.AugLShift:
		return "<<="
	case ast.AugRShift:
		return ">>="
	case ast.AugMod:
		return "%="
	case ast.AugOr:
		return "|="
	case ast.AugXor:
		return "^="
	case ast.AugMatMul:
		return "@="
	default:
		return "<?>"
	}
}
