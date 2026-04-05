package server

import (
	"sort"

	a "rahu/analyser"
	"rahu/jsonrpc"
	"rahu/lsp"
	ast "rahu/parser/ast"
)

// SymbolRefKey uniquely identifies a symbol definition for reference lookups.
type SymbolRefKey struct {
	URI  lsp.DocumentURI
	Span ast.Range
	Name string
	Kind a.SymbolKind
}

// symbolRefKey creates a SymbolRefKey from a symbol.
// Returns false if the symbol cannot be used as a reference key
// (e.g., builtins, imports, or symbols without locations).
func symbolRefKey(sym *a.Symbol) (SymbolRefKey, bool) {
	if sym == nil || sym.URI == "" || sym.Span.IsEmpty() {
		return SymbolRefKey{}, false
	}
	if sym.Kind == a.SymBuiltin || sym.Kind == a.SymImport {
		return SymbolRefKey{}, false
	}
	return SymbolRefKey{URI: sym.URI, Span: sym.Span, Name: sym.Name, Kind: sym.Kind}, true
}

func (s *Server) References(p *lsp.ReferenceParams) ([]lsp.Location, *jsonrpc.Error) {
	// Wait for indexing before searching references
	if err := s.WaitForIndexing(); err != nil {
		return []lsp.Location{}, nil
	}

	doc := s.Get(p.TextDocument.URI)
	if doc == nil {
		return nil, jsonrpc.InvalidParamsError(nil)
	}

	// Get symbol at cursor position
	doc.mu.RLock()
	lineIndex := doc.LineIndex
	offset := lineIndex.PositionToOffset(p.Position.Line, p.Position.Character)
	sym, _, _ := symbolAtOffset(doc, offset)
	doc.mu.RUnlock()

	key, ok := symbolRefKey(sym)
	if !ok {
		return []lsp.Location{}, nil
	}

	// Build declaration location for filtering
	declLoc := lsp.Location{}
	if sym != nil && !sym.Span.IsEmpty() {
		li := s.lineIndexForURI(sym.URI)
		if li != nil {
			declLoc = lsp.Location{URI: sym.URI, Range: ToRange(li, sym.Span)}
		}
	}

	// O(1) lookup using the reference index
	results := s.refIndex.Lookup(key, p.Context.IncludeDeclaration, declLoc)

	// Sort results for consistent ordering
	sort.Slice(results, func(i, j int) bool {
		if results[i].URI != results[j].URI {
			return results[i].URI < results[j].URI
		}
		if results[i].Range.Start.Line != results[j].Range.Start.Line {
			return results[i].Range.Start.Line < results[j].Range.Start.Line
		}
		return results[i].Range.Start.Character < results[j].Range.Start.Character
	})

	return results, nil
}
