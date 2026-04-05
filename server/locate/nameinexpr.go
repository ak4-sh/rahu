package locate

import "rahu/parser/ast"

func nameInExpr(tree *ast.AST, expr ast.NodeID, pos int) ast.NodeID {
	res := locateInExpr(tree, expr, pos, locateNameOnly)
	if res.Kind == NameResult {
		return res.Node
	}

	return ast.NoNode
}

func locateInExpr(tree *ast.AST, expr ast.NodeID, pos int, mode locateMode) Result {
	if tree == nil || expr == ast.NoNode || !nodeContains(tree, expr, pos) {
		return Result{}
	}

	switch tree.Nodes[expr].Kind {
	case ast.NodeName:
		if mode != locateAttrOnly {
			return Result{Kind: NameResult, Node: expr}
		}

	case ast.NodeBinOp:
		left := tree.Nodes[expr].FirstChild
		right := ast.NoNode
		if left != ast.NoNode {
			right = tree.Nodes[left].NextSibling
		}
		if res := locateInExpr(tree, left, pos, mode); res.Kind != NoResult {
			return res
		}
		return locateInExpr(tree, right, pos, mode)

	case ast.NodeNumber, ast.NodeString, ast.NodeBoolean, ast.NodeNone, ast.NodeErrExp:
		return Result{}

	case ast.NodeTuple, ast.NodeList, ast.NodeBooleanOp, ast.NodeCall:
		for child := tree.Nodes[expr].FirstChild; child != ast.NoNode; child = tree.Nodes[child].NextSibling {
			if res := locateInExpr(tree, child, pos, mode); res.Kind != NoResult {
				return res
			}
		}

	case ast.NodeCompare:
		left := tree.Nodes[expr].FirstChild
		if left == ast.NoNode {
			return Result{}
		}
		if res := locateInExpr(tree, left, pos, mode); res.Kind != NoResult {
			return res
		}
		for cmp := tree.Nodes[left].NextSibling; cmp != ast.NoNode; cmp = tree.Nodes[cmp].NextSibling {
			if res := locateInExpr(tree, tree.Nodes[cmp].FirstChild, pos, mode); res.Kind != NoResult {
				return res
			}
		}

	case ast.NodeUnaryOp:
		return locateInExpr(tree, tree.Nodes[expr].FirstChild, pos, mode)

	case ast.NodeAttribute:
		base := tree.Nodes[expr].FirstChild
		attr := ast.NoNode
		if base != ast.NoNode {
			attr = tree.Nodes[base].NextSibling
		}

		if mode != locateNameOnly && nodeContains(tree, attr, pos) {
			return Result{Kind: AttributeResult, Node: expr}
		}
		if res := locateInExpr(tree, base, pos, mode); res.Kind != NoResult {
			return res
		}
		if mode != locateAttrOnly && nodeContains(tree, attr, pos) {
			return Result{Kind: NameResult, Node: attr}
		}
	}

	return Result{}
}
