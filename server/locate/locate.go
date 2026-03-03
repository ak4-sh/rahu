// Package locate contains all locating functionality to locate and navigate to
// statements, expressions in the current doc
package locate

import (
	"rahu/parser/ast"
)

func NameAtPos(module *ast.Module, pos int) *ast.Name {
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

func contains(rng ast.Range, pos int) bool {
	return pos >= rng.Start && pos <= rng.End
}
