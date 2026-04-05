package server

import (
	"os"
	"unicode"

	a "rahu/analyser"
	"rahu/jsonrpc"
	"rahu/lsp"
	ast "rahu/parser/ast"
)

type renameTarget struct {
	nodeID ast.NodeID
	name   string
	sym    *a.Symbol
	span   ast.Range
	isAttr bool
}

func validIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func renameSpanAndName(tree *ast.AST, nodeID ast.NodeID, isAttr bool) (ast.Range, string, bool) {
	if tree == nil || nodeID == ast.NoNode {
		return ast.Range{}, "", false
	}
	if !isAttr {
		name, ok := tree.NameText(nodeID)
		if !ok {
			return ast.Range{}, "", false
		}
		return tree.RangeOf(nodeID), name, true
	}
	node := tree.Node(nodeID)
	base := node.FirstChild
	if base == ast.NoNode {
		return ast.Range{}, "", false
	}
	attrName := tree.Node(base).NextSibling
	if attrName == ast.NoNode {
		return ast.Range{}, "", false
	}
	name, ok := tree.NameText(attrName)
	if !ok {
		return ast.Range{}, "", false
	}
	return tree.RangeOf(attrName), name, true
}

func renameTargetAt(doc *Document, pos lsp.Position) (*renameTarget, *jsonrpc.Error) {
	if doc == nil || doc.Tree == nil || doc.LineIndex == nil {
		return nil, jsonrpc.InvalidParamsError(nil)
	}
	offset := doc.LineIndex.PositionToOffset(pos.Line, pos.Character)
	sym, nodeID, isAttr := symbolAtOffset(doc, offset)
	if sym == nil || nodeID == ast.NoNode {
		return nil, jsonrpc.InvalidParamsError(nil)
	}
	if sym.Kind == a.SymBuiltin || sym.Kind == a.SymImport || sym.URI == "" || sym.Span.IsEmpty() {
		return nil, jsonrpc.InvalidParamsError(nil)
	}
	if isAttr && sym.Kind != a.SymAttr {
		return nil, jsonrpc.InvalidParamsError(nil)
	}
	span, oldName, ok := renameSpanAndName(doc.Tree, nodeID, isAttr)
	if !ok || !validIdentifier(oldName) {
		return nil, jsonrpc.InvalidParamsError(nil)
	}
	return &renameTarget{nodeID: nodeID, name: oldName, sym: sym, span: span, isAttr: isAttr}, nil
}

func (s *Server) sourceTextForURI(uri lsp.DocumentURI) (string, bool) {
	if doc := s.Get(uri); doc != nil {
		return doc.Text, true
	}
	mod, ok := s.LookupModuleByURI(uri)
	if !ok {
		return "", false
	}
	bytes, err := os.ReadFile(mod.Path)
	if err != nil {
		return "", false
	}
	return string(bytes), true
}

func (s *Server) textAtLocation(loc lsp.Location) (string, bool) {
	li := s.lineIndexForURI(loc.URI)
	if li == nil {
		return "", false
	}
	text, ok := s.sourceTextForURI(loc.URI)
	if !ok {
		return "", false
	}
	r := FromRange(li, loc.Range)
	if int(r.Start) < 0 || int(r.End) > len(text) || r.Start > r.End {
		return "", false
	}
	return text[r.Start:r.End], true
}

func (s *Server) Rename(p *lsp.RenameParams) (*lsp.WorkspaceEdit, *jsonrpc.Error) {
	if !validIdentifier(p.NewName) {
		return nil, jsonrpc.InvalidParamsError(nil)
	}

	doc := s.Get(p.TextDocument.URI)
	target, targetErr := renameTargetAt(doc, p.Position)
	if targetErr != nil {
		return nil, targetErr
	}

	refs, err := s.References(&lsp.ReferenceParams{
		TextDocument: p.TextDocument,
		Position:     p.Position,
		Context:      lsp.ReferenceContext{IncludeDeclaration: true},
	})
	if err != nil {
		return nil, err
	}

	changes := make(map[lsp.DocumentURI][]lsp.TextEdit)
	for _, ref := range refs {
		text, ok := s.textAtLocation(ref)
		if !ok || text != target.name {
			continue
		}
		changes[ref.URI] = append(changes[ref.URI], lsp.TextEdit{
			Range:   ref.Range,
			NewText: p.NewName,
		})
	}

	if len(changes) == 0 {
		return nil, jsonrpc.InvalidParamsError(nil)
	}

	return &lsp.WorkspaceEdit{Changes: changes}, nil
}

func (s *Server) PrepareRename(p *lsp.PrepareRenameParams) (*lsp.PrepareRenameResult, *jsonrpc.Error) {
	doc := s.Get(p.TextDocument.URI)
	target, targetErr := renameTargetAt(doc, p.Position)
	if targetErr != nil {
		return nil, targetErr
	}
	return &lsp.PrepareRenameResult{
		Range:       ToRange(doc.LineIndex, target.span),
		Placeholder: target.name,
	}, nil
}
