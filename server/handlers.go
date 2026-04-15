package server

import (
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"rahu/jsonrpc"

	a "rahu/analyser"
	"rahu/lsp"
	"rahu/parser/ast"
	l "rahu/server/locate"
	"rahu/source"
)

func (s *Server) DidOpen(p *lsp.DidOpenTextDocumentParams) {
	s.Open(p.TextDocument)
	doc := s.Get(p.TextDocument.URI)
	if doc != nil {
		s.analyze(doc)
	}
}

func classOwner(scope *a.Scope) *a.Symbol {
	for s := scope; s != nil; s = s.Parent {
		if s.Kind == a.ScopeClass && s.Owner != nil {
			return s.Owner
		}
	}
	return nil
}

func (s *Server) hoverTarget(sym *a.Symbol, current *Document) (lsp.DocumentURI, *source.LineIndex) {
	if current == nil {
		return "", nil
	}
	if sym == nil || sym.URI == "" || sym.URI == current.URI {
		return current.URI, current.LineIndex
	}
	if li := s.lineIndexForURI(sym.URI); li != nil {
		return sym.URI, li
	}
	return sym.URI, nil
}

func formatHoverType(t *a.Type) string {
	if a.IsUnknownType(t) {
		return ""
	}
	switch t.Kind {
	case a.TypeInstance:
		if t.Symbol != nil {
			return t.Symbol.Name
		}
	case a.TypeBuiltin:
		if t.Symbol != nil {
			return t.Symbol.Name
		}
	case a.TypeModule:
		if t.Symbol != nil {
			return "module " + t.Symbol.Name
		}
	case a.TypeClass:
		if t.Symbol != nil {
			return t.Symbol.Name
		}
	case a.TypeList:
		if elem := formatHoverType(t.Elem); elem != "" {
			return "list[" + elem + "]"
		}
		return "list"
	case a.TypeTuple:
		if len(t.Items) == 0 {
			return "tuple"
		}
		parts := make([]string, 0, len(t.Items))
		for _, item := range t.Items {
			formatted := formatHoverType(item)
			if formatted == "" {
				formatted = "unknown"
			}
			parts = append(parts, formatted)
		}
		return "tuple[" + strings.Join(parts, ", ") + "]"
	case a.TypeDict:
		key := formatHoverType(t.Key)
		if key == "" {
			key = "unknown"
		}
		value := formatHoverType(t.Elem)
		if value == "" {
			value = "unknown"
		}
		return "dict[" + key + ", " + value + "]"
	case a.TypeSet:
		if elem := formatHoverType(t.Elem); elem != "" {
			return "set[" + elem + "]"
		}
		return "set"
	case a.TypeUnion:
		parts := make([]string, 0, len(t.Union))
		for _, arm := range t.Union {
			formatted := formatHoverType(arm)
			if formatted == "" {
				continue
			}
			parts = append(parts, formatted)
		}
		return strings.Join(parts, " | ")
	}
	return ""
}

func (s *Server) hoverForSymbol(doc *Document, sym *a.Symbol) *lsp.Hover {
	var kind string
	switch sym.Kind {
	case a.SymVariable:
		kind = "variable"
	case a.SymParameter:
		kind = "parameter"
	case a.SymFunction:
		kind = "function"
	case a.SymClass:
		kind = "class"
	case a.SymModule:
		kind = "module"
	case a.SymBuiltin:
		kind = "builtin"
	case a.SymType:
		kind = "type"
	case a.SymAttr:
		kind = "field"
	default:
		kind = "symbol"
	}

	var builder strings.Builder
	typeText := ""
	if sym.Kind == a.SymVariable || sym.Kind == a.SymParameter || sym.Kind == a.SymAttr || sym.Kind == a.SymField {
		typeText = formatHoverType(a.SymbolType(sym))
	}
	builder.WriteString("```python\n")
	builder.WriteString(kind)
	builder.WriteString("(")
	builder.WriteString(sym.Name)
	if typeText != "" {
		builder.WriteString(": ")
		builder.WriteString(typeText)
	}
	if sym.DefaultValue != "" {
		builder.WriteString(" = ")
		builder.WriteString(sym.DefaultValue)
	}
	builder.WriteString(")\n```")

	if sym.Kind == a.SymClass && sym.DocString != "" {
		builder.Reset()
		builder.WriteString("```python\n")
		builder.WriteString(kind)
		builder.WriteString("(")
		builder.WriteString(sym.Name)
		builder.WriteString(")\n```\n\n")
		builder.WriteString(sym.DocString)
	}

	if sym.Kind == a.SymFunction && sym.Inner != nil {
		params := []string{}
		for _, p := range sym.Inner.Symbols {
			if p.Kind == a.SymParameter {
				paramStr := p.Name
				if p.IsKwArg {
					paramStr = "**" + paramStr
				} else if p.IsVarArg {
					paramStr = "*" + paramStr
				}
				if p.DefaultValue != "" {
					paramStr += "=" + p.DefaultValue
				}
				params = append(params, paramStr)
			}
		}
		name := sym.Name

		if cls := classOwner(sym.Scope); cls != nil {
			name = cls.Name + "." + name
			kind = "method"
		}

		builder.Reset()
		builder.WriteString("```python\n")
		builder.WriteString(name)
		builder.WriteString("(")
		builder.WriteString(strings.Join(params, ", "))
		builder.WriteString(")\n")

		if sym.DocString != "" {

			builder.WriteString("```\n")
			builder.WriteString(sym.DocString)
		} else {
			builder.WriteString("```")
		}

	}

	if (sym.Kind == a.SymVariable || sym.Kind == a.SymParameter || sym.Kind == a.SymAttr || sym.Kind == a.SymField) && typeText != "" {
		builder.Reset()
		builder.WriteString("```\n")
		builder.WriteString(kind)
		builder.WriteString("(")
		builder.WriteString(sym.Name)
		builder.WriteString(": ")
		builder.WriteString(typeText)
		if sym.DefaultValue != "" {
			builder.WriteString(" = ")
			builder.WriteString(sym.DefaultValue)
		}
		builder.WriteString(")\n```")
	}

	targetURI, targetLineIndex := s.hoverTarget(sym, doc)
	filename := filenameFromURI(targetURI)
	builder.WriteString("\n\n")
	builder.WriteString(filename)
	if targetLineIndex != nil {
		line, _ := targetLineIndex.OffsetToPosition(int(sym.Span.Start))
		builder.WriteString(":")
		builder.WriteString(strconv.Itoa(line + 1))
	}

	return &lsp.Hover{
		Contents: lsp.MarkupContent{
			Kind:  "markdown",
			Value: builder.String(),
		},
	}
}

