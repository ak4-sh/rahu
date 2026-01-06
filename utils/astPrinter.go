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
	case parser.Mod:
		return "%"
	default:
		return "<?>"
	}
}
