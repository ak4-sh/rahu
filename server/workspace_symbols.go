package server

import (
	"sort"
	"strings"

	"rahu/analyser"
	"rahu/jsonrpc"
	"rahu/lsp"
)

func toLSPSymbolKind(sym *analyser.Symbol) lsp.SymbolKind {
	if sym == nil {
		return lsp.SymbolKindVariable
	}

	switch sym.Kind {
	case analyser.SymModule:
		return lsp.SymbolKindModule
	case analyser.SymClass, analyser.SymType:
		return lsp.SymbolKindClass
	case analyser.SymFunction:
		return lsp.SymbolKindFunction
	case analyser.SymAttr, analyser.SymField:
		return lsp.SymbolKindField
	case analyser.SymConstant:
		return lsp.SymbolKindConstant
	case analyser.SymParameter, analyser.SymVariable:
		return lsp.SymbolKindVariable
	default:
		return lsp.SymbolKindVariable
	}
}

func matchesWorkspaceSymbol(query, name string) bool {
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(name), strings.ToLower(query))
}

func (s *Server) WorkspaceSymbol(p *lsp.WorkspaceSymbolParams) ([]lsp.SymbolInformation, *jsonrpc.Error) {
	// Wait for indexing before searching workspace symbols
	if err := s.WaitForIndexing(); err != nil {
		return []lsp.SymbolInformation{}, nil
	}

	query := ""
	if p != nil {
		query = p.Query
	}

	s.indexMu.RLock()
	mods := make([]ModuleFile, 0, len(s.modulesByName))
	for _, mod := range s.modulesByName {
		mods = append(mods, mod)
	}
	s.indexMu.RUnlock()

	// Estimate capacity: ~5 exports per module on average
	results := make([]lsp.SymbolInformation, 0, len(mods)*5)
	for _, mod := range mods {
		snapshot, ok := s.analyzeModuleFile(mod)
		if !ok {
			continue
		}
		if snapshot == nil || snapshot.Exports == nil {
			continue
		}
		li := s.lineIndexForURI(snapshot.URI)
		if li == nil {
			continue
		}
		for name, sym := range snapshot.Exports {
			if sym == nil || sym.Kind == analyser.SymImport || sym.Span.IsEmpty() {
				continue
			}
			if !matchesWorkspaceSymbol(query, name) {
				continue
			}
			results = append(results, lsp.SymbolInformation{
				Name:          name,
				Kind:          toLSPSymbolKind(sym),
				Location:      lsp.Location{URI: sym.URI, Range: ToRange(li, sym.Span)},
				ContainerName: snapshot.Name,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Name != results[j].Name {
			return results[i].Name < results[j].Name
		}
		if results[i].ContainerName != results[j].ContainerName {
			return results[i].ContainerName < results[j].ContainerName
		}
		return results[i].Location.URI < results[j].Location.URI
	})

	return results, nil
}
