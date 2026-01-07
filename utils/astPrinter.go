package utils

import (
	"fmt"
	"rahu/parser"
	"strings"
)

func PrintAST(node any, indent int) {
	prefix := strings.Repeat(" ", indent)

	switch n := node.(type) {

	case *parser.Module:
		fmt.Printf("%sModule:\n", prefix)
		for _, stmt := range n.Body {
			PrintAST(stmt, indent+2)
		}

	case *parser.Assign:
		fmt.Printf("%sAssign:\n", prefix)

		fmt.Printf("%s  Targets:\n", prefix)
		for _, target := range n.Targets {
			PrintAST(target, indent+4)
		}

		fmt.Printf("%s  Value:\n", prefix)
		PrintAST(n.Value, indent+4)

	case *parser.BinOp:
		fmt.Printf("%sBinOp:\n", prefix)

		fmt.Printf("%s  Left:\n", prefix)
		PrintAST(n.Left, indent+4)

		fmt.Printf("%s  Op:\n", prefix)
		PrintAST(n.Op, indent+4)

		fmt.Printf("%s  Right:\n", prefix)
		PrintAST(n.Right, indent+4)

	case *parser.Name:
		fmt.Printf("%sName(%s)\n", prefix, n.Id)

	case *parser.Number:
		fmt.Printf("%sNumber(%s)\n", prefix, n.Value)

	case parser.Operator:
		fmt.Printf("%s%s\n", prefix, operatorString(n))

	case *parser.String:
		fmt.Printf("%sString(%s)\n", prefix, n.Value)

	case *parser.Call:
		fmt.Printf("%sCall:\n", prefix)
		fmt.Printf("%s  Func:\n", prefix)
		PrintAST(n.Func, indent + 4)
		if len(n.Args) > 0{
			fmt.Printf("%s  Args:\n", prefix)
			for i := range n.Args{
				PrintAST(n.Args[i], indent + 4)
			}
		} else {
			fmt.Printf("%s  Args: []\n", prefix)
		}

	case *parser.If:
		fmt.Printf("%sIf:\n", prefix)
		fmt.Printf("%s  TestCondition:\n", prefix)
		PrintAST(n.Test, indent + 4)
		fmt.Printf("%s  Body:\n", prefix)
		for i := range n.Body{
			PrintAST(n.Body[i], indent + 4)
		}
		if len(n.Orelse) > 0{
			fmt.Printf("%s  Orelse:\n", prefix)
			for i := range n.Orelse{
				PrintAST(n.Orelse[i], indent + 4)
			}
		} else {
			fmt.Printf("%s Orelse: nil\n", prefix)
		}

	case *parser.ExprStmt:
		fmt.Printf("%sExprStmt:\n", prefix)
		PrintAST(n.Value, indent + 2)
	
	case string:
		fmt.Printf("%s%s\n", prefix, n)

	case *parser.FunctionDef:
		fmt.Printf("%sFunctionDef:\n", prefix)
		fmt.Printf("%s  FuncName (%s)\n", prefix, n.Name)
		fmt.Printf("%s  Args:\n", prefix)
		for i := range n.Args{
			PrintAST(n.Args[i], indent + 4)
		}

		fmt.Printf("%s  Body:\n", prefix)
		for i := range n.Body{
			PrintAST(n.Body[i], indent + 4)
		}

	case *parser.Return:
		fmt.Printf("%sReturn:\n", prefix)
		PrintAST(n.Value, indent + 2)

	
	case parser.CompareOp:
		fmt.Printf("%sCompareOp:\n", prefix)
		switch n{
		case parser.Eq:
			fmt.Printf("%s  Eq()\n", prefix)
		case parser.Gt:
			fmt.Printf("%s Gt()\n", prefix)
		case parser.Lt:
			fmt.Printf("%s Lt()\n", prefix)
		case parser.GtE:
			fmt.Printf("%s GtE()\n", prefix)
		case parser.LtE:
			fmt.Printf("%s LtE()\n", prefix)
		}

	case *parser.Compare:
		fmt.Printf("%sCompare:\n", prefix)
		fmt.Printf("%s  Left:\n", prefix)

		PrintAST(n.Left, indent + 4)

		fmt.Printf("%s  Ops:\n", prefix)
		for i := range n.Ops{
			PrintAST(n.Ops[i], indent + 4)
		}

		fmt.Printf("%s  Right:\n", prefix)

		for i := range n.Right{
			PrintAST(n.Right[i], indent  + 4)
		}


	case *parser.For:
		fmt.Printf("%sFor:\n", prefix)
		fmt.Printf("%s  Target:\n", prefix)
		PrintAST(n.Target, indent + 4)
		fmt.Printf("%s  Iter:\n", prefix)
		PrintAST(n.Iter, indent + 4)
		fmt.Printf("%s  Body:\n", prefix)
		for i := range n.Body{
			PrintAST(n.Body[i], indent + 4)
		}
	default:
		fmt.Printf("%sUnknown(%T)\n", prefix, node)
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
