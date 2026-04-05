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

func innermostEnclosingCall(tree *ast.AST, pos int) ast.NodeID {
	if tree == nil || tree.Root == ast.NoNode {
		return ast.NoNode
	}

	best := ast.NoNode
	bestRange := ast.Range{}
	for id := ast.NodeID(1); int(id) < len(tree.Nodes); id++ {
		if tree.Node(id).Kind != ast.NodeCall {
			continue
		}
		r := tree.RangeOf(id)
		if !l.Contains(r, pos) {
			continue
		}
		if best == ast.NoNode || (r.End-r.Start) < (bestRange.End-bestRange.Start) {
			best = id
			bestRange = r
		}
	}
	return best
}

func callCalleeNode(tree *ast.AST, callID ast.NodeID) ast.NodeID {
	if tree == nil || callID == ast.NoNode {
		return ast.NoNode
	}
	return tree.ChildAt(callID, 0)
}

func callArgNodes(tree *ast.AST, callID ast.NodeID) []ast.NodeID {
	if tree == nil || callID == ast.NoNode {
		return nil
	}
	children := tree.Children(callID)
	if len(children) <= 1 {
		return nil
	}
	return children[1:]
}

func callableSymbolAtCall(doc *Document, callID ast.NodeID) *a.Symbol {
	if doc == nil || doc.Tree == nil || callID == ast.NoNode {
		return nil
	}
	callee := callCalleeNode(doc.Tree, callID)
	if callee == ast.NoNode {
		return nil
	}

	var sym *a.Symbol
	switch doc.Tree.Node(callee).Kind {
	case ast.NodeName:
		sym = doc.Symbols[callee]
		if sym == nil {
			sym = doc.Defs[callee]
		}
	case ast.NodeAttribute:
		sym = doc.AttrSymbols[callee]
	}
	if sym == nil || sym.Kind != a.SymFunction {
		return nil
	}
	return sym
}

func orderedParams(sym *a.Symbol) []*a.Symbol {
	if sym == nil || sym.Inner == nil {
		return nil
	}
	params := make([]*a.Symbol, 0, len(sym.Inner.Symbols))
	for _, inner := range sym.Inner.Symbols {
		if inner != nil && inner.Kind == a.SymParameter {
			params = append(params, inner)
		}
	}
	sort.Slice(params, func(i, j int) bool {
		if params[i].Span.Start != params[j].Span.Start {
			return params[i].Span.Start < params[j].Span.Start
		}
		return params[i].Name < params[j].Name
	})
	return params
}

func formatSignatureParam(sym *a.Symbol) string {
	if sym == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(sym.Name)
	if typeText := formatHoverType(a.SymbolType(sym)); typeText != "" {
		b.WriteString(": ")
		b.WriteString(typeText)
	}
	if sym.DefaultValue != "" {
		b.WriteString(" = ")
		b.WriteString(sym.DefaultValue)
	}
	return b.String()
}

func signatureLabel(sym *a.Symbol) (string, []lsp.ParameterInformation) {
	if sym == nil {
		return "", nil
	}
	params := orderedParams(sym)
	parts := make([]string, 0, len(params))
	paramInfos := make([]lsp.ParameterInformation, 0, len(params))
	for _, param := range params {
		label := formatSignatureParam(param)
		parts = append(parts, label)
		paramInfos = append(paramInfos, lsp.ParameterInformation{Label: label})
	}
	name := sym.Name
	if cls := classOwner(sym.Scope); cls != nil {
		name = cls.Name + "." + name
	}
	label := name + "(" + strings.Join(parts, ", ") + ")"
	if returns := formatHoverType(sym.Returns); returns != "" {
		label += " -> " + returns
	}
	return label, paramInfos
}

func activeParameterForCall(tree *ast.AST, callID ast.NodeID, pos int, params []*a.Symbol) int {
	if tree == nil || callID == ast.NoNode || len(params) == 0 {
		return 0
	}
	args := callArgNodes(tree, callID)
	active := 0
	for i, arg := range args {
		r := tree.RangeOf(arg)
		if pos < int(r.Start) {
			break
		}
		if l.Contains(r, pos) {
			if tree.Node(arg).Kind == ast.NodeKeywordArg {
				nameNode := tree.ChildAt(arg, 0)
				if nameNode != ast.NoNode {
					if name, ok := tree.NameText(nameNode); ok {
						for j, param := range params {
							if param.Name == name {
								return j
							}
						}
					}
				}
			}
			active = i
			break
		}
		active = i + 1
	}
	if active >= len(params) {
		return len(params) - 1
	}
	if active < 0 {
		return 0
	}
	return active
}

func (s *Server) SignatureHelp(p *lsp.SignatureHelpParams) (*lsp.SignatureHelp, *jsonrpc.Error) {
	doc := s.Get(p.TextDocument.URI)
	if doc == nil {
		return nil, jsonrpc.InvalidParamsError(nil)
	}

	doc.mu.RLock()
	defer doc.mu.RUnlock()
	if doc.Tree == nil || doc.LineIndex == nil {
		return nil, jsonrpc.InvalidParamsError(nil)
	}

	offset := doc.LineIndex.PositionToOffset(p.Position.Line, p.Position.Character)
	callID := innermostEnclosingCall(doc.Tree, offset)
	if callID == ast.NoNode {
		return nil, jsonrpc.InvalidParamsError(nil)
	}
	sym := callableSymbolAtCall(doc, callID)
	if sym == nil {
		return nil, jsonrpc.InvalidParamsError(nil)
	}
	label, params := signatureLabel(sym)
	if label == "" {
		return nil, jsonrpc.InvalidParamsError(nil)
	}
	ordered := orderedParams(sym)
	active := activeParameterForCall(doc.Tree, callID, offset, ordered)
	return &lsp.SignatureHelp{
		Signatures: []lsp.SignatureInformation{{
			Label:      label,
			Parameters: params,
		}},
		ActiveSignature: 0,
		ActiveParameter: active,
	}, nil
}
