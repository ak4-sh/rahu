package server

import (
	"rahu/analyser"
	"rahu/lexer"
	"rahu/parser"
)

func (s *Server) analyze(doc *Document) {
	p := parser.New(doc.Text)
	module := p.Parse()

	global := analyser.BuildScopes(module)

	semErrs, resolved := analyser.Resolve(module, global)

	doc.AST = module
	doc.Symbols = resolved
	doc.SemErrs = semErrs

	diags := toDiagnostics(p.Errors(), semErrs)

	s.publishDiagnostics(doc.URI, diags)
}

func toDiagnostics(parseErrs []parser.Error, semErrs []analyser.SemanticError) []lsp.Diagnostic {
	var diags []lsp.Diagnostic

	for _, e := range semErrs {
		diags = append(diags, lsp.Diagnostic{
			Range:    toRange(e.Span),
			Severity: lsp.SeverityError,
			Message:  e.Msg,
			Source:   "semantic",
		})
	}

	return diags
}
