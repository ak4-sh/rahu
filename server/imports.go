package server

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"rahu/analyser"
	"rahu/lsp"
	"rahu/parser"
	ast "rahu/parser/ast"
	"rahu/source"
)

func stampSymbolURIs(uri lsp.DocumentURI, maps ...map[ast.NodeID]*analyser.Symbol) {
	for _, m := range maps {
		for _, sym := range m {
			if sym != nil {
				sym.URI = uri
			}
		}
	}
}

func moduleNameFromExpr(tree *ast.AST, id ast.NodeID) (string, bool) {
	if id == ast.NoNode {
		return "", false
	}

	switch tree.Node(id).Kind {
	case ast.NodeName:
		return tree.NameText(id)
	case ast.NodeAttribute:
		base := tree.ChildAt(id, 0)
		attr := tree.ChildAt(id, 1)
		baseName, ok := moduleNameFromExpr(tree, base)
		if !ok {
			return "", false
		}
		attrName, ok := tree.NameText(attr)
		if !ok {
			return "", false
		}
		return baseName + "." + attrName, true
	default:
		return "", false
	}
}

func parentModuleName(name string) (string, bool) {
	idx := strings.LastIndexByte(name, '.')
	if idx < 0 {
		return "", false
	}
	return name[:idx], true
}

func currentPackageName(mod ModuleFile) (string, bool) {
	if mod.Name == "" {
		return "", false
	}
	if filepath.Base(mod.Path) == "__init__.py" {
		return mod.Name, true
	}
	if pkg, ok := parentModuleName(mod.Name); ok {
		return pkg, true
	}
	return "", false
}

func (s *Server) resolveImportModuleName(importerURI lsp.DocumentURI, tree *ast.AST, module ast.NodeID, depth uint32) (string, bool) {
	if depth == 0 {
		return moduleNameFromExpr(tree, module)
	}

	importer, ok := s.LookupModuleByURI(importerURI)
	if !ok {
		return "", false
	}
	base, ok := currentPackageName(importer)
	if !ok {
		return "", false
	}
	for i := uint32(1); i < depth; i++ {
		base, ok = parentModuleName(base)
		if !ok {
			return "", false
		}
	}
	if module == ast.NoNode {
		return base, true
	}
	suffix, ok := moduleNameFromExpr(tree, module)
	if !ok {
		return "", false
	}
	if base == "" {
		return suffix, true
	}
	return base + "." + suffix, true
}

func firstModuleSegment(name string) string {
	if idx := strings.IndexByte(name, '.'); idx >= 0 {
		return name[:idx]
	}
	return name
}

func importBoundName(tree *ast.AST, target ast.NodeID) ast.NodeID {
	if target == ast.NoNode {
		return ast.NoNode
	}
	if tree.Node(target).Kind == ast.NodeName {
		return target
	}
	if tree.Node(target).Kind != ast.NodeAttribute {
		return ast.NoNode
	}
	return importBoundName(tree, tree.ChildAt(target, 0))
}

func moduleDefSpan(snapshot *ModuleSnapshot) ast.Range {
	if snapshot == nil || snapshot.Tree == nil {
		return ast.Range{}
	}
	return snapshot.Tree.RangeOf(snapshot.Tree.Root)
}

func fromImportModuleSpan(tree *ast.AST, stmt ast.NodeID, module ast.NodeID) ast.Range {
	if tree == nil || stmt == ast.NoNode {
		return ast.Range{}
	}
	start := tree.RangeOf(stmt).Start + uint32(len("from "))
	if module == ast.NoNode {
		return ast.Range{Start: start, End: start + tree.Node(stmt).Data}
	}
	modRange := tree.RangeOf(module)
	if modRange.End < start {
		modRange.Start = start
		return modRange
	}
	modRange.Start = start
	return modRange
}

func cloneImportedSymbol(local, target *analyser.Symbol) {
	if local == nil || target == nil {
		return
	}
	local.Kind = target.Kind
	local.Span = target.Span
	local.URI = target.URI
	local.DocString = target.DocString
	local.Inner = target.Inner
	local.Attrs = target.Attrs
	local.Members = target.Members
	local.Bases = target.Bases
	local.InstanceOf = target.InstanceOf
	local.Inferred = target.Inferred
	local.Returns = target.Returns
	if target.Scope != nil {
		local.Scope = target.Scope
	}
}

func extractExports(global *analyser.Scope) map[string]*analyser.Symbol {
	if global == nil {
		return nil
	}

	exports := make(map[string]*analyser.Symbol, len(global.Symbols))
	for name, sym := range global.Symbols {
		if sym == nil {
			continue
		}
		if sym.Kind == analyser.SymImport && sym.URI == "" && sym.Span.IsEmpty() {
			continue
		}
		exports[name] = sym
	}
	return exports
}

