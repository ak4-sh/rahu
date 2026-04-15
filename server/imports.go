package server

import (
	"hash"
	"hash/fnv"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"rahu/analyser"
	"rahu/lsp"
	"rahu/parser"
	ast "rahu/parser/ast"
	"rahu/source"
)

type moduleSnapshotLookup func(string) (*ModuleSnapshot, bool)

type moduleImportSurfaceLookup func(string) (*ModuleImportSurface, bool)

func computeTextHash(text string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(text))
	return h.Sum64()
}

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
	if before, _, ok := strings.Cut(name, "."); ok {
		return before
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

func importSurfaceDefSpan(surface *ModuleImportSurface) ast.Range {
	if surface == nil || surface.Tree == nil {
		return ast.Range{}
	}
	return surface.Tree.RangeOf(surface.Tree.Root)
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

func cloneSymbolForImport(target *analyser.Symbol) *analyser.Symbol {
	if target == nil {
		return nil
	}
	clone := *target
	clone.Scope = nil
	return &clone
}

func isStarImportAlias(tree *ast.AST, alias ast.NodeID) bool {
	if tree == nil || alias == ast.NoNode {
		return false
	}
	target, _ := tree.AliasParts(alias)
	name, ok := tree.NameText(target)
	return ok && name == "*"
}

func reResolveSnapshot(snapshot *ModuleSnapshot) {
	if snapshot == nil || snapshot.Tree == nil || snapshot.Global == nil {
		return
	}
	resolver, semErrs := analyser.Resolve(snapshot.Tree, snapshot.Global)
	snapshot.Symbols = resolver.Resolved
	snapshot.AttrSymbols = resolver.ResolvedAttr
	snapshot.SemErrs = semErrs
}

func starImportExports(snapshot *ModuleSnapshot) map[string]*analyser.Symbol {
	if snapshot == nil || snapshot.Exports == nil {
		return nil
	}
	if explicit := explicitStarImportExports(snapshot); len(explicit) != 0 {
		return explicit
	}
	exports := make(map[string]*analyser.Symbol)
	for name, sym := range snapshot.Exports {
		if sym == nil || strings.HasPrefix(name, "_") {
			continue
		}
		exports[name] = sym
	}
	return exports
}

func explicitStarImportExports(snapshot *ModuleSnapshot) map[string]*analyser.Symbol {
	if snapshot == nil || snapshot.Tree == nil || snapshot.Exports == nil {
		return nil
	}
	names := staticAllNames(snapshot.Tree)
	if len(names) == 0 {
		return nil
	}
	exports := make(map[string]*analyser.Symbol, len(names))
	for _, name := range names {
		sym, ok := snapshot.Exports[name]
		if !ok || sym == nil {
			continue
		}
		exports[name] = sym
	}
	return exports
}

func staticAllNames(tree *ast.AST) []string {
	if tree == nil || tree.Root == ast.NoNode {
		return nil
	}
	var names []string
	for stmt := tree.Node(tree.Root).FirstChild; stmt != ast.NoNode; stmt = tree.Node(stmt).NextSibling {
		switch tree.Node(stmt).Kind {
		case ast.NodeAssign:
			value := tree.Node(stmt).FirstChild
			for target := tree.Node(value).NextSibling; target != ast.NoNode; target = tree.Node(target).NextSibling {
				name, ok := tree.NameText(target)
				if !ok || name != "__all__" {
					continue
				}
				if explicit := stringSequenceLiteralValues(tree, value); len(explicit) != 0 {
					names = explicit
				}
			}
		case ast.NodeAnnAssign:
			target, _, value := tree.AnnAssignParts(stmt)
			name, ok := tree.NameText(target)
			if !ok || name != "__all__" || value == ast.NoNode {
				continue
			}
			if explicit := stringSequenceLiteralValues(tree, value); len(explicit) != 0 {
				names = explicit
			}
		}
	}
	return names
}

func stringSequenceLiteralValues(tree *ast.AST, id ast.NodeID) []string {
	if tree == nil || id == ast.NoNode {
		return nil
	}
	kind := tree.Node(id).Kind
	if kind != ast.NodeList && kind != ast.NodeTuple {
		return nil
	}
	values := make([]string, 0, tree.ChildCount(id))
	for child := tree.Node(id).FirstChild; child != ast.NoNode; child = tree.Node(child).NextSibling {
		text, ok := tree.StringText(child)
		if !ok {
			return nil
		}
		values = append(values, text)
	}
	return values
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

func (s *Server) augmentExportsFromInterpreter(snapshot *ModuleSnapshot) map[string]*analyser.Symbol {
	if snapshot == nil {
		return nil
	}
	exports := snapshot.Exports
	if exports == nil {
		exports = make(map[string]*analyser.Symbol)
	}
	members, ok := s.pythonModuleMembers(snapshot.Name)
	if !ok {
		return exports
	}
	span := moduleDefSpan(snapshot)
	for _, name := range members {
		if !isValidSyntheticIdentifier(name) {
			continue
		}
		if _, exists := exports[name]; exists {
			continue
		}
		exports[name] = &analyser.Symbol{Name: name, Kind: analyser.SymVariable, URI: snapshot.URI, Span: span}
	}
	return exports
}

func (s *Server) buildBaseModuleSnapshot(name string, uri lsp.DocumentURI, path, text string, lineIndex *source.LineIndex) *ModuleSnapshot {
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
		TextHash:    computeTextHash(text),
		Tree:        tree,
		ParseErrs:   p.Errors(),
		Symbols:     resolver.Resolved,
		AttrSymbols: resolver.ResolvedAttr,
		Defs:        defs,
		SemErrs:     semErrs,
		Global:      global,
	}
	snapshot.Imports = s.extractImportsForModule(tree, uri)
	snapshot.Exports = extractExports(snapshot.Global)
	snapshot.Exports = s.augmentExportsFromInterpreter(snapshot)
	snapshot.ExportHash = computeExportHash(snapshot.Exports)
	snapshot.MemberScope = buildMemberScope(snapshot.Exports)
	return snapshot
}

func (s *Server) buildStartupModuleBase(name string, uri lsp.DocumentURI, path, text string, lineIndex *source.LineIndex) *StartupModuleBase {
	p := parser.New(text)
	tree := p.Parse()
	return &StartupModuleBase{
		Name:      name,
		URI:       uri,
		Path:      path,
		Text:      text,
		TextHash:  computeTextHash(text),
		LineIndex: lineIndex,
		Tree:      tree,
		ParseErrs: p.Errors(),
		Imports:   s.extractImportsForModule(tree, uri),
	}
}

// snapshotMemberScope returns the member scope to use when binding an import
// symbol. Prefers MemberScope (Global + dir() augmentation) over raw Global.
func snapshotMemberScope(snapshot *ModuleSnapshot) *analyser.Scope {
	if snapshot == nil {
		return nil
	}
	if snapshot.MemberScope != nil {
		return snapshot.MemberScope
	}
	return snapshot.Global
}

func importSurfaceMemberScope(surface *ModuleImportSurface) *analyser.Scope {
	if surface == nil {
		return nil
	}
	return surface.MemberScope
}

func buildMemberScope(exports map[string]*analyser.Symbol) *analyser.Scope {
	if len(exports) == 0 {
		return nil
	}
	scope := &analyser.Scope{
		Kind:    analyser.ScopeMember,
		Symbols: make(map[string]*analyser.Symbol, len(exports)),
	}
	maps.Copy(scope.Symbols, exports)
	return scope
}

func (s *Server) buildImportSurfaceFromBase(base *StartupModuleBase) *ModuleImportSurface {
	return s.buildImportSurfaceFromBaseWithLookup(base, nil)
}

func (s *Server) buildImportSurfaceFromBaseWithLookup(base *StartupModuleBase, lookup moduleImportSurfaceLookup) *ModuleImportSurface {
	if base == nil {
		return nil
	}
	global, defs := analyser.BuildScopes(base.Tree, base.Text)
	resolver, _ := analyser.Resolve(base.Tree, global)
	stampSymbolURIs(base.URI, defs, resolver.Resolved, resolver.ResolvedAttr)

	if lookup != nil {
		_ = s.bindWorkspaceImportsWithSurfaceLookup(base.Tree, global, defs, base.URI, lookup)
		tmp := &ModuleSnapshot{Tree: base.Tree, Global: global}
		reResolveSnapshot(tmp)
	}

	tmp := &ModuleSnapshot{
		Name:   base.Name,
		URI:    base.URI,
		Path:   base.Path,
		Tree:   base.Tree,
		Global: global,
	}
	tmp.Exports = extractExports(global)
	tmp.Exports = s.augmentExportsFromInterpreter(tmp)

	return &ModuleImportSurface{
		Name:        base.Name,
		URI:         base.URI,
		Path:        base.Path,
		Tree:        base.Tree,
		Exports:     tmp.Exports,
		MemberScope: buildMemberScope(tmp.Exports),
		ExportHash:  computeExportHash(tmp.Exports),
	}
}

func (s *Server) buildFinalSnapshotFromBase(base *StartupModuleBase, lookup moduleImportSurfaceLookup) *ModuleSnapshot {
	if base == nil {
		return nil
	}
	global, defs := analyser.BuildScopes(base.Tree, base.Text)
	resolver, semErrs := analyser.Resolve(base.Tree, global)
	stampSymbolURIs(base.URI, defs, resolver.Resolved, resolver.ResolvedAttr)

	snapshot := &ModuleSnapshot{
		Name:        base.Name,
		URI:         base.URI,
		Path:        base.Path,
		LineIndex:   base.LineIndex,
		TextHash:    base.TextHash,
		Tree:        base.Tree,
		ParseErrs:   append([]parser.Error(nil), base.ParseErrs...),
		Symbols:     resolver.Resolved,
		AttrSymbols: resolver.ResolvedAttr,
		Defs:        defs,
		SemErrs:     semErrs,
		Global:      global,
		Imports:     append([]string(nil), base.Imports...),
	}

	importErrs := s.bindWorkspaceImportsWithSurfaceLookup(snapshot.Tree, snapshot.Global, snapshot.Defs, snapshot.URI, lookup)
	reResolveSnapshot(snapshot)
	snapshot.SemErrs = append(snapshot.SemErrs, importErrs...)
	snapshot.Exports = extractExports(snapshot.Global)
	snapshot.Exports = s.augmentExportsFromInterpreter(snapshot)
	snapshot.ExportHash = computeExportHash(snapshot.Exports)
	snapshot.MemberScope = buildMemberScope(snapshot.Exports)
	return snapshot
}

func computeExportHash(exports map[string]*analyser.Symbol) uint64 {
	names := make([]string, 0, len(exports))
	for name := range exports {
		names = append(names, name)
	}
	sort.Strings(names)

	h := fnv.New64a()
	visitedSymbols := make(map[analyser.SymbolID]struct{}, len(exports))
	visitedTypes := map[*analyser.Type]struct{}{}
	for _, name := range names {
		writeHashString(h, name)
		writeHashByte(h, 0)
		writeSymbolSignature(h, exports[name], visitedSymbols, visitedTypes)
		writeHashByte(h, 0xff)
	}

	return h.Sum64()
}

func writeSymbolSignature(h hash.Hash64, sym *analyser.Symbol, visitedSymbols map[analyser.SymbolID]struct{}, visitedTypes map[*analyser.Type]struct{}) {
	if sym == nil {
		writeHashString(h, "<nil>")
		return
	}
	if sym.ID != 0 {
		if _, ok := visitedSymbols[sym.ID]; ok {
			writeHashString(h, "<cycle>")
			writeHashByte(h, 0)
			writeHashString(h, sym.Name)
			writeHashByte(h, 0)
			writeHashInt(h, int(sym.Kind))
			return
		}
		visitedSymbols[sym.ID] = struct{}{}
		defer delete(visitedSymbols, sym.ID)
	}

	writeHashString(h, sym.Name)
	writeHashByte(h, 0)
	writeHashInt(h, int(sym.Kind))
	writeHashByte(h, 0)

	switch sym.Kind {
	case analyser.SymFunction:
		writeFunctionSignature(h, sym, visitedSymbols, visitedTypes)
	case analyser.SymClass:
		writeClassSignature(h, sym, visitedSymbols, visitedTypes)
	default:
		writeTypeSignature(h, sym.Inferred, visitedSymbols, visitedTypes)
	}
}

func writeFunctionSignature(h hash.Hash64, sym *analyser.Symbol, visitedSymbols map[analyser.SymbolID]struct{}, visitedTypes map[*analyser.Type]struct{}) {
	if sym == nil || sym.Inner == nil {
		writeHashString(h, "fn")
		writeHashByte(h, 0)
		if sym != nil {
			writeTypeSignature(h, sym.Returns, visitedSymbols, visitedTypes)
		}
		return
	}

	type paramSig struct {
		name  string
		start uint32
		kind  analyser.SymbolKind
		def   string
		typ   *analyser.Type
	}

	params := make([]paramSig, 0, len(sym.Inner.Symbols))
	for _, inner := range sym.Inner.Symbols {
		if inner == nil || inner.Kind != analyser.SymParameter {
			continue
		}
		params = append(params, paramSig{
			name:  inner.Name,
			start: inner.Span.Start,
			kind:  inner.Kind,
			def:   inner.DefaultValue,
			typ:   inner.Inferred,
		})
	}

	sort.Slice(params, func(i, j int) bool {
		if params[i].start != params[j].start {
			return params[i].start < params[j].start
		}
		return params[i].name < params[j].name
	})

	writeHashString(h, "fn")
	writeHashByte(h, 0)
	writeHashInt(h, len(params))
	writeHashByte(h, 0)
	for _, param := range params {
		writeHashString(h, param.name)
		writeHashByte(h, 0)
		writeHashInt(h, int(param.kind))
		writeHashByte(h, 0)
		writeHashString(h, param.def)
		writeHashByte(h, 0)
		writeTypeSignature(h, param.typ, visitedSymbols, visitedTypes)
		writeHashByte(h, 0xfe)
	}
	writeTypeSignature(h, sym.Returns, visitedSymbols, visitedTypes)
}

func writeClassSignature(h hash.Hash64, sym *analyser.Symbol, visitedSymbols map[analyser.SymbolID]struct{}, visitedTypes map[*analyser.Type]struct{}) {
	writeHashString(h, "class")
	writeHashByte(h, 0)
	writeHashInt(h, len(sym.Bases))
	for _, base := range sym.Bases {
		writeHashByte(h, 0)
		if base == nil {
			writeHashString(h, "<nil>")
			continue
		}
		writeHashString(h, base.Name)
	}
	writeScopeSignature(h, sym.Attrs, visitedSymbols, visitedTypes)
	writeScopeSignature(h, sym.Members, visitedSymbols, visitedTypes)
}

func writeScopeSignature(h hash.Hash64, scope *analyser.Scope, visitedSymbols map[analyser.SymbolID]struct{}, visitedTypes map[*analyser.Type]struct{}) {
	if scope == nil || len(scope.Symbols) == 0 {
		writeHashByte(h, 0)
		return
	}

	names := make([]string, 0, len(scope.Symbols))
	for name := range scope.Symbols {
		names = append(names, name)
	}
	sort.Strings(names)
	writeHashInt(h, len(names))
	for _, name := range names {
		writeHashByte(h, 0)
		writeHashString(h, name)
		writeHashByte(h, 0)
		writeSymbolSignature(h, scope.Symbols[name], visitedSymbols, visitedTypes)
	}
}

func writeTypeSignature(h hash.Hash64, typ *analyser.Type, visitedSymbols map[analyser.SymbolID]struct{}, visitedTypes map[*analyser.Type]struct{}) {
	_ = visitedSymbols
	if typ == nil {
		writeHashString(h, "<nil>")
		return
	}
	if _, ok := visitedTypes[typ]; ok {
		writeHashString(h, "<cycle>")
		writeHashByte(h, 0)
		writeHashInt(h, int(typ.Kind))
		return
	}
	visitedTypes[typ] = struct{}{}
	defer delete(visitedTypes, typ)

	writeHashInt(h, int(typ.Kind))
	writeHashByte(h, 0)
	if typ.Symbol != nil {
		writeHashString(h, typ.Symbol.Name)
	}
	writeHashByte(h, 0)
	for _, union := range typ.Union {
		writeTypeSignature(h, union, visitedSymbols, visitedTypes)
		writeHashByte(h, 1)
	}
	writeTypeSignature(h, typ.Elem, visitedSymbols, visitedTypes)
	writeHashByte(h, 2)
	for _, item := range typ.Items {
		writeTypeSignature(h, item, visitedSymbols, visitedTypes)
		writeHashByte(h, 3)
	}
	writeTypeSignature(h, typ.Key, visitedSymbols, visitedTypes)
}

func writeHashString(h hash.Hash64, s string) {
	_, _ = h.Write([]byte(s))
}

func writeHashByte(h hash.Hash64, b byte) {
	_, _ = h.Write([]byte{b})
}

func writeHashInt(h hash.Hash64, n int) {
	writeHashString(h, strconv.Itoa(n))
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
	snapshot := s.buildBaseModuleSnapshot(name, uri, path, text, lineIndex)
	if snapshot == nil {
		return nil
	}
	if name != "" {
		partial := *snapshot
		s.snapshotsMu.Lock()
		s.moduleSnapshotsByName[name] = &partial
		s.moduleSnapshotsByURI[uri] = &partial
		s.snapshotsMu.Unlock()

		s.depsMu.Lock()
		s.moduleImportsByURI[uri] = append([]string(nil), partial.Imports...)
		s.depsMu.Unlock()
	}

	importErrs := s.bindWorkspaceImports(snapshot.Tree, snapshot.Global, snapshot.Defs, uri)
	reResolveSnapshot(snapshot)
	snapshot.SemErrs = append(snapshot.SemErrs, importErrs...)
	snapshot.Exports = extractExports(snapshot.Global)
	snapshot.Exports = s.augmentExportsFromInterpreter(snapshot)
	snapshot.ExportHash = computeExportHash(snapshot.Exports)
	snapshot.MemberScope = buildMemberScope(snapshot.Exports)
	return snapshot
}

func (s *Server) buildBaseSnapshotForModule(mod ModuleFile) (*ModuleSnapshot, bool) {
	text, lineIndex, ok := s.moduleSourceForSnapshot(mod)
	if !ok {
		return nil, false
	}
	return s.buildBaseModuleSnapshot(mod.Name, mod.URI, mod.Path, text, lineIndex), true
}

func (s *Server) buildStartupBaseForModule(mod ModuleFile) (*StartupModuleBase, bool) {
	text, lineIndex, ok := s.moduleSourceForSnapshot(mod)
	if !ok {
		return nil, false
	}
	return s.buildStartupModuleBase(mod.Name, mod.URI, mod.Path, text, lineIndex), true
}

func (s *Server) moduleSourceForSnapshot(mod ModuleFile) (string, *source.LineIndex, bool) {
	// Check for open document
	s.docsMu.RLock()
	openDoc := s.docs[mod.URI]
	s.docsMu.RUnlock()

	text := ""
	lineIndex := (*source.LineIndex)(nil)
	if openDoc != nil {
		openDoc.mu.RLock()
		text = openDoc.Text
		lineIndex = openDoc.LineIndex
		openDoc.mu.RUnlock()
	} else if syntheticText, syntheticLineIndex, ok := s.syntheticModuleSource(mod); ok {
		text = syntheticText
		lineIndex = syntheticLineIndex
	} else {
		bytes, err := os.ReadFile(mod.Path)
		if err != nil {
			return "", nil, false
		}
		text = string(bytes)
		lineIndex = source.NewLineIndex(text)
	}

	return text, lineIndex, true
}

func (s *Server) analyzeModuleByName(name string) (*ModuleSnapshot, bool) {
	if name == "" {
		return nil, false
	}

	if snapshot, ok := s.getModuleSnapshotByName(name); ok {
		return snapshot, true
	}

	mod, ok := s.LookupModule(name)
	if !ok {
		return nil, false
	}

	return s.analyzeModuleFile(mod)
}

func (s *Server) analyzeModuleFile(mod ModuleFile) (*ModuleSnapshot, bool) {
	if snapshot, ok := s.getModuleSnapshotByURI(mod.URI); ok {
		return snapshot, true
	}
	if wait, started := s.beginModuleBuild(mod.Name); !started {
		if wait != nil {
			<-wait
		}
		return s.getModuleSnapshotByURI(mod.URI)
	} else {
		defer s.finishModuleBuild(mod.Name)
	}

	// Check for open document
	s.docsMu.RLock()
	openDoc := s.docs[mod.URI]
	s.docsMu.RUnlock()

	text := ""
	lineIndex := (*source.LineIndex)(nil)
	if openDoc != nil {
		openDoc.mu.RLock()
		text = openDoc.Text
		lineIndex = openDoc.LineIndex
		openDoc.mu.RUnlock()
	} else if syntheticText, syntheticLineIndex, ok := s.syntheticModuleSource(mod); ok {
		text = syntheticText
		lineIndex = syntheticLineIndex
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
	s.publishModuleSnapshot(mod, snapshot)

	// Update reference index with snapshot's symbols
	s.refIndex.IndexDocument(snapshot.URI, snapshot.Tree, snapshot.LineIndex,
		snapshot.Symbols, snapshot.AttrSymbols, snapshot.Defs)

	s.enforceSnapshotLRULimit()
}

func (s *Server) publishModuleSnapshot(mod ModuleFile, snapshot *ModuleSnapshot) {
	if snapshot == nil {
		return
	}

	s.snapshotsMu.Lock()
	s.moduleSnapshotsByName[mod.Name] = snapshot
	s.moduleSnapshotsByURI[mod.URI] = snapshot
	s.snapshotLRU.touch(mod.URI, mod.Name)
	s.snapshotsMu.Unlock()

	s.depsMu.Lock()
	s.moduleImportsByURI[mod.URI] = append([]string(nil), snapshot.Imports...)
	s.depsMu.Unlock()
}

func (s *Server) beginModuleBuild(name string) (<-chan struct{}, bool) {
	if name == "" {
		return nil, true
	}
	s.snapshotsMu.Lock()
	if s.moduleSnapshotsByName[name] != nil {
		s.snapshotsMu.Unlock()
		return nil, false
	}
	if ch, ok := s.buildingModules[name]; ok {
		s.snapshotsMu.Unlock()
		return ch, false
	}
	ch := make(chan struct{})
	s.buildingModules[name] = ch
	s.snapshotsMu.Unlock()
	return ch, true
}

func (s *Server) finishModuleBuild(name string) {
	if name == "" {
		return
	}
	s.snapshotsMu.Lock()
	ch := s.buildingModules[name]
	delete(s.buildingModules, name)
	s.snapshotsMu.Unlock()
	if ch != nil {
		close(ch)
	}
}

// rebuildModuleSnapshotOnly rebuilds a module's snapshot without updating reverse
// dependency mappings. Callers that rebuild multiple modules in a batch should use
// this and call rebuildReverseDeps once when done.
func (s *Server) rebuildModuleSnapshotOnly(uri lsp.DocumentURI) (*ModuleSnapshot, bool) {
	mod, ok := s.LookupModuleByURI(uri)
	if !ok {
		return nil, false
	}

	text := ""
	lineIndex := (*source.LineIndex)(nil)
	if openDoc := s.Get(uri); openDoc != nil {
		text = openDoc.Text
		lineIndex = openDoc.LineIndex
	} else if syntheticText, syntheticLineIndex, ok := s.syntheticModuleSource(mod); ok {
		text = syntheticText
		lineIndex = syntheticLineIndex
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

func (s *Server) rebuildModuleByURI(uri lsp.DocumentURI) (*ModuleSnapshot, bool) {
	snapshot, ok := s.rebuildModuleSnapshotOnly(uri)
	if ok {
		s.rebuildReverseDeps()
	}
	return snapshot, ok
}

func (s *Server) getModuleSnapshotByURI(uri lsp.DocumentURI) (*ModuleSnapshot, bool) {
	s.snapshotsMu.Lock()
	snapshot := s.moduleSnapshotsByURI[uri]
	if snapshot != nil {
		s.snapshotLRU.touch(snapshot.URI, snapshot.Name)
	}
	s.snapshotsMu.Unlock()
	return snapshot, snapshot != nil
}

func (s *Server) getModuleSnapshotByName(name string) (*ModuleSnapshot, bool) {
	s.snapshotsMu.Lock()
	snapshot := s.moduleSnapshotsByName[name]
	if snapshot != nil {
		s.snapshotLRU.touch(snapshot.URI, snapshot.Name)
	}
	s.snapshotsMu.Unlock()
	return snapshot, snapshot != nil
}

func (s *Server) enforceSnapshotLRULimit() {
	if s.maxCachedModules <= 0 {
		return
	}

	for {
		s.snapshotsMu.Lock()
		resident := len(s.moduleSnapshotsByURI)
		if resident <= s.maxCachedModules {
			s.snapshotsMu.Unlock()
			return
		}

		candidate, ok := s.snapshotLRU.oldest()
		if !ok {
			s.snapshotsMu.Unlock()
			return
		}
		if s.openModuleCounts[candidate.uri] > 0 || s.buildingModules[candidate.name] != nil {
			s.snapshotLRU.touch(candidate.uri, candidate.name)
			s.snapshotsMu.Unlock()
			continue
		}

		snapshot := s.moduleSnapshotsByURI[candidate.uri]
		if snapshot == nil {
			s.snapshotLRU.remove(candidate.uri)
			s.snapshotsMu.Unlock()
			continue
		}

		delete(s.moduleSnapshotsByURI, candidate.uri)
		if candidate.name != "" {
			delete(s.moduleSnapshotsByName, candidate.name)
		}
		s.snapshotLRU.remove(candidate.uri)
		s.snapshotsMu.Unlock()

		s.refIndex.RemoveDocument(candidate.uri)
	}
}

func lookupExport(snapshot *ModuleSnapshot, name string) (*analyser.Symbol, bool) {
	if snapshot == nil || snapshot.Exports == nil {
		return nil, false
	}
	sym, ok := snapshot.Exports[name]
	return sym, ok
}

func lookupSurfaceExport(surface *ModuleImportSurface, name string) (*analyser.Symbol, bool) {
	if surface == nil || surface.Exports == nil {
		return nil, false
	}
	sym, ok := surface.Exports[name]
	return sym, ok
}

func lookupSnapshotByNameFromMap(snapshots map[string]*ModuleSnapshot, name string) (*ModuleSnapshot, bool) {
	if snapshots == nil || name == "" {
		return nil, false
	}
	snapshot := snapshots[name]
	return snapshot, snapshot != nil
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

func (s *Server) bindWorkspaceImports(tree *ast.AST, global *analyser.Scope, defs map[ast.NodeID]*analyser.Symbol, importerURI lsp.DocumentURI) []analyser.SemanticError {
	return s.bindWorkspaceImportsWithLookup(tree, global, defs, importerURI, s.analyzeModuleByName)
}

func (s *Server) bindWorkspaceImportsWithLookup(tree *ast.AST, global *analyser.Scope, defs map[ast.NodeID]*analyser.Symbol, importerURI lsp.DocumentURI, lookup moduleSnapshotLookup) []analyser.SemanticError {
	if tree == nil || tree.Root == ast.NoNode {
		return nil
	}

	var errs []analyser.SemanticError

	for stmt := tree.Node(tree.Root).FirstChild; stmt != ast.NoNode; stmt = tree.Node(stmt).NextSibling {
		switch tree.Node(stmt).Kind {
		case ast.NodeImport:
			errs = append(errs, s.bindImportStmtWithLookup(tree, stmt, defs, lookup)...)
		case ast.NodeFromImport:
			errs = append(errs, s.bindFromImportStmtWithLookup(tree, stmt, global, defs, importerURI, lookup)...)
		}
	}

	return errs
}

func (s *Server) bindWorkspaceImportsWithSurfaceLookup(tree *ast.AST, global *analyser.Scope, defs map[ast.NodeID]*analyser.Symbol, importerURI lsp.DocumentURI, lookup moduleImportSurfaceLookup) []analyser.SemanticError {
	if tree == nil || tree.Root == ast.NoNode {
		return nil
	}

	var errs []analyser.SemanticError

	for stmt := tree.Node(tree.Root).FirstChild; stmt != ast.NoNode; stmt = tree.Node(stmt).NextSibling {
		switch tree.Node(stmt).Kind {
		case ast.NodeImport:
			err := s.bindImportStmtWithSurfaceLookup(tree, stmt, defs, lookup)
			errs = append(errs, err...)
		case ast.NodeFromImport:
			err := s.bindFromImportStmtWithSurfaceLookup(tree, stmt, global, defs, importerURI, lookup)
			errs = append(errs, err...)
		}
	}

	return errs
}

func (s *Server) bindImportStmt(tree *ast.AST, stmt ast.NodeID, defs map[ast.NodeID]*analyser.Symbol) []analyser.SemanticError {
	return s.bindImportStmtWithLookup(tree, stmt, defs, s.analyzeModuleByName)
}

func (s *Server) bindImportStmtWithLookup(tree *ast.AST, stmt ast.NodeID, defs map[ast.NodeID]*analyser.Symbol, lookup moduleSnapshotLookup) []analyser.SemanticError {
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

		snapshot, ok := lookup(moduleToBind)
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
		local.Members = snapshotMemberScope(snapshot)
		local.Bases = nil
		local.InstanceOf = nil

		// For dotted imports without alias (e.g. `import a.b.c`), attach each
		// submodule as an attribute of its parent so `a.b.c` resolves correctly.
		if asName == ast.NoNode && strings.ContainsRune(fullName, '.') {
			s.bindSubmoduleChain(local, moduleToBind, fullName, lookup)
		}
	}

	return errs
}

func (s *Server) bindImportStmtWithSurfaceLookup(tree *ast.AST, stmt ast.NodeID, defs map[ast.NodeID]*analyser.Symbol, lookup moduleImportSurfaceLookup) []analyser.SemanticError {
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

		surface, ok := lookup(moduleToBind)
		if !ok {
			errs = append(errs, unresolvedModuleError(tree.RangeOf(target), moduleToBind))
			continue
		}

		local.Kind = analyser.SymModule
		local.URI = surface.URI
		local.Span = importSurfaceDefSpan(surface)
		local.DocString = ""
		local.Inner = nil
		local.Attrs = nil
		local.Members = importSurfaceMemberScope(surface)
		local.Bases = nil
		local.InstanceOf = nil

		if asName == ast.NoNode && strings.ContainsRune(fullName, '.') {
			s.bindSubmoduleChainFromSurface(local, moduleToBind, fullName, lookup)
		}
	}

	return errs
}

// bindSubmoduleChain walks the dotted segments of fullName beyond rootName and
// attaches each submodule as a member of the previous symbol.  For example,
// `import a.b.c` binds `local` (== `a`), then sets `local.Members["b"]` to the
// snapshot of `a.b`, then sets that symbol's Members["c"] to the snapshot of
// `a.b.c`.  We create a fresh Members scope for each level so we don't mutate
// the shared snapshot.Global scope.
func (s *Server) bindSubmoduleChain(root *analyser.Symbol, rootName, fullName string, lookup moduleSnapshotLookup) {
	segments := strings.Split(fullName, ".")
	currentSym := root
	prefix := rootName

	for _, seg := range segments[1:] {
		prefix = prefix + "." + seg
		subSnap, ok := lookup(prefix)
		if !ok {
			return
		}

		subSym := &analyser.Symbol{
			Name:    seg,
			Kind:    analyser.SymModule,
			URI:     subSnap.URI,
			Span:    moduleDefSpan(subSnap),
			Members: snapshotMemberScope(subSnap),
		}

		// Build a fresh Members scope for currentSym so we don't mutate the
		// shared snapshot.Global.  Copy existing entries first, then add subSym.
		oldMembers := currentSym.Members
		cap := 1
		if oldMembers != nil {
			cap = len(oldMembers.Symbols) + 1
		}
		newMembers := &analyser.Scope{
			Kind:    analyser.ScopeMember,
			Symbols: make(map[string]*analyser.Symbol, cap),
		}
		if oldMembers != nil {
			maps.Copy(newMembers.Symbols, oldMembers.Symbols)
		}
		newMembers.Symbols[seg] = subSym
		currentSym.Members = newMembers

		currentSym = subSym
	}
}

func (s *Server) bindSubmoduleChainFromSurface(root *analyser.Symbol, rootName, fullName string, lookup moduleImportSurfaceLookup) {
	segments := strings.Split(fullName, ".")
	currentSym := root
	prefix := rootName

	for _, seg := range segments[1:] {
		prefix = prefix + "." + seg
		surface, ok := lookup(prefix)
		if !ok {
			return
		}

		subSym := &analyser.Symbol{
			Name:    seg,
			Kind:    analyser.SymModule,
			URI:     surface.URI,
			Span:    importSurfaceDefSpan(surface),
			Members: importSurfaceMemberScope(surface),
		}

		oldMembers := currentSym.Members
		cap := 1
		if oldMembers != nil {
			cap = len(oldMembers.Symbols) + 1
		}
		newMembers := &analyser.Scope{
			Kind:    analyser.ScopeMember,
			Symbols: make(map[string]*analyser.Symbol, cap),
		}
		if oldMembers != nil {
			maps.Copy(newMembers.Symbols, oldMembers.Symbols)
		}
		newMembers.Symbols[seg] = subSym
		currentSym.Members = newMembers

		currentSym = subSym
	}
}

func (s *Server) bindFromImportStmt(tree *ast.AST, stmt ast.NodeID, global *analyser.Scope, defs map[ast.NodeID]*analyser.Symbol, importerURI lsp.DocumentURI) []analyser.SemanticError {
	return s.bindFromImportStmtWithLookup(tree, stmt, global, defs, importerURI, s.analyzeModuleByName)
}

func (s *Server) bindFromImportStmtWithLookup(tree *ast.AST, stmt ast.NodeID, global *analyser.Scope, defs map[ast.NodeID]*analyser.Symbol, importerURI lsp.DocumentURI, lookup moduleSnapshotLookup) []analyser.SemanticError {
	module, aliases := tree.FromImportParts(stmt)
	moduleName, ok := s.resolveImportModuleName(importerURI, tree, module, tree.Node(stmt).Data)
	if !ok {
		return nil
	}

	snapshot, ok := lookup(moduleName)
	if !ok {
		return []analyser.SemanticError{unresolvedModuleError(fromImportModuleSpan(tree, stmt, module), moduleName)}
	}

	var errs []analyser.SemanticError

	for _, alias := range aliases {
		if isStarImportAlias(tree, alias) {
			for name, remote := range starImportExports(snapshot) {
				if global == nil || remote == nil {
					continue
				}
				if _, exists := global.LookupLocal(name); exists {
					continue
				}
				local := cloneSymbolForImport(remote)
				if local == nil {
					continue
				}
				local.Name = name
				_ = global.Define(local)
			}
			continue
		}
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
		if ok && remote != nil {
			cloneImportedSymbol(local, remote)
			continue
		}

		submoduleName := moduleName + "." + name
		submodule, ok := lookup(submoduleName)
		if !ok || submodule == nil {
			errs = append(errs, missingImportNameError(tree.RangeOf(target), moduleName, name))
			continue
		}

		local.Kind = analyser.SymModule
		local.URI = submodule.URI
		local.Span = moduleDefSpan(submodule)
		local.DocString = ""
		local.Inner = nil
		local.Attrs = nil
		local.Members = snapshotMemberScope(submodule)
		local.Bases = nil
		local.InstanceOf = nil
	}

	return errs
}

func (s *Server) bindFromImportStmtWithSurfaceLookup(tree *ast.AST, stmt ast.NodeID, global *analyser.Scope, defs map[ast.NodeID]*analyser.Symbol, importerURI lsp.DocumentURI, lookup moduleImportSurfaceLookup) []analyser.SemanticError {
	module, aliases := tree.FromImportParts(stmt)
	moduleName, ok := s.resolveImportModuleName(importerURI, tree, module, tree.Node(stmt).Data)
	if !ok {
		return nil
	}

	surface, ok := lookup(moduleName)
	if !ok {
		return []analyser.SemanticError{unresolvedModuleError(fromImportModuleSpan(tree, stmt, module), moduleName)}
	}

	var errs []analyser.SemanticError

	for _, alias := range aliases {
		if isStarImportAlias(tree, alias) {
			for name, remote := range surface.Exports {
				if global == nil || remote == nil {
					continue
				}
				if _, exists := global.LookupLocal(name); exists {
					continue
				}
				if strings.HasPrefix(name, "_") {
					continue
				}
				local := cloneSymbolForImport(remote)
				if local == nil {
					continue
				}
				local.Name = name
				_ = global.Define(local)
			}
			continue
		}
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
		remote, ok := lookupSurfaceExport(surface, name)
		if ok && remote != nil {
			cloneImportedSymbol(local, remote)
			continue
		}

		submoduleName := moduleName + "." + name
		submodule, ok := lookup(submoduleName)
		if !ok || submodule == nil {
			errs = append(errs, missingImportNameError(tree.RangeOf(target), moduleName, name))
			continue
		}

		local.Kind = analyser.SymModule
		local.URI = submodule.URI
		local.Span = importSurfaceDefSpan(submodule)
		local.DocString = ""
		local.Inner = nil
		local.Attrs = nil
		local.Members = importSurfaceMemberScope(submodule)
		local.Bases = nil
		local.InstanceOf = nil
	}

	return errs
}

func (s *Server) lineIndexForURI(uri lsp.DocumentURI) *source.LineIndex {
	if doc := s.Get(uri); doc != nil {
		doc.mu.RLock()
		li := doc.LineIndex
		doc.mu.RUnlock()
		return li
	}

	snapshot, _ := s.getModuleSnapshotByURI(uri)
	if snapshot != nil {
		return snapshot.LineIndex
	}

	if mod, ok := s.LookupModuleByURI(uri); ok {
		snapshot, ok := s.analyzeModuleFile(mod)
		if ok && snapshot != nil {
			return snapshot.LineIndex
		}
	}
	return nil
}
