package server

import (
	"sort"
	"strings"

	a "rahu/analyser"
	"rahu/jsonrpc"
	"rahu/lsp"
	ast "rahu/parser/ast"
	"rahu/source"
)

var semanticTokenLegendTypes = []string{
	"keyword",
	"class",
	"function",
	"method",
	"parameter",
	"variable",
	"property",
	"type",
	"module",
}

var semanticTokenLegendModifiers = []string{
	"declaration",
	"readonly",
	"defaultLibrary",
}

const (
	semanticTokenKeyword = iota
	semanticTokenClass
	semanticTokenFunction
	semanticTokenMethod
	semanticTokenParameter
	semanticTokenVariable
	semanticTokenProperty
	semanticTokenType
	semanticTokenModule
)

const (
	semanticModifierDeclaration = 1 << iota
	semanticModifierReadonly
	semanticModifierDefaultLibrary
)

type semanticTokenEntry struct {
	line      int
	start     int
	length    int
	tokenType int
	modifiers int
}

type semanticTokenKey struct {
	line      int
	start     int
	length    int
	tokenType int
	modifiers int
}

func semanticTokenRangeForNode(tree *ast.AST, nodeID ast.NodeID, isAttr bool) ast.Range {
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

func semanticTokenTypeForSymbol(sym *a.Symbol) (int, bool) {
	if sym == nil {
		return 0, false
	}
	switch sym.Kind {
	case a.SymClass:
		return semanticTokenClass, true
	case a.SymFunction:
		return semanticTokenFunction, true
	case a.SymParameter:
		return semanticTokenParameter, true
	case a.SymVariable, a.SymConstant, a.SymBuiltin:
		return semanticTokenVariable, true
	case a.SymAttr, a.SymField:
		return semanticTokenProperty, true
	case a.SymType:
		return semanticTokenType, true
	case a.SymModule, a.SymImport:
		return semanticTokenModule, true
	default:
		return 0, false
	}
}

func isMethodDeclarationNode(tree *ast.AST, nodeID ast.NodeID) bool {
	if tree == nil || nodeID == ast.NoNode {
		return false
	}
	for id := ast.NodeID(1); int(id) < len(tree.Nodes); id++ {
		if tree.Node(id).Kind != ast.NodeClassDef {
			continue
		}
		_, _, body := tree.ClassParts(id)
		for stmt := tree.Node(body).FirstChild; stmt != ast.NoNode; stmt = tree.Node(stmt).NextSibling {
			if tree.Node(stmt).Kind != ast.NodeFunctionDef {
				continue
			}
			nameID, _, _ := tree.FunctionParts(stmt)
			if nameID == nodeID {
				return true
			}
		}
	}
	return false
}

func semanticTokenModifiersForSymbol(sym *a.Symbol, declaration bool) int {
	mods := 0
	if declaration {
		mods |= semanticModifierDeclaration
	}
	if sym == nil {
		return mods
	}
	if sym.Kind == a.SymConstant {
		mods |= semanticModifierReadonly
	}
	if sym.Scope != nil && sym.Scope.Kind == a.ScopeBuiltin {
		mods |= semanticModifierDefaultLibrary
	}
	return mods
}

func appendSemanticToken(entries *[]semanticTokenEntry, seen map[semanticTokenKey]struct{}, li *source.LineIndex, r ast.Range, tokenType int, modifiers int) {
	if li == nil || r.IsEmpty() || r.End <= r.Start {
		return
	}
	line, char := li.OffsetToPosition(int(r.Start))
	length := int(r.End - r.Start)
	key := semanticTokenKey{line: line, start: char, length: length, tokenType: tokenType, modifiers: modifiers}
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	*entries = append(*entries, semanticTokenEntry{
		line:      line,
		start:     char,
		length:    length,
		tokenType: tokenType,
		modifiers: modifiers,
	})
}

func appendSemanticSymbolToken(entries *[]semanticTokenEntry, seen map[semanticTokenKey]struct{}, doc *Document, nodeID ast.NodeID, sym *a.Symbol, isAttr bool, declaration bool) {
	if doc == nil || doc.Tree == nil || doc.LineIndex == nil {
		return
	}
	tokenType, ok := semanticTokenTypeForSymbol(sym)
	if !ok {
		return
	}
	if sym.Kind == a.SymFunction && (isAttr || isMethodDeclarationNode(doc.Tree, nodeID)) {
		tokenType = semanticTokenMethod
	}
	r := semanticTokenRangeForNode(doc.Tree, nodeID, isAttr)
	appendSemanticToken(entries, seen, doc.LineIndex, r, tokenType, semanticTokenModifiersForSymbol(sym, declaration))
}

func appendKeywordToken(entries *[]semanticTokenEntry, seen map[semanticTokenKey]struct{}, doc *Document, start uint32, keyword string) {
	if doc == nil || doc.LineIndex == nil || keyword == "" {
		return
	}
	r := ast.Range{Start: start, End: start + uint32(len(keyword))}
	appendSemanticToken(entries, seen, doc.LineIndex, r, semanticTokenKeyword, 0)
}

func appendKeywordTokens(entries *[]semanticTokenEntry, seen map[semanticTokenKey]struct{}, doc *Document) {
	if doc == nil || doc.Tree == nil {
		return
	}
	for id := ast.NodeID(1); int(id) < len(doc.Tree.Nodes); id++ {
		node := doc.Tree.Node(id)
		switch node.Kind {
		case ast.NodeFunctionDef:
			appendKeywordToken(entries, seen, doc, node.Start, "def")
		case ast.NodeClassDef:
			appendKeywordToken(entries, seen, doc, node.Start, "class")
		case ast.NodeReturn:
			appendKeywordToken(entries, seen, doc, node.Start, "return")
		case ast.NodeRaise:
			appendKeywordToken(entries, seen, doc, node.Start, "raise")
		case ast.NodePass:
			appendKeywordToken(entries, seen, doc, node.Start, "pass")
		case ast.NodeIf:
			appendKeywordToken(entries, seen, doc, node.Start, "if")
		case ast.NodeFor:
			appendKeywordToken(entries, seen, doc, node.Start, "for")
		case ast.NodeWhile:
			appendKeywordToken(entries, seen, doc, node.Start, "while")
		case ast.NodeTry:
			appendKeywordToken(entries, seen, doc, node.Start, "try")
			_, excepts, elseBlock, finallyBlock := doc.Tree.TryParts(id)
			for _, exceptClause := range excepts {
				appendKeywordToken(entries, seen, doc, doc.Tree.Node(exceptClause).Start, "except")
			}
			if elseBlock != ast.NoNode && doc.Tree.Node(elseBlock).Data > 0 {
				appendKeywordToken(entries, seen, doc, doc.Tree.Node(elseBlock).Data, "else")
			}
			if finallyBlock != ast.NoNode && doc.Tree.Node(finallyBlock).Data > 0 {
				appendKeywordToken(entries, seen, doc, doc.Tree.Node(finallyBlock).Data, "finally")
			}
		case ast.NodeBreak:
			appendKeywordToken(entries, seen, doc, node.Start, "break")
		case ast.NodeContinue:
			appendKeywordToken(entries, seen, doc, node.Start, "continue")
		case ast.NodeImport:
			appendKeywordToken(entries, seen, doc, node.Start, "import")
		case ast.NodeFromImport:
			appendKeywordToken(entries, seen, doc, node.Start, "from")
			if idx := strings.Index(doc.Text[node.Start:node.End], " import "); idx >= 0 {
				appendKeywordToken(entries, seen, doc, node.Start+uint32(idx)+1, "import")
			}
		}
	}
}

func appendImportModuleTokens(entries *[]semanticTokenEntry, seen map[semanticTokenKey]struct{}, doc *Document) {
	if doc == nil || doc.Tree == nil || doc.LineIndex == nil {
		return
	}
	for id := ast.NodeID(1); int(id) < len(doc.Tree.Nodes); id++ {
		node := doc.Tree.Node(id)
		switch node.Kind {
		case ast.NodeImport:
			for alias := node.FirstChild; alias != ast.NoNode; alias = doc.Tree.Node(alias).NextSibling {
				target, _ := doc.Tree.AliasParts(alias)
				if target == ast.NoNode {
					continue
				}
				appendSemanticToken(entries, seen, doc.LineIndex, doc.Tree.RangeOf(target), semanticTokenModule, 0)
			}
		case ast.NodeFromImport:
			module, _ := doc.Tree.FromImportParts(id)
			if module != ast.NoNode {
				appendSemanticToken(entries, seen, doc.LineIndex, doc.Tree.RangeOf(module), semanticTokenModule, 0)
			}
		}
	}
}

func encodeSemanticTokens(entries []semanticTokenEntry) []uint32 {
	if len(entries) == 0 {
		return []uint32{}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].line != entries[j].line {
			return entries[i].line < entries[j].line
		}
		if entries[i].start != entries[j].start {
			return entries[i].start < entries[j].start
		}
		if entries[i].length != entries[j].length {
			return entries[i].length < entries[j].length
		}
		if entries[i].tokenType != entries[j].tokenType {
			return entries[i].tokenType < entries[j].tokenType
		}
		return entries[i].modifiers < entries[j].modifiers
	})
	out := make([]uint32, 0, len(entries)*5)
	prevLine := 0
	prevStart := 0
	for i, entry := range entries {
		deltaLine := entry.line - prevLine
		deltaStart := entry.start
		if i > 0 && deltaLine == 0 {
			deltaStart = entry.start - prevStart
		}
		out = append(out,
			uint32(deltaLine),
			uint32(deltaStart),
			uint32(entry.length),
			uint32(entry.tokenType),
			uint32(entry.modifiers),
		)
		prevLine = entry.line
		prevStart = entry.start
	}
	return out
}

