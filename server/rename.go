package server

import (
	"os"
	"unicode"

	a "rahu/analyser"
	"rahu/jsonrpc"
	"rahu/lsp"
	ast "rahu/parser/ast"
)

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

func renameTargetAt(doc *Document, pos lsp.Position) (ast.NodeID, string, *a.Symbol, *jsonrpc.Error) {
	if doc == nil || doc.Tree == nil || doc.LineIndex == nil {
		return ast.NoNode, "", nil, jsonrpc.InvalidParamsError(nil)
	}
	offset := doc.LineIndex.PositionToOffset(pos.Line, pos.Character)
	sym, nodeID, isAttr := symbolAtOffset(doc, offset)
	if isAttr || sym == nil || nodeID == ast.NoNode {
		return ast.NoNode, "", nil, jsonrpc.InvalidParamsError(nil)
	}
	if sym.Kind == a.SymBuiltin || sym.Kind == a.SymImport || sym.URI == "" || sym.Span.IsEmpty() {
		return ast.NoNode, "", nil, jsonrpc.InvalidParamsError(nil)
	}
	oldName, ok := doc.Tree.NameText(nodeID)
	if !ok || !validIdentifier(oldName) {
		return ast.NoNode, "", nil, jsonrpc.InvalidParamsError(nil)
	}
	return nodeID, oldName, sym, nil
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
	_, oldName, _, targetErr := renameTargetAt(doc, p.Position)
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
		if !ok || text != oldName {
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
	nodeID, oldName, _, targetErr := renameTargetAt(doc, p.Position)
	if targetErr != nil {
		return nil, targetErr
	}
	return &lsp.PrepareRenameResult{
		Range:       ToRange(doc.LineIndex, doc.Tree.RangeOf(nodeID)),
		Placeholder: oldName,
	}, nil
}
