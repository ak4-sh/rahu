package locate

import a "rahu/parser/ast"

func attributeInStmt(tree *a.AST, stmt a.NodeID, pos int) a.NodeID {
	res := locateInStmt(tree, stmt, pos, locateAttrOnly)
	if res.Kind == AttributeResult {
		return res.Node
	}

	return a.NoNode
}

func attributeInExpr(tree *a.AST, expr a.NodeID, pos int) a.NodeID {
	res := locateInExpr(tree, expr, pos, locateAttrOnly)
	if res.Kind == AttributeResult {
		return res.Node
	}

	return a.NoNode
}