func (s *Server) extractImportsForModule(tree *ast.AST, importerURI lsp.DocumentURI) []string {
	if tree == nil || tree.Root == ast.NoNode {
		return nil
	}

	deps := make([]string, 0, 4)
	add := func(name string) {
		if name == "" || slices.Contains(deps, name) {
			return
		}
		deps = append(deps, name)
	}

	for stmt := tree.Node(tree.Root).FirstChild; stmt != ast.NoNode; stmt = tree.Node(stmt).NextSibling {
		switch tree.Node(stmt).Kind {
		case ast.NodeImport:
			for alias := tree.Node(stmt).FirstChild; alias != ast.NoNode; alias = tree.Node(alias).NextSibling {
				target, _ := tree.AliasParts(alias)
				if name, ok := moduleNameFromExpr(tree, target); ok {
					add(name)
				}
			}
		case ast.NodeFromImport:
			module, _ := tree.FromImportParts(stmt)
			if name, ok := s.resolveImportModuleName(importerURI, tree, module, tree.Node(stmt).Data); ok {
				add(name)
			}
		}
	}

	return deps
}

func (s *Server) buildModuleSnapshot(name string, uri lsp.DocumentURI, path, text string, lineIndex *source.LineIndex) *ModuleSnapshot {
	p := parser.New(text)
	tree := p.Parse()
	global, defs := analyser.BuildScopes(tree, text)
	resolver, semErrs := analyser.Resolve(tree, global)
	stampSymbolURIs(uri, defs, resolver.Resolved, resolver.ResolvedAttr)

	snapshot := &ModuleSnapshot{
		Name:        name,
		URI:         uri,
		Path:        path,
		LineIndex:   lineIndex,
		Tree:        tree,
		ParseErrs:   p.Errors(),
		Symbols:     resolver.Resolved,
		AttrSymbols: resolver.ResolvedAttr,
		Defs:        defs,
		Global:      global,
	}
	snapshot.Exports = extractExports(global)
	snapshot.Imports = s.extractImportsForModule(tree, uri)
	if name != "" {
		s.mu.Lock()
		if !s.buildingModules[name] {
			s.buildingModules[name] = true
		}
		s.moduleSnapshotsByName[name] = snapshot
		s.moduleSnapshotsByURI[uri] = snapshot
		s.moduleImportsByURI[uri] = append([]string(nil), snapshot.Imports...)
		s.mu.Unlock()
	}

	semErrs = append(semErrs, s.bindWorkspaceImports(tree, defs, uri)...)
	snapshot.SemErrs = semErrs
	snapshot.Exports = extractExports(global)
	return snapshot
}

func (s *Server) analyzeModuleByName(name string) (*ModuleSnapshot, bool) {
	if name == "" {
		return nil, false
	}

	s.mu.RLock()
	if snapshot, ok := s.moduleSnapshotsByName[name]; ok {
		s.mu.RUnlock()
		return snapshot, true
	}
	mod, ok := s.modulesByName[name]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}

	return s.analyzeModuleFile(mod)
}

func (s *Server) analyzeModuleFile(mod ModuleFile) (*ModuleSnapshot, bool) {
	s.mu.RLock()
	if snapshot, ok := s.moduleSnapshotsByURI[mod.URI]; ok {
		s.mu.RUnlock()
		return snapshot, true
	}
	openDoc := s.docs[mod.URI]
	s.mu.RUnlock()

	text := ""
	lineIndex := (*source.LineIndex)(nil)
	if openDoc != nil {
		text = openDoc.Text
		lineIndex = openDoc.LineIndex
	} else {
		bytes, err := os.ReadFile(mod.Path)
		if err != nil {
			return nil, false
		}
		text = string(bytes)
		lineIndex = source.NewLineIndex(text)
	}

	snapshot := s.buildModuleSnapshot(mod.Name, mod.URI, mod.Path, text, lineIndex)
	s.storeModuleSnapshot(mod, snapshot)

	return snapshot, true
}

func (s *Server) storeModuleSnapshot(mod ModuleFile, snapshot *ModuleSnapshot) {
	if snapshot == nil {
		return
	}

	s.mu.Lock()
	s.moduleSnapshotsByName[mod.Name] = snapshot
	s.moduleSnapshotsByURI[mod.URI] = snapshot
	s.moduleImportsByURI[mod.URI] = append([]string(nil), snapshot.Imports...)
	delete(s.buildingModules, mod.Name)
	s.mu.Unlock()
}

