package server

import (
	"sort"
	"strings"

	a "rahu/analyser"
	"rahu/jsonrpc"
	"rahu/lsp"
	ast "rahu/parser/ast"
	l "rahu/server/locate"
)

type scoredCompletion struct {
	item      lsp.CompletionItem
	score     int
	isBuiltin bool
}

func isBuiltinSymbol(sym *a.Symbol) bool {
	return sym != nil && sym.Scope != nil && sym.Scope.Kind == a.ScopeBuiltin
}

func toCompletionItemKind(sym *a.Symbol) lsp.CompletionItemKind {
	if sym == nil {
		return lsp.CompletionItemKindVariable
	}
	switch sym.Kind {
	case a.SymModule:
		return lsp.CompletionItemKindModule
	case a.SymClass, a.SymType:
		return lsp.CompletionItemKindClass
	case a.SymFunction:
		return lsp.CompletionItemKindFunction
	case a.SymAttr, a.SymField:
		return lsp.CompletionItemKindField
	case a.SymConstant:
		return lsp.CompletionItemKindConstant
	default:
		return lsp.CompletionItemKindVariable
	}
}

func matchesPrefix(prefix, name string) bool {
	if prefix == "" {
		return true
	}
	return strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix))
}

func completionScore(label, prefix string, scopeDistance int, contextBonus int, builtin bool) int {
	score := contextBonus
	if prefix == "" {
		score += 100
	} else if label == prefix {
		score += 1000
	} else if strings.EqualFold(label, prefix) {
		score += 900
	} else if strings.HasPrefix(label, prefix) {
		score += 800
	} else if strings.HasPrefix(strings.ToLower(label), strings.ToLower(prefix)) {
		score += 700
	}

	if builtin {
		score -= 200
	} else {
		score += 300 - min(scopeDistance, 3)*75
	}

	if strings.HasPrefix(label, "_") {
		score -= 25
	}

	return score
}

func rankAndDedupeCompletions(candidates []scoredCompletion, keepBuiltinDuplicates bool) []lsp.CompletionItem {
	bestNonBuiltin := make(map[string]scoredCompletion)
	bestBuiltin := make(map[string]scoredCompletion)
	for _, candidate := range candidates {
		if candidate.isBuiltin && keepBuiltinDuplicates {
			if existing, ok := bestBuiltin[candidate.item.Label]; !ok || candidate.score > existing.score {
				bestBuiltin[candidate.item.Label] = candidate
			}
			continue
		}
		if existing, ok := bestNonBuiltin[candidate.item.Label]; !ok || candidate.score > existing.score {
			bestNonBuiltin[candidate.item.Label] = candidate
		}
	}

	ranked := make([]scoredCompletion, 0, len(bestNonBuiltin)+len(bestBuiltin))
	for _, candidate := range bestNonBuiltin {
		ranked = append(ranked, candidate)
	}
	for _, candidate := range bestBuiltin {
		ranked = append(ranked, candidate)
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		if ranked[i].item.Label != ranked[j].item.Label {
			return ranked[i].item.Label < ranked[j].item.Label
		}
		return ranked[i].item.Detail < ranked[j].item.Detail
	})

	out := make([]lsp.CompletionItem, 0, len(ranked))
	for _, candidate := range ranked {
		out = append(out, candidate.item)
	}
	return out
}

func linePrefixAt(doc *Document, pos lsp.Position) string {
	offset := doc.LineIndex.PositionToOffset(pos.Line, pos.Character)
	if offset < 0 {
		return ""
	}
	if offset > len(doc.Text) {
		offset = len(doc.Text)
	}
	lineStart := strings.LastIndex(doc.Text[:offset], "\n") + 1
	return doc.Text[lineStart:offset]
}

func isIdentChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

func parseImportCompletion(line string) (string, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(trimmed, "import ") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(trimmed, "import ")), true
}

