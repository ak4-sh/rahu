package server

import (
	"rahu/jsonrpc"
	"rahu/lsp"
	ast "rahu/parser/ast"
)

func documentSymbolKind(kind ast.NodeKind, method bool) lsp.SymbolKind {
	switch kind {
	case ast.NodeClassDef:
		return lsp.SymbolKindClass
	case ast.NodeFunctionDef:
		if method {
			return lsp.SymbolKindMethod
		}
		return lsp.SymbolKindFunction
	default:
		return lsp.SymbolKindVariable
	}
}

func assignmentSymbols(doc *Document, stmt ast.NodeID, target ast.NodeID) []lsp.DocumentSymbol {
	if target == ast.NoNode || doc == nil || doc.Tree == nil || doc.LineIndex == nil {
		return nil
	}

	switch doc.Tree.Node(target).Kind {
	case ast.NodeName:
		name, ok := doc.Tree.NameText(target)
		if !ok {
			return nil
		}
		return []lsp.DocumentSymbol{{
			Name:           name,
			Kind:           lsp.SymbolKindVariable,
			Range:          ToRange(doc.LineIndex, doc.Tree.RangeOf(stmt)),
			SelectionRange: ToRange(doc.LineIndex, doc.Tree.RangeOf(target)),
		}}
	case ast.NodeTuple, ast.NodeList:
		var out []lsp.DocumentSymbol
		for child := doc.Tree.Node(target).FirstChild; child != ast.NoNode; child = doc.Tree.Node(child).NextSibling {
			out = append(out, assignmentSymbols(doc, stmt, child)...)
		}
		return out
	default:
		return nil
	}
}

func functionSymbol(doc *Document, stmt ast.NodeID, method bool) (lsp.DocumentSymbol, bool) {
	nameID, _, _ := doc.Tree.FunctionParts(stmt)
	name, ok := doc.Tree.NameText(nameID)
	if !ok {
		return lsp.DocumentSymbol{}, false
	}
	return lsp.DocumentSymbol{
		Name:           name,
		Kind:           documentSymbolKind(ast.NodeFunctionDef, method),
		Range:          ToRange(doc.LineIndex, doc.Tree.RangeOf(stmt)),
		SelectionRange: ToRange(doc.LineIndex, doc.Tree.RangeOf(nameID)),
	}, true
}

func classChildren(doc *Document, body ast.NodeID) []lsp.DocumentSymbol {
	if body == ast.NoNode {
		return nil
	}
	children := make([]lsp.DocumentSymbol, 0)
	for stmt := doc.Tree.Node(body).FirstChild; stmt != ast.NoNode; stmt = doc.Tree.Node(stmt).NextSibling {
		if doc.Tree.Node(stmt).Kind != ast.NodeFunctionDef {
			continue
		}
		if sym, ok := functionSymbol(doc, stmt, true); ok {
			children = append(children, sym)
		}
	}
	return children
}

func classSymbol(doc *Document, stmt ast.NodeID) (lsp.DocumentSymbol, bool) {
	nameID, _, body := doc.Tree.ClassParts(stmt)
	name, ok := doc.Tree.NameText(nameID)
	if !ok {
		return lsp.DocumentSymbol{}, false
	}
	return lsp.DocumentSymbol{
		Name:           name,
		Kind:           lsp.SymbolKindClass,
		Range:          ToRange(doc.LineIndex, doc.Tree.RangeOf(stmt)),
		SelectionRange: ToRange(doc.LineIndex, doc.Tree.RangeOf(nameID)),
		Children:       classChildren(doc, body),
	}, true
}

func buildDocumentSymbols(doc *Document) []lsp.DocumentSymbol {
	if doc == nil || doc.Tree == nil || doc.LineIndex == nil {
		return nil
	}

	result := make([]lsp.DocumentSymbol, 0)
	for stmt := doc.Tree.Node(doc.Tree.Root).FirstChild; stmt != ast.NoNode; stmt = doc.Tree.Node(stmt).NextSibling {
		switch doc.Tree.Node(stmt).Kind {
		case ast.NodeClassDef:
			if sym, ok := classSymbol(doc, stmt); ok {
				result = append(result, sym)
			}
		case ast.NodeFunctionDef:
			if sym, ok := functionSymbol(doc, stmt, false); ok {
				result = append(result, sym)
			}
		case ast.NodeAssign:
			value := doc.Tree.Node(stmt).FirstChild
			for target := ast.NoNode; value != ast.NoNode; {
				target = doc.Tree.Node(value).NextSibling
				if target == ast.NoNode {
					break
				}
				result = append(result, assignmentSymbols(doc, stmt, target)...)
				value = target
			}
		}
	}
	return result
}

func (s *Server) DocumentSymbol(p *lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, *jsonrpc.Error) {
	doc := s.Get(p.TextDocument.URI)
	if doc == nil || doc.Tree == nil {
		return nil, jsonrpc.InvalidParamsError(nil)
	}
	return buildDocumentSymbols(doc), nil
}