func filenameFromURI(uri lsp.DocumentURI) string {
	u, err := url.Parse(string(uri))
	if err != nil {
		return string(uri)
	}

	return filepath.Base(u.Path)
}

func symbolAtOffset(doc *Document, offset int) (*a.Symbol, ast.NodeID, bool) {
	if doc == nil || doc.Tree == nil {
		return nil, ast.NoNode, false
	}

	// Use indexed lookup for O(log n) performance when available
	res := l.LocateAtPosIndexed(doc.Tree, offset, doc.PosIndex)
	switch res.Kind {
	case l.AttributeResult:
		sym := doc.AttrSymbols[res.Node]
		return sym, res.Node, true
	case l.NameResult:
		sym := doc.Symbols[res.Node]
		if sym == nil {
			sym = doc.Defs[res.Node]
		}
		return sym, res.Node, false
	default:
		return nil, ast.NoNode, false
	}
}

func (s *Server) importModuleSymbolAtOffset(doc *Document, offset int) *a.Symbol {
	if doc == nil || doc.Tree == nil {
		return nil
	}
	for stmt := doc.Tree.Node(doc.Tree.Root).FirstChild; stmt != ast.NoNode; stmt = doc.Tree.Node(stmt).NextSibling {
		if !l.Contains(doc.Tree.RangeOf(stmt), offset) {
			continue
		}
		switch doc.Tree.Node(stmt).Kind {
		case ast.NodeImport:
			for alias := doc.Tree.Node(stmt).FirstChild; alias != ast.NoNode; alias = doc.Tree.Node(alias).NextSibling {
				target, _ := doc.Tree.AliasParts(alias)
				if target == ast.NoNode || !l.Contains(doc.Tree.RangeOf(target), offset) {
					continue
				}
				moduleName, ok := moduleNameFromExpr(doc.Tree, target)
				if !ok {
					continue
				}
				snapshot, ok := s.analyzeModuleByName(moduleName)
				if !ok {
					continue
				}
				return &a.Symbol{Name: moduleName, Kind: a.SymModule, URI: snapshot.URI, Span: moduleDefSpan(snapshot)}
			}
		case ast.NodeFromImport:
			module, _ := doc.Tree.FromImportParts(stmt)
			if module == ast.NoNode || !l.Contains(doc.Tree.RangeOf(module), offset) {
				continue
			}
			moduleName, ok := s.resolveImportModuleName(doc.URI, doc.Tree, module, doc.Tree.Node(stmt).Data)
			if !ok {
				continue
			}
			snapshot, ok := s.analyzeModuleByName(moduleName)
			if !ok {
				continue
			}
			return &a.Symbol{Name: moduleName, Kind: a.SymModule, URI: snapshot.URI, Span: moduleDefSpan(snapshot)}
		}
	}
	return nil
}