func parseFromImportCompletion(line string) (moduleName, prefix string, ok bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(trimmed, "from ") {
		return "", "", false
	}
	rest := strings.TrimPrefix(trimmed, "from ")
	parts := strings.SplitN(rest, " import ", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	moduleName = strings.TrimSpace(parts[0])
	prefix = strings.TrimSpace(parts[1])
	if moduleName == "" {
		return "", "", false
	}
	return moduleName, prefix, true
}

func dottedAccessAt(doc *Document, pos lsp.Position) (string, string, bool) {
	if doc == nil || doc.LineIndex == nil {
		return "", "", false
	}
	offset := doc.LineIndex.PositionToOffset(pos.Line, pos.Character)
	if offset < 0 || offset > len(doc.Text) {
		return "", "", false
	}
	lineStart := strings.LastIndex(doc.Text[:offset], "\n") + 1
	segment := doc.Text[lineStart:offset]
	if segment == "" {
		return "", "", false
	}
	end := len(segment)
	start := end
	for start > 0 && isIdentChar(segment[start-1]) {
		start--
	}
	memberPrefix := segment[start:end]
	if start == 0 || segment[start-1] != '.' {
		return "", "", false
	}
	receiverEnd := start - 1
	receiverStart := receiverEnd
	brackets := 0
	for receiverStart > 0 {
		ch := segment[receiverStart-1]
		switch ch {
		case ']':
			brackets++
		case '[':
			brackets--
		case ' ', '\t':
			if brackets == 0 {
				goto done
			}
		}
		receiverStart--
	}
done:
	receiver := segment[receiverStart:receiverEnd]
	if receiver == "" {
		return "", "", false
	}
	return receiver, memberPrefix, true
}

func identifierPrefixAt(line string) string {
	end := len(line)
	start := end
	for start > 0 && isIdentChar(line[start-1]) {
		start--
	}
	return line[start:end]
}

func innermostEnclosingDef(tree *ast.AST, pos int) ast.NodeID {
	if tree == nil || tree.Root == ast.NoNode {
		return ast.NoNode
	}
	var best ast.NodeID = ast.NoNode
	var visitStmt func(ast.NodeID)
	visitStmt = func(id ast.NodeID) {
		if id == ast.NoNode || !l.Contains(tree.RangeOf(id), pos) {
			return
		}
		kind := tree.Node(id).Kind
		if kind == ast.NodeFunctionDef || kind == ast.NodeClassDef {
			best = id
		}
		switch kind {
		case ast.NodeClassDef:
			_, _, body := tree.ClassParts(id)
			for stmt := tree.Node(body).FirstChild; stmt != ast.NoNode; stmt = tree.Node(stmt).NextSibling {
				visitStmt(stmt)
			}
		case ast.NodeFunctionDef:
			_, _, body := tree.FunctionParts(id)
			for stmt := tree.Node(body).FirstChild; stmt != ast.NoNode; stmt = tree.Node(stmt).NextSibling {
				visitStmt(stmt)
			}
		case ast.NodeIf:
			test := tree.ChildAt(id, 0)
			body := ast.NoNode
			orelse := ast.NoNode
			if test != ast.NoNode {
				body = tree.Node(test).NextSibling
			}
			if body != ast.NoNode {
				orelse = tree.Node(body).NextSibling
			}
			for stmt := tree.Node(body).FirstChild; stmt != ast.NoNode; stmt = tree.Node(stmt).NextSibling {
				visitStmt(stmt)
			}
			for stmt := tree.Node(orelse).FirstChild; stmt != ast.NoNode; stmt = tree.Node(stmt).NextSibling {
				visitStmt(stmt)
			}
		case ast.NodeFor:
			target := tree.ChildAt(id, 0)
			iter := ast.NoNode
			body := ast.NoNode
			orelse := ast.NoNode
			if target != ast.NoNode {
				iter = tree.Node(target).NextSibling
			}
			if iter != ast.NoNode {
				body = tree.Node(iter).NextSibling
			}
			if body != ast.NoNode {
				orelse = tree.Node(body).NextSibling
			}
			for stmt := tree.Node(body).FirstChild; stmt != ast.NoNode; stmt = tree.Node(stmt).NextSibling {
				visitStmt(stmt)
			}
			for stmt := tree.Node(orelse).FirstChild; stmt != ast.NoNode; stmt = tree.Node(stmt).NextSibling {
				visitStmt(stmt)
			}
		case ast.NodeWhile:
			test := tree.ChildAt(id, 0)
			body := ast.NoNode
			if test != ast.NoNode {
				body = tree.Node(test).NextSibling
			}
			for stmt := tree.Node(body).FirstChild; stmt != ast.NoNode; stmt = tree.Node(stmt).NextSibling {
				visitStmt(stmt)
			}
		}
	}
	for stmt := tree.Node(tree.Root).FirstChild; stmt != ast.NoNode; stmt = tree.Node(stmt).NextSibling {
		visitStmt(stmt)
	}
	return best
}

func scopeAtPosition(doc *Document, pos lsp.Position) *a.Scope {
	if doc == nil || doc.Tree == nil || doc.LineIndex == nil || doc.Global == nil {
		return nil
	}
	offset := doc.LineIndex.PositionToOffset(pos.Line, pos.Character)
	def := innermostEnclosingDef(doc.Tree, offset)
	if def == ast.NoNode {
		return doc.Global
	}
	var nameID ast.NodeID
	switch doc.Tree.Node(def).Kind {
	case ast.NodeFunctionDef:
		nameID, _, _ = doc.Tree.FunctionParts(def)
	case ast.NodeClassDef:
		nameID, _, _ = doc.Tree.ClassParts(def)
	}
	if sym := doc.Defs[nameID]; sym != nil && sym.Inner != nil {
		return sym.Inner
	}
	return doc.Global
}

func visibleNameCompletionItems(doc *Document, pos lsp.Position, prefix string) []lsp.CompletionItem {
	scope := scopeAtPosition(doc, pos)
	if scope == nil {
		return nil
	}
	// Estimate capacity based on scope symbols; will grow if needed for parent scopes
	initialCap := 32
	if scope.Symbols != nil && len(scope.Symbols) > initialCap {
		initialCap = len(scope.Symbols)
	}
	candidates := make([]scoredCompletion, 0, initialCap)
	seen := make(map[string]struct{}, initialCap)
	builtinSeen := make(map[string]struct{}, 32)
	distance := 0
	for current := scope; current != nil; current = current.Parent {
		for name, sym := range current.Symbols {
			if sym == nil || !matchesPrefix(prefix, name) {
				continue
			}
			if sym.Kind == a.SymImport && sym.URI == "" && sym.Span.IsEmpty() {
				continue
			}
			isBuiltin := isBuiltinSymbol(sym)
			if isBuiltin {
				if _, ok := builtinSeen[name]; ok {
					continue
				}
				builtinSeen[name] = struct{}{}
			} else {
				if _, ok := seen[name]; ok {
					continue
				}
				seen[name] = struct{}{}
			}
			candidates = append(candidates, scoredCompletion{
				item:      lsp.CompletionItem{Label: name, Kind: toCompletionItemKind(sym), Detail: sym.Kind.String()},
				score:     completionScore(name, prefix, distance, 180, isBuiltin),
				isBuiltin: isBuiltin,
			})
		}
		distance++
	}
	return rankAndDedupeCompletions(candidates, true)
}

func (s *Server) moduleCompletionItems(prefix string) []lsp.CompletionItem {
	s.indexMu.RLock()
	candidates := make([]scoredCompletion, 0, len(s.modulesByName))
	for name := range s.modulesByName {
		if !matchesPrefix(prefix, name) {
			continue
		}
		candidates = append(candidates, scoredCompletion{
			item:  lsp.CompletionItem{Label: name, Kind: lsp.CompletionItemKindModule, Detail: "module"},
			score: completionScore(name, prefix, 0, 250, false),
		})
	}
	s.indexMu.RUnlock()
	return rankAndDedupeCompletions(candidates, false)
}

func (s *Server) childModuleItems(container, prefix string) []lsp.CompletionItem {
	if container == "" {
		return nil
	}
	needle := container + "."
	s.indexMu.RLock()
	seen := make(map[string]struct{})
	candidates := make([]scoredCompletion, 0)
	for name := range s.modulesByName {
		if !strings.HasPrefix(name, needle) {
			continue
		}
		rest := strings.TrimPrefix(name, needle)
		if rest == "" {
			continue
		}
		segment := rest
		if idx := strings.IndexByte(rest, '.'); idx >= 0 {
			segment = rest[:idx]
		}
		if segment == "" || !matchesPrefix(prefix, segment) {
			continue
		}
		if _, ok := seen[segment]; ok {
			continue
		}
		seen[segment] = struct{}{}
		candidates = append(candidates, scoredCompletion{
			item:  lsp.CompletionItem{Label: segment, Kind: lsp.CompletionItemKindModule, Detail: container},
			score: completionScore(segment, prefix, 0, 220, false),
		})
	}
	s.indexMu.RUnlock()
	return rankAndDedupeCompletions(candidates, false)
}

func exportCompletionItems(snapshot *ModuleSnapshot, prefix string) []lsp.CompletionItem {
	if snapshot == nil || snapshot.Exports == nil {
		return nil
	}
	candidates := make([]scoredCompletion, 0, len(snapshot.Exports))
	for name, sym := range snapshot.Exports {
		if sym == nil || sym.Kind == a.SymImport || sym.Span.IsEmpty() || !matchesPrefix(prefix, name) {
			continue
		}
		isBuiltin := isBuiltinSymbol(sym)
		candidates = append(candidates, scoredCompletion{
			item:      lsp.CompletionItem{Label: name, Kind: toCompletionItemKind(sym), Detail: snapshot.Name},
			score:     completionScore(name, prefix, 0, 240, isBuiltin),
			isBuiltin: isBuiltin,
		})
	}
	return rankAndDedupeCompletions(candidates, false)
}

func memberCompletionItems(scope *a.Scope, prefix string, detail string) []lsp.CompletionItem {
	if scope == nil {
		return nil
	}
	candidates := make([]scoredCompletion, 0, len(scope.Symbols))
	for name, sym := range scope.Symbols {
		if sym == nil || !matchesPrefix(prefix, name) {
			continue
		}
		isBuiltin := isBuiltinSymbol(sym)
		candidates = append(candidates, scoredCompletion{
			item:      lsp.CompletionItem{Label: name, Kind: toCompletionItemKind(sym), Detail: detail},
			score:     completionScore(name, prefix, 0, 230, isBuiltin),
			isBuiltin: isBuiltin,
		})
	}
	return rankAndDedupeCompletions(candidates, false)
}

func classMemberCompletionItems(cls *a.Symbol, prefix string, detail string) []lsp.CompletionItem {
	if cls == nil {
		return nil
	}
	// Estimate ~10 members per class on average
	candidates := make([]scoredCompletion, 0, 16)
	seen := make(map[string]struct{}, 16)
	var collect func(*a.Symbol)
	collect = func(sym *a.Symbol) {
		if sym == nil {
			return
		}
		if sym.Members != nil {
			for name, member := range sym.Members.Symbols {
				if _, ok := seen[name]; ok || member == nil || !matchesPrefix(prefix, name) {
					continue
				}
				seen[name] = struct{}{}
				isBuiltin := isBuiltinSymbol(member)
				candidates = append(candidates, scoredCompletion{
					item:      lsp.CompletionItem{Label: name, Kind: toCompletionItemKind(member), Detail: detail},
					score:     completionScore(name, prefix, 0, 230, isBuiltin),
					isBuiltin: isBuiltin,
				})
			}
		}
		for _, base := range sym.Bases {
			collect(base)
		}
	}
	collect(cls)
	return rankAndDedupeCompletions(candidates, false)
}

func typeMemberCompletionItems(t *a.Type, prefix string, detail string) []lsp.CompletionItem {
	if a.IsUnknownType(t) {
		return nil
	}
	if t.Kind == a.TypeUnion {
		candidates := make([]scoredCompletion, 0, 16)
		seen := make(map[string]struct{}, 16)
		for _, arm := range t.Union {
			for _, item := range typeMemberCompletionItems(arm, prefix, detail) {
				if _, ok := seen[item.Label]; ok {
					continue
				}
				seen[item.Label] = struct{}{}
				candidates = append(candidates, scoredCompletion{item: item, score: completionScore(item.Label, prefix, 0, 230, false)})
			}
		}
		return rankAndDedupeCompletions(candidates, false)
	}
	if t.Symbol == nil {
		return nil
	}
	if t.Kind == a.TypeInstance || t.Kind == a.TypeClass {
		return classMemberCompletionItems(t.Symbol, prefix, t.Symbol.Name)
	}
	return nil
}

func receiverTypeFromExpr(doc *Document, pos lsp.Position, sym *a.Symbol, expr string) *a.Type {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil
	}
	if sym != nil {
		if inferred := a.SymbolType(sym); inferred != nil {
			return inferred
		}
	}
	if idx := strings.IndexByte(expr, '['); idx > 0 && strings.HasSuffix(expr, "]") {
		baseName := expr[:idx]
		baseName = strings.TrimSpace(baseName)
		if baseName == "" || doc == nil {
			return nil
		}
		if scope := scopeAtPosition(doc, pos); scope != nil {
			if baseSym, ok := scope.Lookup(baseName); ok {
				return a.SubscriptResultType(a.SymbolType(baseSym))
			}
		}
	}
	return nil
}

