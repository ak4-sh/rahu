package server

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"rahu/analyser"
	"rahu/jsonrpc"
	"rahu/lsp"
	ast "rahu/parser/ast"
	"rahu/server/locate"
	"rahu/source"
)

type Document struct {
	mu sync.RWMutex // Protects all fields below

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
	PosIndex    *locate.PositionIndex // O(log n) position-to-node lookup
}

func (s *Server) Open(item lsp.TextDocumentItem) {
	doc := &Document{
		URI:       item.URI,
		Version:   item.Version,
		Text:      item.Text,
		LineIndex: source.NewLineIndex(item.Text),
	}

	s.docsMu.Lock()
	s.docs[item.URI] = doc
	s.docsMu.Unlock()

	if _, isModule := s.LookupModuleByURI(item.URI); isModule {
		s.snapshotsMu.Lock()
		s.openModuleCounts[item.URI]++
		s.snapshotsMu.Unlock()
	}
}

func (s *Server) Get(uri lsp.DocumentURI) *Document {
	s.docsMu.RLock()
	doc := s.docs[uri]
	s.docsMu.RUnlock()
	return doc
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
	doc := s.Get(uri)
	if doc == nil {
		return
	}

	// Build position index outside the lock (read-only tree access)
	posIndex := locate.Build(tree)

	doc.mu.Lock()
	doc.Tree = tree
	doc.Global = global
	doc.Symbols = symbols
	doc.SemErrs = semErrs
	doc.AttrSymbols = attrSymbols
	doc.Defs = defs
	doc.PosIndex = posIndex
	lineIndex := doc.LineIndex
	doc.mu.Unlock()

	// Update reference index with new analysis results
	s.refIndex.IndexDocument(uri, tree, lineIndex, symbols, attrSymbols, defs)
}

func (s *Server) Close(uri lsp.DocumentURI) {
	// Stop debounce timer (separate lock scope)
	s.miscMu.Lock()
	if t, ok := s.debounce[uri]; ok {
		t.Stop()
		delete(s.debounce, uri)
	}
	s.miscMu.Unlock()

	// Remove from docs map (separate lock scope)
	s.docsMu.Lock()
	delete(s.docs, uri)
	s.docsMu.Unlock()

	if _, isModule := s.LookupModuleByURI(uri); isModule {
		s.snapshotsMu.Lock()
		if count := s.openModuleCounts[uri]; count <= 1 {
			delete(s.openModuleCounts, uri)
		} else {
			s.openModuleCounts[uri] = count - 1
		}
		s.snapshotsMu.Unlock()
	}

	// If not a workspace module, remove from reference index
	// (workspace modules keep their snapshot-based index)
	if _, isModule := s.LookupModuleByURI(uri); !isModule {
		s.refIndex.RemoveDocument(uri)
	}
}

func (s *Server) Update(uri lsp.DocumentURI, text string, version int) {
	doc := s.Get(uri)
	if doc == nil {
		return
	}

	doc.mu.Lock()
	defer doc.mu.Unlock()

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
	doc := s.Get(uri)
	if doc == nil {
		return
	}

	doc.mu.Lock()
	defer doc.mu.Unlock()

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

	s.miscMu.Lock()
	s.capabilities = p.Capabilities
	s.rootURI = rootURI
	s.rootPath = rootPath
	s.priorityDir = rootPath // Default priority to workspace root
	s.miscMu.Unlock()

	env := discoverPythonEnv(rootPath)
	roots := normalizeExternalSearchRoots(rootPath, env.Paths)
	s.indexMu.Lock()
	s.pythonExecutable = env.Executable
	s.externalSearchRoots = roots
	s.indexMu.Unlock()

	// Indexing will start in backgroundIndex() triggered by Initialized

	return &lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			TextDocumentSync:      lsp.TDSKFull,
			HoverProvider:         true,
			CompletionProvider:    map[string]any{"triggerCharacters": []string{"."}},
			SignatureHelpProvider: map[string]any{"triggerCharacters": []string{"(", ","}},
			SemanticTokensProvider: map[string]any{
				"legend": map[string]any{
					"tokenTypes":     semanticTokenLegendTypes,
					"tokenModifiers": semanticTokenLegendModifiers,
				},
				"full": true,
			},
			DefinitionProvider:      true,
			ReferencesProvider:      true,
			RenameProvider:          map[string]any{"prepareProvider": true},
			DocumentSymbolProvider:  true,
			WorkspaceSymbolProvider: true,
		},
	}, nil
}

func (s *Server) Initialized(_ *struct{}) {
	s.miscMu.Lock()
	if s.workspaceIndexedNotified {
		s.miscMu.Unlock()
		return
	}
	s.workspaceIndexedNotified = true

	// Create cancellation context and done channel
	ctx, cancel := context.WithCancel(context.Background())
	s.indexingCtx = ctx
	s.indexingCancel = cancel
	s.indexingDone = make(chan struct{})
	s.miscMu.Unlock()

	go s.backgroundIndex(ctx)
}

func (s *Server) backgroundIndex(ctx context.Context) {
	defer close(s.indexingDone)

	s.createWorkspaceIndexingProgress()
	s.beginWorkspaceIndexingProgress()

	moduleIndexStart := time.Now()
	if err := s.buildModuleIndexWithContext(ctx); err != nil {
		s.endWorkspaceIndexingProgress()
		return
	}
	moduleIndexDuration := time.Since(moduleIndexStart)

	workspaceStart := time.Now()
	if err := s.buildWorkspaceSnapshotsWithPriority(ctx); err != nil {
		s.endWorkspaceIndexingProgress()
		return
	}
	workspaceDuration := time.Since(workspaceStart)

	s.endWorkspaceIndexingProgress()

	// Re-analyze all open documents to pick up cross-file imports
	reanalyzeStart := time.Now()
	s.reanalyzeOpenDocuments()
	reanalyzeDuration := time.Since(reanalyzeStart)
	if s.conn != nil {
		log.Printf("INDEX: module_index=%s workspace=%s reanalyze_open_docs=%s", moduleIndexDuration, workspaceDuration, reanalyzeDuration)
	}
}

// reanalyzeOpenDocuments re-analyzes all currently open documents.
// Called after background indexing completes to update cross-file imports.
func (s *Server) reanalyzeOpenDocuments() {
	s.docsMu.RLock()
	docs := make([]*Document, 0, len(s.docs))
	for _, doc := range s.docs {
		docs = append(docs, doc)
	}
	s.docsMu.RUnlock()

	for _, doc := range docs {
		if doc != nil {
			s.analyze(doc)
		}
	}
}

func (s *Server) Shutdown(_ *struct{}) (*struct{}, *jsonrpc.Error) {
	s.miscMu.Lock()
	cancel := s.indexingCancel
	s.miscMu.Unlock()

	if cancel != nil {
		cancel()
	}

	return &struct{}{}, nil
}

func (s *Server) Exit(_ *struct{}) {}
