package locate

import "rahu/parser/ast"

func nameInStmt(tree *ast.AST, stmt ast.NodeID, pos int) ast.NodeID {
	res := locateInStmt(tree, stmt, pos, locateNameOnly)
	if res.Kind == NameResult {
		return res.Node
	}

	return ast.NoNode
}

func locateInStmt(tree *ast.AST, stmt ast.NodeID, pos int, mode locateMode) Result {
	if tree == nil || stmt == ast.NoNode || !nodeContains(tree, stmt, pos) {
		return Result{}
	}

	switch tree.Nodes[stmt].Kind {
	case ast.NodeAssign:
		value := tree.Nodes[stmt].FirstChild
		for target := ast.NoNode; value != ast.NoNode; {
			target = tree.Nodes[value].NextSibling
			if target == ast.NoNode {
				break
			}
			if res := locateInExpr(tree, target, pos, mode); res.Kind != NoResult {
				return res
			}
			value = target
		}
		if value != ast.NoNode {
			return locateInExpr(tree, tree.Nodes[stmt].FirstChild, pos, mode)
		}

	case ast.NodeAnnAssign:
		target, annotation, value := tree.AnnAssignParts(stmt)
		if res := locateInExpr(tree, target, pos, mode); res.Kind != NoResult {
			return res
		}
		if res := locateInExpr(tree, annotation, pos, mode); res.Kind != NoResult {
			return res
		}
		if value != ast.NoNode {
			return locateInExpr(tree, value, pos, mode)
		}

	case ast.NodeClassDef:
		nameID, bases, body := tree.ClassParts(stmt)
		if mode != locateAttrOnly && nodeContains(tree, nameID, pos) {
			return Result{Kind: NameResult, Node: nameID}
		}
		for base := tree.Nodes[bases].FirstChild; base != ast.NoNode; base = tree.Nodes[base].NextSibling {
			if res := locateInExpr(tree, base, pos, mode); res.Kind != NoResult {
				return res
			}
		}
		for inner := tree.Nodes[body].FirstChild; inner != ast.NoNode; inner = tree.Nodes[inner].NextSibling {
			if res := locateInStmt(tree, inner, pos, mode); res.Kind != NoResult {
				return res
			}
		}

	case ast.NodeAugAssign:
		target := tree.Nodes[stmt].FirstChild
		if res := locateInExpr(tree, target, pos, mode); res.Kind != NoResult {
			return res
		}
		value := ast.NoNode
		if target != ast.NoNode {
			value = tree.Nodes[target].NextSibling
		}
		if res := locateInExpr(tree, value, pos, mode); res.Kind != NoResult {
			return res
		}

	case ast.NodeFunctionDef:
		nameID, args, returnAnnotation, body := tree.FunctionPartsWithReturn(stmt)
		if mode != locateAttrOnly && nodeContains(tree, nameID, pos) {
			return Result{Kind: NameResult, Node: nameID}
		}
		if res := locateInExpr(tree, returnAnnotation, pos, mode); res.Kind != NoResult {
			return res
		}
		if args != ast.NoNode {
			for arg := tree.Nodes[args].FirstChild; arg != ast.NoNode; arg = tree.Nodes[arg].NextSibling {
				_, annotation, defaultExpr := tree.ParamParts(arg)
				if res := locateInExpr(tree, annotation, pos, mode); res.Kind != NoResult {
					return res
				}
				if res := locateInExpr(tree, defaultExpr, pos, mode); res.Kind != NoResult {
					return res
				}
			}
		}
		for inner := tree.Nodes[body].FirstChild; inner != ast.NoNode; inner = tree.Nodes[inner].NextSibling {
			if res := locateInStmt(tree, inner, pos, mode); res.Kind != NoResult {
				return res
			}
		}

	case ast.NodeIf:
		test := tree.Nodes[stmt].FirstChild
		if res := locateInExpr(tree, test, pos, mode); res.Kind != NoResult {
			return res
		}
		body := ast.NoNode
		if test != ast.NoNode {
			body = tree.Nodes[test].NextSibling
		}
		for inner := tree.Nodes[body].FirstChild; inner != ast.NoNode; inner = tree.Nodes[inner].NextSibling {
			if res := locateInStmt(tree, inner, pos, mode); res.Kind != NoResult {
				return res
			}
		}
		orelse := ast.NoNode
		if body != ast.NoNode {
			orelse = tree.Nodes[body].NextSibling
		}
		for inner := tree.Nodes[orelse].FirstChild; inner != ast.NoNode; inner = tree.Nodes[inner].NextSibling {
			if res := locateInStmt(tree, inner, pos, mode); res.Kind != NoResult {
				return res
			}
		}

	case ast.NodeWhile:
		test := tree.Nodes[stmt].FirstChild
		if res := locateInExpr(tree, test, pos, mode); res.Kind != NoResult {
			return res
		}
		body := ast.NoNode
		if test != ast.NoNode {
			body = tree.Nodes[test].NextSibling
		}
		for inner := tree.Nodes[body].FirstChild; inner != ast.NoNode; inner = tree.Nodes[inner].NextSibling {
			if res := locateInStmt(tree, inner, pos, mode); res.Kind != NoResult {
				return res
			}
		}

	case ast.NodeFor:
		target := tree.Nodes[stmt].FirstChild
		if res := locateInExpr(tree, target, pos, mode); res.Kind != NoResult {
			return res
		}
		iter := ast.NoNode
		if target != ast.NoNode {
			iter = tree.Nodes[target].NextSibling
		}
		if res := locateInExpr(tree, iter, pos, mode); res.Kind != NoResult {
			return res
		}
		body := ast.NoNode
		if iter != ast.NoNode {
			body = tree.Nodes[iter].NextSibling
		}
		for inner := tree.Nodes[body].FirstChild; inner != ast.NoNode; inner = tree.Nodes[inner].NextSibling {
			if res := locateInStmt(tree, inner, pos, mode); res.Kind != NoResult {
				return res
			}
		}
		orelse := ast.NoNode
		if body != ast.NoNode {
			orelse = tree.Nodes[body].NextSibling
		}
		for inner := tree.Nodes[orelse].FirstChild; inner != ast.NoNode; inner = tree.Nodes[inner].NextSibling {
			if res := locateInStmt(tree, inner, pos, mode); res.Kind != NoResult {
				return res
			}
		}

	case ast.NodeExprStmt, ast.NodeReturn:
		return locateInExpr(tree, tree.Nodes[stmt].FirstChild, pos, mode)

	case ast.NodeImport:
		for alias := tree.Nodes[stmt].FirstChild; alias != ast.NoNode; alias = tree.Nodes[alias].NextSibling {
			target, asName := tree.AliasParts(alias)
			if res := locateInExpr(tree, target, pos, mode); res.Kind != NoResult {
				return res
			}
			if mode != locateAttrOnly && nodeContains(tree, asName, pos) {
				return Result{Kind: NameResult, Node: asName}
			}
		}

	case ast.NodeFromImport:
		module, aliases := tree.FromImportParts(stmt)
		if module != ast.NoNode {
			if res := locateInExpr(tree, module, pos, mode); res.Kind != NoResult {
				return res
			}
		}
		for _, alias := range aliases {
			target, asName := tree.AliasParts(alias)
			if res := locateInExpr(tree, target, pos, mode); res.Kind != NoResult {
				return res
			}
			if mode != locateAttrOnly && nodeContains(tree, asName, pos) {
				return Result{Kind: NameResult, Node: asName}
			}
		}
	}

	return Result{}
}
