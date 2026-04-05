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
	builder.WriteString("```python\n")
	builder.WriteString(kind)
	builder.WriteString("(")
	builder.WriteString(sym.Name)
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
				params = append(params, p.Name)
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

	if sym.Kind == a.SymVariable && sym.InstanceOf != nil {
		builder.Reset()
		builder.WriteString("```\n")
		builder.WriteString(kind)
		builder.WriteString("(")
		builder.WriteString(sym.Name)
		builder.WriteString(")\n```")
	}

	filename := filenameFromURI(doc.URI)
	line, _ := doc.LineIndex.OffsetToPosition(int(sym.Span.Start))
	builder.WriteString("\n\n")
	builder.WriteString(filename)
	builder.WriteString(":")
	builder.WriteString(strconv.Itoa(line + 1))

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

	res := l.LocateAtPos(doc.Tree, offset)
	switch res.Kind {
	case l.AttributeResult:
		sym := doc.AttrSymbols[res.Node]
		return sym, res.Node, true
	case l.NameResult:
		sym := doc.Symbols[res.Node]
		return sym, res.Node, false
	default:
		return nil, ast.NoNode, false
	}
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
			hovPos := ToRange(doc.LineIndex, sym.Span)
			hov.Range = &hovPos
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
	s.Close(p.TextDocument.URI)
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
		if sym.Kind != a.SymBuiltin &&
			sym.Kind != a.SymConstant &&
			sym.Kind != a.SymType &&
			!sym.Span.IsEmpty() {

			return &lsp.Location{
				URI:   doc.URI,
				Range: ToRange(doc.LineIndex, sym.Span),
			}, nil
		}
	}

	return nil, jsonrpc.InvalidParamsError(nil)
}

func (s *Server) scheduleAnalysis(uri lsp.DocumentURI) {
	s.mu.Lock()
	if t, ok := s.debounce[uri]; ok {
		t.Stop()
	}

	s.debounce[uri] = time.AfterFunc(80*time.Millisecond, func() {
		doc := s.Get(uri)
		if doc != nil {
			s.analyze(doc)
		}
	})

	s.mu.Unlock()
}