func (s *Server) Hover(p *lsp.HoverParams) (*lsp.Hover, *jsonrpc.Error) {
	doc := s.Get(p.TextDocument.URI)
	if doc == nil {
		return nil, jsonrpc.InvalidParamsError(nil)
	}

	offset := doc.LineIndex.PositionToOffset(
		p.Position.Line,
		p.Position.Character,
	)

	if sym, _, _ := symbolAtOffset(doc, offset); sym != nil {
		hov := s.hoverForSymbol(doc, sym)
		if hov != nil {
			_, targetLineIndex := s.hoverTarget(sym, doc)
			if targetLineIndex != nil && !sym.Span.IsEmpty() {
				hovPos := ToRange(targetLineIndex, sym.Span)
				hov.Range = &hovPos
			}
		}
		return hov, nil
	}
	if sym := s.importModuleSymbolAtOffset(doc, offset); sym != nil {
		hov := s.hoverForSymbol(doc, sym)
		if hov != nil {
			_, targetLineIndex := s.hoverTarget(sym, doc)
			if targetLineIndex != nil && !sym.Span.IsEmpty() {
				hovPos := ToRange(targetLineIndex, sym.Span)
				hov.Range = &hovPos
			}
		}
		return hov, nil
	}
	return nil, jsonrpc.InvalidParamsError(nil)
}

func (s *Server) DidChange(p *lsp.DidChangeTextDocumentParams) {
	doc := s.Get(p.TextDocument.URI)
	if doc == nil {
		return
	}

	if doc.Version >= p.TextDocument.Version {
		return
	}

	s.ApplyFullChange(
		p.TextDocument.URI,
		p.ContentChanges,
		p.TextDocument.Version,
	)

	s.scheduleAnalysis(p.TextDocument.URI)
}

func (s *Server) DidClose(p *lsp.DidCloseTextDocumentParams) {
	_, indexed := s.LookupModuleByURI(p.TextDocument.URI)
	s.Close(p.TextDocument.URI)
	if indexed {
		s.refreshModuleAndDependents(p.TextDocument.URI)
	}
}

// Diagnostic is a stub handler for textDocument/diagnostic (pull model).
// Real diagnostics are delivered via publishDiagnostics (push model).
func (s *Server) Diagnostic(p *lsp.DocumentDiagnosticParams) (*lsp.DocumentDiagnosticReport, *jsonrpc.Error) {
	return &lsp.DocumentDiagnosticReport{
		Kind:  "full",
		Items: []lsp.Diagnostic{},
	}, nil
}

func (s *Server) publishDiagnostics(uri lsp.DocumentURI, diags []lsp.Diagnostic) {
	s.markOpenDocumentDiagnosticsPublished(uri)

	// Skip if no connection available
	if s.conn == nil {
		return
	}

	// If document no longer exists, clear diagnostics
	if s.Get(uri) == nil {
		_ = s.conn.Notify("textDocument/publishDiagnostics",
			lsp.PublishDiagnosticsParams{
				URI:         uri,
				Diagnostics: nil,
			},
		)
		return
	}

	params := lsp.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diags,
	}

	_ = s.conn.Notify("textDocument/publishDiagnostics", params)
}

func (s *Server) Definition(p *lsp.DefinitionParams) (*lsp.Location, *jsonrpc.Error) {
	doc := s.Get(p.TextDocument.URI)
	if doc == nil {
		return nil, jsonrpc.InvalidParamsError(nil)
	}

	offset := doc.LineIndex.PositionToOffset(
		p.Position.Line,
		p.Position.Character,
	)

	if sym, _, _ := symbolAtOffset(doc, offset); sym != nil {
		uri := doc.URI
		if sym.URI != "" {
			uri = sym.URI
		}
		li := s.lineIndexForURI(uri)
		if li == nil {
			return nil, jsonrpc.InvalidParamsError(nil)
		}

		if sym.Kind != a.SymBuiltin &&
			sym.Kind != a.SymConstant &&
			sym.Kind != a.SymType &&
			!sym.Span.IsEmpty() {

			return &lsp.Location{
				URI:   uri,
				Range: ToRange(li, sym.Span),
			}, nil
		}
	}

	if sym := s.importModuleSymbolAtOffset(doc, offset); sym != nil {
		uri := sym.URI
		li := s.lineIndexForURI(uri)
		if li == nil || sym.Span.IsEmpty() {
			return nil, jsonrpc.InvalidParamsError(nil)
		}
		return &lsp.Location{URI: uri, Range: ToRange(li, sym.Span)}, nil
	}

	return nil, jsonrpc.InvalidParamsError(nil)
}

func (s *Server) scheduleAnalysis(uri lsp.DocumentURI) {
	s.miscMu.Lock()
	if t, ok := s.debounce[uri]; ok {
		t.Stop()
	}

	s.debounce[uri] = time.AfterFunc(80*time.Millisecond, func() {
		doc := s.Get(uri)
		if doc != nil {
			s.analyze(doc)
		}
	})

	s.miscMu.Unlock()
}
