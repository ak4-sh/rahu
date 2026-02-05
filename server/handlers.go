package server

import (
	"rahu/jsonrpc"

	"rahu/lsp"
	"rahu/parser"
)

func (s *Server) DidOpen(p *lsp.DidOpenTextDocumentParams) {
	s.Open(p.TextDocument)
	s.publishDiagnostics(p.TextDocument.URI)
}

func (s *Server) Hover(p *lsp.HoverParams) (*lsp.Hover, *jsonrpc.Error) {
	doc := s.Get(p.TextDocument.URI)
	if doc == nil {
		return nil, jsonrpc.InvalidParamsError(nil)
	}

	return &lsp.Hover{
		Contents: "ok",
	}, nil
}

func (s *Server) DidChange(p *lsp.DidChangeTextDocumentParams) {
	s.ApplyFullChange(
		p.TextDocument.URI,
		p.ContentChanges,
		p.TextDocument.Version,
	)
}

func (s *Server) DidClose(p *lsp.DidCloseTextDocumentParams) {
	s.Close(p.TextDocument.URI)
}

func (s *Server) publishDiagnostics(uri lsp.DocumentURI) {
	doc := s.Get(uri)
	if doc == nil {
		return
	}

	p := parser.New(doc.Text)
	p.Parse()

	diags := []lsp.Diagnostic{}
	for _, err := range p.Errors() {
		diags = append(diags, lsp.Diagnostic{
			Range:    err.Span.ToLSPRange(),
			Severity: lsp.SeverityWarning,
		})
	}

	s.conn.Notify()
}
