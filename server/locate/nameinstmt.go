package locate

import (
	"rahu/parser/ast"
)

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

	case *ast.ClassDef:
		// Optional: allow goto-def/hover on class name itself if your AST has a NamePos.
		// If you don't, just delete this block.
		if contains(v.Name.Pos, pos) {
			return v.Name
		}
		for _, base := range v.Bases {
			if base != nil {
				if name := nameInExpr(base, pos); name != nil {
					return name
				}
			}
		}
		for _, st := range v.Body {
			if name := nameInStmt(st, pos); name != nil {
				return name
			}
		}
		return nil

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
