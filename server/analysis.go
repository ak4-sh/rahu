package server

import (
	"rahu/analyser"
	"rahu/lsp"
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
	diags := make([]lsp.Diagnostic, 0, len(parseErrs)+len(semErrs))

	for _, e := range parseErrs {
		diags = append(diags, lsp.Diagnostic{
			Range:    toRange(e.Span),
			Severity: lsp.SeverityError,
			Message:  e.Msg,
			Source:   "parser",
		})
	}

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

func toRange(r parser.Range) lsp.Range {
	return lsp.Range{
		Start: lsp.Position{
			Line:      r.Start.Line,
			Character: r.Start.Col,
		},
		End: lsp.Position{
			Line:      r.End.Line,
			Character: r.End.Col,
		},
	}
}