func buildSemanticTokens(doc *Document) []semanticTokenEntry {
	if doc == nil || doc.Tree == nil || doc.LineIndex == nil {
		return nil
	}
	entries := make([]semanticTokenEntry, 0, len(doc.Symbols)+len(doc.Defs)+len(doc.AttrSymbols)+16)
	seen := make(map[semanticTokenKey]struct{}, len(entries))
	appendKeywordTokens(&entries, seen, doc)
	appendImportModuleTokens(&entries, seen, doc)
	for nodeID, sym := range doc.Defs {
		appendSemanticSymbolToken(&entries, seen, doc, nodeID, sym, false, true)
	}
	for nodeID, sym := range doc.Symbols {
		appendSemanticSymbolToken(&entries, seen, doc, nodeID, sym, false, false)
	}
	for nodeID, sym := range doc.AttrSymbols {
		decl := sym != nil && sym.URI == doc.URI && semanticTokenRangeForNode(doc.Tree, nodeID, true) == sym.Span
		appendSemanticSymbolToken(&entries, seen, doc, nodeID, sym, true, decl)
	}
	return entries
}

func (s *Server) SemanticTokensFull(p *lsp.SemanticTokensParams) (*lsp.SemanticTokens, *jsonrpc.Error) {
	doc := s.Get(p.TextDocument.URI)
	if doc == nil {
		return &lsp.SemanticTokens{Data: []uint32{}}, nil
	}
	doc.mu.RLock()
	entries := buildSemanticTokens(doc)
	doc.mu.RUnlock()
	return &lsp.SemanticTokens{Data: encodeSemanticTokens(entries)}, nil
}