func classScopeForReceiver(sym *a.Symbol) *a.Scope {
	if sym == nil || sym.Scope == nil {
		return nil
	}
	if cls := classOwner(sym.Scope); cls != nil {
		return cls.Members
	}
	return nil
}

func instanceClassForReceiver(doc *Document, sym *a.Symbol) *a.Symbol {
	if sym != nil {
		if inferred := a.SymbolType(sym); inferred != nil && inferred.Kind == a.TypeInstance && inferred.Symbol != nil {
			return inferred.Symbol
		}
		if sym.InstanceOf != nil {
			return sym.InstanceOf
		}
	}
	if doc == nil || sym == nil {
		return nil
	}
	if sym.Scope != nil {
		if scoped, ok := sym.Scope.Lookup(sym.Name); ok && scoped != nil {
			if inferred := a.SymbolType(scoped); inferred != nil && inferred.Kind == a.TypeInstance && inferred.Symbol != nil {
				return inferred.Symbol
			}
			if scoped.InstanceOf != nil {
				return scoped.InstanceOf
			}
		}
	}
	if doc.Global != nil {
		if global, ok := doc.Global.Lookup(sym.Name); ok && global != nil {
			if inferred := a.SymbolType(global); inferred != nil && inferred.Kind == a.TypeInstance && inferred.Symbol != nil {
				return inferred.Symbol
			}
			if global.InstanceOf != nil {
				return global.InstanceOf
			}
		}
	}
	return nil
}

