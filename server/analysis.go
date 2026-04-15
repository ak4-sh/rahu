package server

import (
	"rahu/analyser"
	"rahu/lsp"
	"rahu/parser"
	"rahu/source"
)

func (s *Server) analyze(doc *Document) {
	if _, ok := s.LookupModuleByURI(doc.URI); ok {
		s.refreshModuleAndDependents(doc.URI)
		return
	}

	snapshot := s.buildModuleSnapshot("", doc.URI, "", doc.Text, doc.LineIndex)
	s.SetAnalysis(doc.URI, snapshot.Tree, snapshot.Global, snapshot.Defs, snapshot.Symbols, snapshot.AttrSymbols, snapshot.SemErrs)
	s.publishDiagnostics(doc.URI, toDiagnostics(doc.LineIndex, snapshot.ParseErrs, snapshot.SemErrs))
}

func (s *Server) analyzeOpenDocumentFast(doc *Document) bool {
	if doc == nil {
		return false
	}

	doc.mu.RLock()
	text := doc.Text
	lineIndex := doc.LineIndex
	version := doc.Version
	doc.mu.RUnlock()

	if _, ok := s.LookupModuleByURI(doc.URI); !ok {
		snapshot := s.buildModuleSnapshot("", doc.URI, "", text, lineIndex)
		s.SetAnalysis(doc.URI, snapshot.Tree, snapshot.Global, snapshot.Defs, snapshot.Symbols, snapshot.AttrSymbols, snapshot.SemErrs)
		s.publishDiagnostics(doc.URI, toDiagnostics(lineIndex, snapshot.ParseErrs, snapshot.SemErrs))
		return false
	}

	snapshot := s.buildBaseModuleSnapshot("", doc.URI, "", text, lineIndex)
	s.SetAnalysis(doc.URI, snapshot.Tree, snapshot.Global, snapshot.Defs, snapshot.Symbols, snapshot.AttrSymbols, snapshot.SemErrs)
	s.publishDiagnostics(doc.URI, toDiagnostics(lineIndex, snapshot.ParseErrs, snapshot.SemErrs))

	s.scheduleAsync(func() {
		uri := doc.URI
		refined := s.Get(uri)
		if refined == nil {
			return
		}
		refined.mu.RLock()
		currentVersion := refined.Version
		refined.mu.RUnlock()
		if currentVersion != version {
			return
		}
		s.refreshModuleAndDependents(uri)
	})

	return true
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
