package server

import (
	"fmt"
	"strings"
	"time"

	"rahu/analyser"
	"rahu/jsonrpc"

	a "rahu/analyser"
	"rahu/lsp"
	l "rahu/server/locate"
)

func (s *Server) DidOpen(p *lsp.DidOpenTextDocumentParams) {
	s.Open(p.TextDocument)
	doc := s.Get(p.TextDocument.URI)
	if doc != nil {
		s.analyze(doc)
	}
}

func (s *Server) hoverForSymbol(doc *Document, sym *analyser.Symbol) *lsp.Hover {
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
	default:
		kind = "symbol"
	}

	value := fmt.Sprintf("**%s** `%s`", kind, sym.Name)

	if sym.Kind == a.SymFunction && sym.Inner != nil {
		params := []string{}
		for _, s := range sym.Inner.Symbols {
			if s.Kind == a.SymParameter {
				params = append(params, s.Name)
			}
		}
		value = fmt.Sprintf(
			"```python\n%s(%s)\n```",
			sym.Name,
			strings.Join(params, ", "),
		)
	}

	line, _ := doc.LineIndex.OffsetToPosition(sym.Span.Start)
	value += fmt.Sprintf("\n\nDefined at line %d", line+1)

	return &lsp.Hover{
		Contents: lsp.MarkupContent{
			Kind:  "markdown",
			Value: value,
		},
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

	if name := l.NameAtPos(doc.AST, offset); name != nil {
		if sym, ok := doc.Symbols[name]; ok && sym != nil {
			return s.hoverForSymbol(doc, sym), nil
		}
	}

	if attr := l.AttributeAtPos(doc.AST, offset); attr != nil {
		if sym, ok := doc.AttrSymbols[attr]; ok && sym != nil {
			return s.hoverForSymbol(doc, sym), nil
		}
	}
	return nil, jsonrpc.InvalidParamsError(nil)
}

func (s *Server) DidChange(p *lsp.DidChangeTextDocumentParams) {
	s.ApplyFullChange(
		p.TextDocument.URI,
		p.ContentChanges,
		p.TextDocument.Version,
	)

	doc := s.Get(p.TextDocument.URI)
	if doc != nil {
		s.analyze(doc)
	}
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

	if name := l.NameAtPos(doc.AST, offset); name != nil {
		if sym, ok := doc.Symbols[name]; ok && sym != nil {
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
	}

	if attr := l.AttributeAtPos(doc.AST, offset); attr != nil {
		if sym, ok := doc.AttrSymbols[attr]; ok && sym != nil && !sym.Span.IsEmpty() {
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
