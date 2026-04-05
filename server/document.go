package server

import (
	"strings"

	"rahu/analyser"
	"rahu/jsonrpc"
	"rahu/lsp"
	ast "rahu/parser/ast"
	"rahu/source"
)

type Document struct {
	URI       lsp.DocumentURI
	Version   int
	Text      string
	LineIndex *source.LineIndex

	Tree        *ast.AST
	Global      *analyser.Scope
	Symbols     map[ast.NodeID]*analyser.Symbol
	SemErrs     []analyser.SemanticError
	AttrSymbols map[ast.NodeID]*analyser.Symbol
	Defs        map[ast.NodeID]*analyser.Symbol
}

func (s *Server) Open(item lsp.TextDocumentItem) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.docs[item.URI] = &Document{
		URI:       item.URI,
		Version:   item.Version,
		Text:      item.Text,
		LineIndex: source.NewLineIndex(item.Text),
	}
}

func (s *Server) Get(uri lsp.DocumentURI) *Document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.docs[uri]
}

func (s *Server) SetAnalysis(
	uri lsp.DocumentURI,
	tree *ast.AST,
	global *analyser.Scope,
	defs map[ast.NodeID]*analyser.Symbol,
	symbols map[ast.NodeID]*analyser.Symbol,
	attrSymbols map[ast.NodeID]*analyser.Symbol,
	semErrs []analyser.SemanticError,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc, ok := s.docs[uri]
	if !ok {
		return
	}

	doc.Tree = tree
	doc.Global = global
	doc.Symbols = symbols
	doc.SemErrs = semErrs
	doc.AttrSymbols = attrSymbols
	doc.Defs = defs
}

func (s *Server) Close(uri lsp.DocumentURI) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if t, ok := s.debounce[uri]; ok {
		t.Stop()
		delete(s.debounce, uri)
	}
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
	doc.LineIndex = source.NewLineIndex(text)
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
	if version <= doc.Version {
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
	doc.LineIndex = source.NewLineIndex(text)
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
	rootURI := lsp.DocumentURI("")
	rootPath := ""
	if p.RootURI != nil {
		rootURI = *p.RootURI
		if path, ok := uriToPath(rootURI); ok {
			rootPath = path
		}
	}

	s.mu.Lock()
	s.capabilities = p.Capabilities
	s.rootURI = rootURI
	s.rootPath = rootPath
	s.mu.Unlock()

	s.buildModuleIndex()
	s.buildWorkspaceSnapshots()

	return &lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			TextDocumentSync:        lsp.TDSKFull,
			HoverProvider:           true,
			CompletionProvider:      map[string]any{"triggerCharacters": []string{"."}},
			DefinitionProvider:      true,
			ReferencesProvider:      true,
			RenameProvider:          map[string]any{"prepareProvider": true},
			DocumentSymbolProvider:  true,
			WorkspaceSymbolProvider: true,
		},
	}, nil
}

func (s *Server) Initialized(_ *struct{}) {
	s.mu.Lock()
	if s.workspaceIndexedNotified {
		s.mu.Unlock()
		return
	}
	s.workspaceIndexedNotified = true
	s.mu.Unlock()

	s.createWorkspaceIndexingProgress()
	s.beginWorkspaceIndexingProgress()
	s.endWorkspaceIndexingProgress()
}

func (s *Server) Shutdown(_ *struct{}) (*struct{}, *jsonrpc.Error) {
	return &struct{}{}, nil
}

func (s *Server) Exit(_ *struct{}) {}
