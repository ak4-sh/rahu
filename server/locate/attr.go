package locate

import (
	a "rahu/parser/ast"
)

func AttributeAtPos(m *a.Module, pos int) *a.Attribute {
	if m == nil {
		return nil
	}

	for _, stmt := range m.Body {
		if attr := attributeInStmt(stmt, pos); attr != nil {
			return attr
		}
	}

	return nil
}

func attributeInStmt(stmt a.Statement, pos int) *a.Attribute {
	switch v := stmt.(type) {
	case *a.Assign:
		for _, t := range v.Targets {
			if a := attributeInExpr(t, pos); a != nil {
				return a
			}
		}
		return attributeInExpr(v.Value, pos)

	case *a.AugAssign:
		if a := attributeInExpr(v.Target, pos); a != nil {
			return a
		}
		return attributeInExpr(v.Value, pos)

	case *a.ExprStmt:
		return attributeInExpr(v.Value, pos)

	case *a.Return:
		if v.Value != nil {
			return attributeInExpr(v.Value, pos)
		}

	case *a.FunctionDef:
		for _, stmt := range v.Body {
			if a := attributeInStmt(stmt, pos); a != nil {
				return a
			}
		}

	case *a.If:
		if a := attributeInExpr(v.Test, pos); a != nil {
			return a
		}
		for _, stmt := range v.Body {
			if a := attributeInStmt(stmt, pos); a != nil {
				return a
			}
		}
		for _, stmt := range v.Orelse {
			if a := attributeInStmt(stmt, pos); a != nil {
				return a
			}
		}

	case *a.For:
		if a := attributeInExpr(v.Target, pos); a != nil {
			return a
		}
		if a := attributeInExpr(v.Iter, pos); a != nil {
			return a
		}
		for _, stmt := range v.Body {
			if a := attributeInStmt(stmt, pos); a != nil {
				return a
			}
		}

	case *a.WhileLoop:
		if a := attributeInExpr(v.Test, pos); a != nil {
			return a
		}
		for _, stmt := range v.Body {
			if a := attributeInStmt(stmt, pos); a != nil {
				return a
			}
		}

	case *a.ClassDef:
		for _, stmt := range v.Body {
			if a := attributeInStmt(stmt, pos); a != nil {
				return a
			}
		}
	}
	return nil
}

func attributeInExpr(expr a.Expression, pos int) *a.Attribute {
	switch e := expr.(type) {
	case *a.Attribute:
		if contains(e.Attr.Pos, pos) {
			return e
		}
		return nil
		// return attributeInExpr(e.Value, pos)

	case *a.BinOp:
		if a := attributeInExpr(e.Left, pos); a != nil {
			return a
		}

		return attributeInExpr(e.Right, pos)

	case *a.Call:
		if a := attributeInExpr(e.Func, pos); a != nil {
			return a
		}
		for _, arg := range e.Args {
			if a := attributeInExpr(arg, pos); a != nil {
				return a
			}
		}

	case *a.Tuple:
		for _, elt := range e.Elts {
			if a := attributeInExpr(elt, pos); a != nil {
				return a
			}
		}

	case *a.List:
		for _, elt := range e.Elts {
			if a := attributeInExpr(elt, pos); a != nil {
				return a
			}
		}
	}
	return nil
}