func instanceTypeForReceiver(doc *Document, sym *a.Symbol) *a.Type {
	if sym != nil {
		if inferred := a.SymbolType(sym); inferred != nil && (inferred.Kind == a.TypeInstance || inferred.Kind == a.TypeUnion) {
			return inferred
		}
	}
	cls := instanceClassForReceiver(doc, sym)
	if cls == nil {
		return nil
	}
	return a.InstanceType(cls)
}

func (s *Server) moduleMemberCompletions(doc *Document, pos lsp.Position, receiver string, memberPrefix string) []lsp.CompletionItem {
	if doc == nil {
		return nil
	}
	scope := scopeAtPosition(doc, pos)
	if scope == nil {
		return nil
	}
	sym, ok := scope.Lookup(receiver)
	if !ok {
		sym = nil
	}
	if sym != nil && sym.Kind == a.SymModule && sym.URI != "" {
		snapshot, _ := s.getModuleSnapshotByURI(sym.URI)
		if snapshot == nil {
			if mod, ok := s.LookupModuleByURI(sym.URI); ok {
				snapshot, _ = s.analyzeModuleFile(mod)
			}
		}
		candidates := make([]scoredCompletion, 0, 16)
		for _, item := range exportCompletionItems(snapshot, memberPrefix) {
			candidates = append(candidates, scoredCompletion{item: item, score: completionScore(item.Label, memberPrefix, 0, 240, false)})
		}
		if snapshot != nil && snapshot.Name != "" {
			for _, item := range s.childModuleItems(snapshot.Name, memberPrefix) {
				candidates = append(candidates, scoredCompletion{item: item, score: completionScore(item.Label, memberPrefix, 0, 220, false)})
			}
		}
		return rankAndDedupeCompletions(candidates, false)
	}
	if sym != nil && sym.Kind == a.SymClass {
		return classMemberCompletionItems(sym, memberPrefix, sym.Name)
	}
	if inferred := receiverTypeFromExpr(doc, pos, sym, receiver); inferred != nil {
		return typeMemberCompletionItems(inferred, memberPrefix, "member")
	}
	if sym != nil && sym.Kind == a.SymParameter {
		if cls := classOwner(sym.Scope); cls != nil {
			return classMemberCompletionItems(cls, memberPrefix, cls.Name)
		}
		return memberCompletionItems(classScopeForReceiver(sym), memberPrefix, "member")
	}
	return nil
}

func (s *Server) Completion(p *lsp.CompletionParams) ([]lsp.CompletionItem, *jsonrpc.Error) {
	// Wait for indexing before providing completions
	if err := s.WaitForIndexing(); err != nil {
		return []lsp.CompletionItem{}, nil
	}

	doc := s.Get(p.TextDocument.URI)
	if doc == nil || doc.LineIndex == nil {
		return nil, jsonrpc.InvalidParamsError(nil)
	}

	line := linePrefixAt(doc, p.Position)
	if prefix, ok := parseImportCompletion(line); ok {
		return s.moduleCompletionItems(prefix), nil
	}
	if moduleName, prefix, ok := parseFromImportCompletion(line); ok {
		snapshot, found := s.analyzeModuleByName(moduleName)
		if !found {
			return []lsp.CompletionItem{}, nil
		}
		return exportCompletionItems(snapshot, prefix), nil
	}
	if receiver, memberPrefix, ok := dottedAccessAt(doc, p.Position); ok {
		return s.moduleMemberCompletions(doc, p.Position, receiver, memberPrefix), nil
	}
	return visibleNameCompletionItems(doc, p.Position, identifierPrefixAt(line)), nil
}
