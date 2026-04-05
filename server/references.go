package server

import (
	"fmt"
	"sort"

	a "rahu/analyser"
	"rahu/jsonrpc"
	"rahu/lsp"
	ast "rahu/parser/ast"
)

type SymbolRefKey struct {
	URI  lsp.DocumentURI
	Span ast.Range
	Name string
	Kind a.SymbolKind
}

func symbolRefKey(sym *a.Symbol) (SymbolRefKey, bool) {
	if sym == nil || sym.URI == "" || sym.Span.IsEmpty() {
		return SymbolRefKey{}, false
	}
	if sym.Kind == a.SymBuiltin || sym.Kind == a.SymImport {
		return SymbolRefKey{}, false
	}
	return SymbolRefKey{URI: sym.URI, Span: sym.Span, Name: sym.Name, Kind: sym.Kind}, true
}

func locationKey(loc lsp.Location) string {
	return fmt.Sprintf("%s:%d:%d:%d:%d", loc.URI, loc.Range.Start.Line, loc.Range.Start.Character, loc.Range.End.Line, loc.Range.End.Character)
}

func collectDocumentReferences(doc *Document, key SymbolRefKey, includeDecl bool) []lsp.Location {
	if doc == nil || doc.Tree == nil || doc.LineIndex == nil {
		return nil
	}
	results := make([]lsp.Location, 0)
	declRange := ToRange(doc.LineIndex, key.Span)
	for nodeID, sym := range doc.Symbols {
		candidate, ok := symbolRefKey(sym)
		if !ok || candidate != key {
			continue
		}
		loc := lsp.Location{URI: doc.URI, Range: ToRange(doc.LineIndex, doc.Tree.RangeOf(nodeID))}
		if !includeDecl && loc.URI == key.URI && loc.Range == declRange {
			continue
		}
		results = append(results, loc)
	}
	for nodeID, sym := range doc.Defs {
		candidate, ok := symbolRefKey(sym)
		if !ok || candidate != key {
			continue
		}
		loc := lsp.Location{URI: doc.URI, Range: ToRange(doc.LineIndex, doc.Tree.RangeOf(nodeID))}
		if !includeDecl && loc.URI == key.URI && loc.Range == declRange {
			continue
		}
		results = append(results, loc)
	}
	if includeDecl && key.URI == doc.URI {
		results = append(results, lsp.Location{URI: doc.URI, Range: declRange})
	}
	return results
}

func collectSnapshotReferences(snapshot *ModuleSnapshot, key SymbolRefKey, includeDecl bool) []lsp.Location {
	if snapshot == nil || snapshot.Tree == nil || snapshot.LineIndex == nil {
		return nil
	}
	results := make([]lsp.Location, 0)
	declRange := ToRange(snapshot.LineIndex, key.Span)
	for nodeID, sym := range snapshot.Symbols {
		candidate, ok := symbolRefKey(sym)
		if !ok || candidate != key {
			continue
		}
		loc := lsp.Location{URI: snapshot.URI, Range: ToRange(snapshot.LineIndex, snapshot.Tree.RangeOf(nodeID))}
		if !includeDecl && loc.URI == key.URI && loc.Range == declRange {
			continue
		}
		results = append(results, loc)
	}
	for nodeID, sym := range snapshot.Defs {
		candidate, ok := symbolRefKey(sym)
		if !ok || candidate != key {
			continue
		}
		loc := lsp.Location{URI: snapshot.URI, Range: ToRange(snapshot.LineIndex, snapshot.Tree.RangeOf(nodeID))}
		if !includeDecl && loc.URI == key.URI && loc.Range == declRange {
			continue
		}
		results = append(results, loc)
	}
	if includeDecl && key.URI == snapshot.URI {
		results = append(results, lsp.Location{URI: snapshot.URI, Range: declRange})
	}
	return results
}

func (s *Server) References(p *lsp.ReferenceParams) ([]lsp.Location, *jsonrpc.Error) {
	doc := s.Get(p.TextDocument.URI)
	if doc == nil {
		return nil, jsonrpc.InvalidParamsError(nil)
	}

	offset := doc.LineIndex.PositionToOffset(p.Position.Line, p.Position.Character)
	sym, _, isAttr := symbolAtOffset(doc, offset)
	if isAttr {
		return []lsp.Location{}, nil
	}
	key, ok := symbolRefKey(sym)
	if !ok {
		return []lsp.Location{}, nil
	}

	results := make([]lsp.Location, 0)
	seen := make(map[string]struct{})
	appendUnique := func(locs []lsp.Location) {
		for _, loc := range locs {
			k := locationKey(loc)
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			results = append(results, loc)
		}
	}

	s.mu.RLock()
	openDocs := make([]*Document, 0, len(s.docs))
	openURIs := make(map[lsp.DocumentURI]struct{}, len(s.docs))
	for uri, openDoc := range s.docs {
		openDocs = append(openDocs, openDoc)
		openURIs[uri] = struct{}{}
	}
	snapshots := make([]*ModuleSnapshot, 0, len(s.moduleSnapshotsByURI))
	for uri, snapshot := range s.moduleSnapshotsByURI {
		if _, open := openURIs[uri]; open {
			continue
		}
		snapshots = append(snapshots, snapshot)
	}
	s.mu.RUnlock()

	for _, openDoc := range openDocs {
		appendUnique(collectDocumentReferences(openDoc, key, p.Context.IncludeDeclaration))
	}
	for _, snapshot := range snapshots {
		appendUnique(collectSnapshotReferences(snapshot, key, p.Context.IncludeDeclaration))
	}

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
