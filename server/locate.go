package server

import (
	"rahu/parser/ast"
)

func nameAtPos(module *ast.Module, pos int) *ast.Name {
	if module == nil {
		return nil
	}
	for _, stmt := range module.Body {
		if name := nameInStmt(stmt, pos); name != nil {
			return name
		}
	}
	return nil
}

func nameInStmt(stmt ast.Statement, pos int) *ast.Name {
	if stmt == nil {
		return nil
	}

	switch v := stmt.(type) {
	case *ast.Assign:
		for _, targ := range v.Targets {
			if name := nameInExpr(targ, pos); name != nil {
				return name
			}
		}
		if name := nameInExpr(v.Value, pos); name != nil {
			return name
		}

	case *ast.AugAssign:
		if name := nameInExpr(v.Target, pos); name != nil {
			return name
		}

		if name := nameInExpr(v.Value, pos); name != nil {
			return name
		}

	case *ast.FunctionDef:
		if contains(v.NamePos, pos) {
			return v.Name
		}
		for _, args := range v.Args {
			if name := nameInExpr(args.Default, pos); name != nil {
				return name
			}
		}

		for _, stmt := range v.Body {
			if name := nameInStmt(stmt, pos); name != nil {
				return name
			}
		}

	case *ast.If:
		if name := nameInExpr(v.Test, pos); name != nil {
			return name
		}

		for _, stmt := range v.Body {
			if name := nameInStmt(stmt, pos); name != nil {
				return name
			}
		}

		for _, stmt := range v.Orelse {
			if name := nameInStmt(stmt, pos); name != nil {
				return name
			}
		}
		return nil

	case *ast.WhileLoop:
		if name := nameInExpr(v.Test, pos); name != nil {
			return name
		}

		for _, stmt := range v.Body {
			if name := nameInStmt(stmt, pos); name != nil {
				return name
			}
		}
		return nil

	case *ast.ExprStmt:
		return nameInExpr(v.Value, pos)

	case *ast.Return:
		if v.Value != nil {
			return nameInExpr(v.Value, pos)
		}
		return nil

	}

	return nil
}

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
	default:
		return nil
	}
	return nil
}

func contains(rng ast.Range, pos int) bool {
	return pos >= rng.Start && pos <= rng.End
}
