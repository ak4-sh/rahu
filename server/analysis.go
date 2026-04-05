package server

import (
	"rahu/analyser"
	"rahu/lsp"
	"rahu/parser"
	"rahu/source"
)

func (s *Server) analyze(doc *Document) {
	p := parser.New(doc.Text)
	tree := p.Parse()

	global, defs := analyser.BuildScopes(tree)
	resolver, semErrs := analyser.Resolve(tree, global)

	s.SetAnalysis(doc.URI, tree, defs, resolver.Resolved, resolver.ResolvedAttr, semErrs)

	diags := toDiagnostics(doc.LineIndex, p.Errors(), semErrs)
	s.publishDiagnostics(doc.URI, diags)
}

func toDiagnostics(
	li *source.LineIndex,
	parseErrs []parser.Error,
	semErrs []analyser.SemanticError,
) []lsp.Diagnostic {
	diags := make([]lsp.Diagnostic, 0, len(parseErrs)+len(semErrs))

	for _, e := range parseErrs {
		diags = append(diags, lsp.Diagnostic{
			Range:    ToRange(li, e.Span),
			Severity: lsp.SeverityError,
			Message:  e.Msg,
			Source:   "parser",
		})
	}

	for _, e := range semErrs {
		diags = append(diags, lsp.Diagnostic{
			Range:    ToRange(li, e.Span),
			Severity: lsp.SeverityError,
			Message:  e.Msg,
			Source:   "semantic",
		})
	}

	return diags
}
