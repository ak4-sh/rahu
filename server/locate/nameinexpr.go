package locate

import (
	"rahu/parser/ast"
)

func nameInExpr(expr ast.Expression, pos int) *ast.Name {
	switch e := expr.(type) {
	case *ast.Name:
		if contains(e.Pos, pos) {
			return e
		}

	case *ast.BinOp:
		if name := nameInExpr(e.Left, pos); name != nil {
			return name
		}

		return nameInExpr(e.Right, pos)

	case *ast.Number, *ast.String, *ast.Boolean:
		return nil

	case *ast.Tuple:
		for _, elt := range e.Elts {
			if name := nameInExpr(elt, pos); name != nil {
				return name
			}
		}
		return nil

	case *ast.Call:
		if name := nameInExpr(e.Func, pos); name != nil {
			return name
		}

		for _, arg := range e.Args {
			if name := nameInExpr(arg, pos); name != nil {
				return name
			}
		}
		return nil

	case *ast.Compare:
		if name := nameInExpr(e.Left, pos); name != nil {
			return name
		}

		for _, exprs := range e.Right {
			if name := nameInExpr(exprs, pos); name != nil {
				return name
			}
		}

	case *ast.List:
		for _, elt := range e.Elts {
			if name := nameInExpr(elt, pos); name != nil {
				return name
			}
		}
		return nil

		// TODO: boolean op support, list,
	case *ast.BooleanOp:
		for _, exp := range e.Values {
			if name := nameInExpr(exp, pos); name != nil {
				return name
			}
		}
		return nil

	case *ast.Attribute:
		// check base first
		if name := nameInExpr(e.Value, pos); name != nil {
			return name
		}

		if contains(e.Attr.Pos, pos) {
			return e.Attr
		}

		return nil
	default:
		return nil
	}
	return nil
}
