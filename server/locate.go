package server

import (
	"rahu/parser"
)

func nameAtPos(module *parser.Module, pos parser.Position) *parser.Name {
	for _, stmt := range module.Body {
		if name := nameInStmt(stmt, pos); name != nil {
			return name
		}
	}
	return nil
}

func nameInStmt(stmt parser.Statement, pos parser.Position) *parser.Name {
	if stmt == nil {
		return nil
	}

	switch v := stmt.(type) {
	case *parser.Assign:
		for _, targ := range v.Targets {
			if name := nameInExpr(targ, pos); name != nil {
				return name
			}
		}

	case *parser.FunctionDef:
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

	case *parser.If:
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

	case *parser.WhileLoop:
		if name := nameInExpr(v.Test, pos); name != nil {
			return name
		}

		for _, stmt := range v.Body {
			if name := nameInStmt(stmt, pos); name != nil {
				return name
			}
		}
		return nil

	}

	return nil
}

func nameInExpr(expr parser.Expression, pos parser.Position) *parser.Name {
	switch e := expr.(type) {
	case *parser.Name:
		if contains(e.Pos, pos) {
			return e
		}

	case *parser.BinOp:
		if name := nameInExpr(e.Left, pos); name != nil {
			return name
		}

		return nameInExpr(e.Right, pos)

	case *parser.Number, *parser.String, *parser.Boolean:
		return nil

	case *parser.Tuple:
		for _, elt := range e.Elts {
			if name := nameInExpr(elt, pos); name != nil {
				return name
			}
		}
		return nil

	case *parser.Call:
		if name := nameInExpr(e.Func, pos); name != nil {
			return name
		}

		for _, arg := range e.Args {
			if name := nameInExpr(arg, pos); name != nil {
				return name
			}
		}
		return nil

	case *parser.Compare:
		if name := nameInExpr(e.Left, pos); name != nil {
			return name
		}

		for _, exprs := range e.Right {
			if name := nameInExpr(exprs, pos); name != nil {
				return name
			}
		}

	case *parser.List:
		for _, elt := range e.Elts {
			if name := nameInExpr(elt, pos); name != nil {
				return name
			}
		}
		return nil

		// TODO: boolean op support, list,
	case *parser.BooleanOp:
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

func contains(rng parser.Range, pos parser.Position) bool {
	return pos.Line >= rng.Start.Line &&
		pos.Line <= rng.End.Line &&
		pos.Col >= rng.Start.Col &&
		pos.Col <= rng.End.Col
}
