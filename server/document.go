package server

import (
	"strings"

	"rahu/analyser"
	"rahu/jsonrpc"
	"rahu/lsp"
	"rahu/parser"
)

type Document struct {
	URI     lsp.DocumentURI
	Version int
	Text    string
	Lines   []string

	AST     *parser.Module
	Symbols map[*parser.Name]*analyser.Symbol
	SemErrs []analyser.SemanticError
}

func (s *Server) Open(item lsp.TextDocumentItem) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.docs[item.URI] = &Document{
		URI:     item.URI,
		Version: item.Version,
		Text:    item.Text,
		Lines:   strings.Split(item.Text, "\n"),
	}
}

func (s *Server) Get(uri lsp.DocumentURI) *Document {
	s.mu.RLock()
	defer s.mu.RUnlock()

	doc, ok := s.docs[uri]
	if !ok {
		return nil
	}

	d := *doc
	return &d
}

func (s *Server) Close(uri lsp.DocumentURI) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.docs, uri)
}

func (s *Server) Update(uri lsp.DocumentURI, text string, version int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, ok := s.docs[uri]
	if !ok {
		return
	}

	if version <= doc.Version {
		return
	}

	doc.Text = text
	doc.Version = version
	doc.Lines = strings.Split(text, "\n")
}

func (s *Server) ApplyFullChange(
	uri lsp.DocumentURI,
	changes []lsp.TextDocumentContentChangeEvent,
	version int,
) {
	if len(changes) == 0 {
		return
	}

	c := changes[0]

	if c.Range == nil {
		s.Update(uri, c.Text, version)
		return
	}

	s.ApplyIncremental(uri, changes, version)
}

func (s *Server) ApplyIncremental(
	uri lsp.DocumentURI,
	changes []lsp.TextDocumentContentChangeEvent,
	version int,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, ok := s.docs[uri]
	if !ok {
		return
	}

	text := doc.Text

	for _, c := range changes {
		if c.Range == nil {
			text = c.Text
			continue
		}

		text = applyRangeEdit(text, *c.Range, c.Text)
	}

	doc.Text = text
	doc.Version = version
	doc.Lines = strings.Split(text, "\n")
}

func applyRangeEdit(old string, r lsp.Range, newText string) string {
	lines := strings.Split(old, "\n")
	if r.Start.Line >= len(lines) {
		return old
	}
	startLine := lines[r.Start.Line]

	if r.Start.Character > len(startLine) {
		r.Start.Character = len(startLine)
	}

	prefix := startLine[:r.Start.Character]

	endLine := lines[r.End.Line]

	if r.End.Character > len(endLine) {
		r.End.Character = len(endLine)
	}

	suffix := endLine[r.End.Character:]

	var out strings.Builder

	for i := 0; i < r.Start.Line; i++ {
		out.WriteString(lines[i])
		out.WriteByte('\n')
	}

	out.WriteString(prefix)
	out.WriteString(newText)
	out.WriteString(suffix)

	for i := r.End.Line + 1; i < len(lines); i++ {
		out.WriteByte('\n')
		out.WriteString(lines[i])
	}

	return out.String()
}

func (s *Server) Initialize(
	p *lsp.InitializeParams,
) (*lsp.InitializeResult, *jsonrpc.Error) {
	s.mu.Lock()
	s.capabilities = p.Capabilities
	s.mu.Unlock()

	return &lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			TextDocumentSync: lsp.TDSKFull,
			HoverProvider:    true,
		},
	}, nil
}

func (s *Server) Initialized(_ *struct{}) {}

func (s *Server) Shutdown(_ *struct{}) (*struct{}, *jsonrpc.Error) {
	return &struct{}{}, nil
}

func (s *Server) Exit(_ *struct{}) {}
