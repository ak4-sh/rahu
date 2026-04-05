// Package locate contains all locating functionality to locate and navigate to
// statements and expressions in the current doc.
package locate

import "rahu/parser/ast"

type ResultKind uint8

const (
	NoResult ResultKind = iota
	NameResult
	AttributeResult
)

type Result struct {
	Kind ResultKind
	Node ast.NodeID
}

type locateMode uint8

const (
	locateAny locateMode = iota
	locateNameOnly
	locateAttrOnly
)

func LocateAtPos(tree *ast.AST, pos int) Result {
	return locateAtPos(tree, pos, locateAny)
}

func NameAtPos(tree *ast.AST, pos int) ast.NodeID {
	res := locateAtPos(tree, pos, locateNameOnly)
	if res.Kind == NameResult {
		return res.Node
	}
	return ast.NoNode
}

func AttributeAtPos(tree *ast.AST, pos int) ast.NodeID {
	res := locateAtPos(tree, pos, locateAttrOnly)
	if res.Kind == AttributeResult {
		return res.Node
	}

	return ast.NoNode
}

func locateAtPos(tree *ast.AST, pos int, mode locateMode) Result {
	if tree == nil || tree.Root == ast.NoNode {
		return Result{}
	}

	for stmt := tree.Nodes[tree.Root].FirstChild; stmt != ast.NoNode; stmt = tree.Nodes[stmt].NextSibling {
		if res := locateInStmt(tree, stmt, pos, mode); res.Kind != NoResult {
			return res
		}
	}

	return Result{}
}

func Contains(rng ast.Range, pos int) bool {
	return pos >= int(rng.Start) && pos <= int(rng.End)
}

func contains(rng ast.Range, pos int) bool {
	return Contains(rng, pos)
}

func nodeContains(tree *ast.AST, id ast.NodeID, pos int) bool {
	if tree == nil || id == ast.NoNode {
		return false
	}

	n := tree.Nodes[id]
	return pos >= int(n.Start) && pos <= int(n.End)
}