func (s *Server) rebuildModuleByURI(uri lsp.DocumentURI) (*ModuleSnapshot, bool) {
	mod, ok := s.LookupModuleByURI(uri)
	if !ok {
		return nil, false
	}

	text := ""
	lineIndex := (*source.LineIndex)(nil)
	if openDoc := s.Get(uri); openDoc != nil {
		text = openDoc.Text
		lineIndex = openDoc.LineIndex
	} else {
		bytes, err := os.ReadFile(mod.Path)
		if err != nil {
			return nil, false
		}
		text = string(bytes)
		lineIndex = source.NewLineIndex(text)
	}

	snapshot := s.buildModuleSnapshot(mod.Name, mod.URI, mod.Path, text, lineIndex)
	s.storeModuleSnapshot(mod, snapshot)
	s.rebuildReverseDeps()
	return snapshot, true
}

func lookupExport(snapshot *ModuleSnapshot, name string) (*analyser.Symbol, bool) {
	if snapshot == nil || snapshot.Exports == nil {
		return nil, false
	}
	sym, ok := snapshot.Exports[name]
	return sym, ok
}

func unresolvedModuleError(span ast.Range, name string) analyser.SemanticError {
	return analyser.SemanticError{
		Span: span,
		Msg:  "unresolved module: " + name,
	}
}

func missingImportNameError(span ast.Range, moduleName, name string) analyser.SemanticError {
	return analyser.SemanticError{
		Span: span,
		Msg:  "cannot import name '" + name + "' from '" + moduleName + "'",
	}
}

func (s *Server) bindWorkspaceImports(tree *ast.AST, defs map[ast.NodeID]*analyser.Symbol, importerURI lsp.DocumentURI) []analyser.SemanticError {
	if tree == nil || tree.Root == ast.NoNode {
		return nil
	}

	var errs []analyser.SemanticError

	for stmt := tree.Node(tree.Root).FirstChild; stmt != ast.NoNode; stmt = tree.Node(stmt).NextSibling {
		switch tree.Node(stmt).Kind {
		case ast.NodeImport:
			errs = append(errs, s.bindImportStmt(tree, stmt, defs)...)
		case ast.NodeFromImport:
			errs = append(errs, s.bindFromImportStmt(tree, stmt, defs, importerURI)...)
		}
	}

	return errs
}

func (s *Server) bindImportStmt(tree *ast.AST, stmt ast.NodeID, defs map[ast.NodeID]*analyser.Symbol) []analyser.SemanticError {
	var errs []analyser.SemanticError

	for alias := tree.Node(stmt).FirstChild; alias != ast.NoNode; alias = tree.Node(alias).NextSibling {
		target, asName := tree.AliasParts(alias)
		fullName, ok := moduleNameFromExpr(tree, target)
		if !ok {
			continue
		}

		bound := asName
		moduleToBind := fullName
		if bound == ast.NoNode {
			bound = importBoundName(tree, target)
			moduleToBind = firstModuleSegment(fullName)
		}
		local := defs[bound]
		if local == nil {
			continue
		}
		local.Span = ast.Range{}
		local.URI = ""

		snapshot, ok := s.analyzeModuleByName(moduleToBind)
		if !ok {
			errs = append(errs, unresolvedModuleError(tree.RangeOf(target), moduleToBind))
			continue
		}

		local.Kind = analyser.SymModule
		local.URI = snapshot.URI
		local.Span = moduleDefSpan(snapshot)
		local.DocString = ""
		local.Inner = nil
		local.Attrs = nil
		local.Members = nil
		local.Bases = nil
		local.InstanceOf = nil
	}

	return errs
}

func (s *Server) bindFromImportStmt(tree *ast.AST, stmt ast.NodeID, defs map[ast.NodeID]*analyser.Symbol, importerURI lsp.DocumentURI) []analyser.SemanticError {
	module, aliases := tree.FromImportParts(stmt)
	moduleName, ok := s.resolveImportModuleName(importerURI, tree, module, tree.Node(stmt).Data)
	if !ok {
		return nil
	}

	snapshot, ok := s.analyzeModuleByName(moduleName)
	if !ok {
		return []analyser.SemanticError{unresolvedModuleError(fromImportModuleSpan(tree, stmt, module), moduleName)}
	}

	var errs []analyser.SemanticError

	for _, alias := range aliases {
		target, asName := tree.AliasParts(alias)
		name, ok := tree.NameText(target)
		if !ok {
			continue
		}
		bound := asName
		if bound == ast.NoNode {
			bound = target
		}
		local := defs[bound]
		if local == nil {
			continue
		}
		local.Span = ast.Range{}
		local.URI = ""
		remote, ok := lookupExport(snapshot, name)
		if !ok || remote == nil {
			errs = append(errs, missingImportNameError(tree.RangeOf(target), moduleName, name))
			continue
		}
		cloneImportedSymbol(local, remote)
	}

	return errs
}

func (s *Server) lineIndexForURI(uri lsp.DocumentURI) *source.LineIndex {
	if doc := s.Get(uri); doc != nil {
		return doc.LineIndex
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if snapshot, ok := s.moduleSnapshotsByURI[uri]; ok {
		return snapshot.LineIndex
	}
	return nil
}
