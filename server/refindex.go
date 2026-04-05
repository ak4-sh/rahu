package server

import (
	"sync"

	"rahu/analyser"
	"rahu/lsp"
	"rahu/parser/ast"
	"rahu/source"
)

func referenceRangeForNode(tree *ast.AST, nodeID ast.NodeID, isAttr bool) ast.Range {
	if tree == nil || nodeID == ast.NoNode {
		return ast.Range{}
	}
	if !isAttr {
		return tree.RangeOf(nodeID)
	}
	node := tree.Node(nodeID)
	base := node.FirstChild
	if base == ast.NoNode {
		return tree.RangeOf(nodeID)
	}
	attrName := tree.Node(base).NextSibling
	if attrName == ast.NoNode {
		return tree.RangeOf(nodeID)
	}
	return tree.RangeOf(attrName)
}

// RefIndex maintains an inverse mapping from symbol definitions
// to all locations where they are referenced. This enables O(1)
// reference lookups instead of scanning all documents.
type RefIndex struct {
	mu sync.RWMutex

	// Primary index: symbol key -> list of reference locations
	refs map[SymbolRefKey][]lsp.Location

	// Reverse lookup: URI -> set of symbol keys referenced in that file
	// Used for efficient removal when a document changes
	byURI map[lsp.DocumentURI]map[SymbolRefKey]struct{}
}

// NewRefIndex creates a new reference index.
func NewRefIndex() *RefIndex {
	return &RefIndex{
		refs:  make(map[SymbolRefKey][]lsp.Location),
		byURI: make(map[lsp.DocumentURI]map[SymbolRefKey]struct{}),
	}
}

// IndexDocument adds or updates all references from a document in the index.
// This should be called after document analysis completes.
func (ri *RefIndex) IndexDocument(
	uri lsp.DocumentURI,
	tree *ast.AST,
	lineIndex *source.LineIndex,
	symbols map[ast.NodeID]*analyser.Symbol,
	attrSymbols map[ast.NodeID]*analyser.Symbol,
	defs map[ast.NodeID]*analyser.Symbol,
) {
	if tree == nil || lineIndex == nil {
		return
	}

	ri.mu.Lock()
	defer ri.mu.Unlock()

	// Step 1: Remove old entries for this URI
	ri.removeURILocked(uri)

	// Step 2: Build new entries
	// Track which nodeIDs we've already processed to avoid duplicates
	// (a node might appear in both symbols and defs)
	keysInDoc := make(map[SymbolRefKey]struct{})
	seenNodes := make(map[ast.NodeID]struct{})

	// Index symbol references
	for nodeID, sym := range symbols {
		key, ok := symbolRefKey(sym)
		if !ok {
			continue
		}
		seenNodes[nodeID] = struct{}{}
		loc := lsp.Location{
			URI:   uri,
			Range: ToRange(lineIndex, referenceRangeForNode(tree, nodeID, false)),
		}
		ri.refs[key] = append(ri.refs[key], loc)
		keysInDoc[key] = struct{}{}
	}

	// Index directly resolved attribute references.
	for nodeID, sym := range attrSymbols {
		if _, seen := seenNodes[nodeID]; seen {
			continue
		}
		if sym == nil || sym.Kind != analyser.SymAttr {
			continue
		}
		key, ok := symbolRefKey(sym)
		if !ok {
			continue
		}
		seenNodes[nodeID] = struct{}{}
		loc := lsp.Location{
			URI:   uri,
			Range: ToRange(lineIndex, referenceRangeForNode(tree, nodeID, true)),
		}
		ri.refs[key] = append(ri.refs[key], loc)
		keysInDoc[key] = struct{}{}
	}

	// Index definitions (for "include declaration" option)
	// Skip nodes already seen in symbols to avoid duplicates
	for nodeID, sym := range defs {
		if _, seen := seenNodes[nodeID]; seen {
			continue
		}
		key, ok := symbolRefKey(sym)
		if !ok {
			continue
		}
		loc := lsp.Location{
			URI:   uri,
			Range: ToRange(lineIndex, referenceRangeForNode(tree, nodeID, false)),
		}
		ri.refs[key] = append(ri.refs[key], loc)
		keysInDoc[key] = struct{}{}
	}

	// Step 3: Store reverse lookup
	if len(keysInDoc) > 0 {
		ri.byURI[uri] = keysInDoc
	}
}

// RemoveDocument removes all references from a document from the index.
func (ri *RefIndex) RemoveDocument(uri lsp.DocumentURI) {
	ri.mu.Lock()
	defer ri.mu.Unlock()
	ri.removeURILocked(uri)
}

// removeURILocked removes all references for a URI. Caller must hold mu.
func (ri *RefIndex) removeURILocked(uri lsp.DocumentURI) {
	keys, ok := ri.byURI[uri]
	if !ok {
		return
	}

	// Remove all references from this URI
	for key := range keys {
		locs := ri.refs[key]
		filtered := locs[:0] // reuse backing array
		for _, loc := range locs {
			if loc.URI != uri {
				filtered = append(filtered, loc)
			}
		}
		if len(filtered) == 0 {
			delete(ri.refs, key)
		} else {
			ri.refs[key] = filtered
		}
	}

	delete(ri.byURI, uri)
}

// Lookup returns all locations referencing the given symbol.
// If includeDecl is true, the declaration location is included in results.
// declLoc is the location of the symbol's declaration.
func (ri *RefIndex) Lookup(key SymbolRefKey, includeDecl bool, declLoc lsp.Location) []lsp.Location {
	ri.mu.RLock()
	locs := ri.refs[key]

	// Copy to avoid holding lock and to filter
	result := make([]lsp.Location, 0, len(locs)+1)
	seen := make(map[lsp.Location]struct{}, len(locs)+1)

	// Track if we've seen the declaration location in the refs
	seenDecl := false

	for _, loc := range locs {
		if loc.URI == declLoc.URI && loc.Range == declLoc.Range {
			seenDecl = true
			if !includeDecl {
				continue
			}
		}
		if _, ok := seen[loc]; ok {
			continue
		}
		seen[loc] = struct{}{}
		result = append(result, loc)
	}
	ri.mu.RUnlock()

	// If includeDecl is true and we haven't seen the declaration in refs,
	// add it explicitly. This handles cross-file cases where the declaration
	// might be in a different file than the references.
	if includeDecl && !seenDecl && declLoc.URI != "" {
		if _, ok := seen[declLoc]; !ok {
			result = append(result, declLoc)
		}
	}

	return result
}

// Clear removes all entries from the index.
func (ri *RefIndex) Clear() {
	ri.mu.Lock()
	defer ri.mu.Unlock()
	ri.refs = make(map[SymbolRefKey][]lsp.Location)
	ri.byURI = make(map[lsp.DocumentURI]map[SymbolRefKey]struct{})
}
