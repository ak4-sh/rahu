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
	li := doc.LineIndex

	for _, c := range changes {
		if c.Range == nil {
			text = c.Text
			li = source.NewLineIndex(text)
			continue
		}

		startOff := li.PositionToOffset(c.Range.Start.Line, c.Range.Start.Character)
		endOff := li.PositionToOffset(c.Range.End.Line, c.Range.End.Character)
		// Clamp to valid text range.
		if startOff > len(text) {
			startOff = len(text)
		}
		if endOff > len(text) {
			endOff = len(text)
		}

		li = li.ApplyEdit(startOff, endOff, c.Text)
		var b strings.Builder
		b.Grow(startOff + len(c.Text) + len(text) - endOff)
		b.WriteString(text[:startOff])
		b.WriteString(c.Text)
		b.WriteString(text[endOff:])
		text = b.String()
	}

	doc.Text = text
	doc.Version = version
	doc.LineIndex = li
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
	builtins := make(map[string]struct{}, len(env.Builtins))
	for _, name := range env.Builtins {
		if name == "" {
			continue
		}
		builtins[name] = struct{}{}
	}
	s.indexMu.Lock()
	s.pythonExecutable = env.Executable
	s.externalSearchRoots = roots
	s.pythonBuiltinNames = builtins
	s.pythonModuleInfoByName = make(map[string]pythonModuleInfo)
	s.indexMu.Unlock()

	// Indexing will start in backgroundIndex() triggered by Initialized

	return &lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			TextDocumentSync:      lsp.TDSKIncremental,
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

func newStartupReadiness() *startupReadiness {
	return &startupReadiness{
		priorityModuleNames: make(map[string]struct{}),
		priorityOpenURIs:    make(map[lsp.DocumentURI]struct{}),
		firstDiagAtByURI:    make(map[lsp.DocumentURI]time.Time),
		firstApplyAtByURI:   make(map[lsp.DocumentURI]time.Time),
	}
}

func (s *Server) resetStartupReadiness() {
	s.miscMu.Lock()
	s.startup = newStartupReadiness()
	s.miscMu.Unlock()
}

func (s *Server) markOpenDocumentSnapshotApplied(uri lsp.DocumentURI) {
	s.miscMu.Lock()
	defer s.miscMu.Unlock()
	if s.startup == nil {
		return
	}
	if _, ok := s.startup.priorityOpenURIs[uri]; !ok {
		return
	}
	if s.startup.firstApplyAtByURI[uri].IsZero() {
		s.startup.firstApplyAtByURI[uri] = time.Now()
	}
	s.markPriorityReadyIfSatisfiedLocked()
}

func (s *Server) markOpenDocumentDiagnosticsPublished(uri lsp.DocumentURI) {
	s.miscMu.Lock()
	defer s.miscMu.Unlock()
	if s.startup == nil {
		return
	}
	if _, ok := s.startup.priorityOpenURIs[uri]; !ok {
		return
	}
	if s.startup.firstDiagAtByURI[uri].IsZero() {
		s.startup.firstDiagAtByURI[uri] = time.Now()
	}
	s.markPriorityReadyIfSatisfiedLocked()
}

func (s *Server) markPriorityReadyIfSatisfied() {
	s.miscMu.Lock()
	defer s.miscMu.Unlock()
	s.markPriorityReadyIfSatisfiedLocked()
}

func (s *Server) markPriorityReadyIfSatisfiedLocked() {
	if s.startup == nil || !s.startup.allOpenFilesReadyAt.IsZero() {
		return
	}
	if len(s.startup.priorityOpenURIs) == 0 {
		return
	}
	for uri := range s.startup.priorityOpenURIs {
		if s.startup.firstApplyAtByURI[uri].IsZero() || s.startup.firstDiagAtByURI[uri].IsZero() {
			return
		}
	}
	s.startup.allOpenFilesReadyAt = time.Now()
}

func (s *Server) backgroundIndex(ctx context.Context) {
	defer close(s.indexingDone)
	s.resetStartupReadiness()

	s.createWorkspaceIndexingProgress()
	s.beginWorkspaceIndexingProgress()

	moduleIndexStart := time.Now()
	if err := s.buildModuleIndexWithContext(ctx); err != nil {
		s.endWorkspaceIndexingProgress()
		return
	}
	moduleIndexDuration := time.Since(moduleIndexStart)

	workspaceStart := time.Now()
	if err := s.buildWorkspaceSnapshotsWithPriority(ctx, s.indexingCancel); err != nil {
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
		s.miscMu.Lock()
		startup := s.startup
		priorityReadyAt := time.Time{}
		allOpenReadyAt := time.Time{}
		priorityCount := 0
		priorityRounds := 0
		if startup != nil {
			priorityReadyAt = startup.priorityReadyAt
			allOpenReadyAt = startup.allOpenFilesReadyAt
			priorityCount = startup.priorityModuleCount
			priorityRounds = startup.prioritySurfaceRounds
		}
		s.miscMu.Unlock()
		priorityReadyDuration := time.Duration(0)
		allOpenReadyDuration := time.Duration(0)
		if !priorityReadyAt.IsZero() {
			priorityReadyDuration = priorityReadyAt.Sub(workspaceStart)
		}
		if !allOpenReadyAt.IsZero() {
			allOpenReadyDuration = allOpenReadyAt.Sub(workspaceStart)
		}
		log.Printf("INDEX: module_index=%s workspace=%s priority_ready=%s all_open_ready=%s priority_modules=%d priority_rounds=%d reanalyze_open_docs=%s", moduleIndexDuration, workspaceDuration, priorityReadyDuration, allOpenReadyDuration, priorityCount, priorityRounds, reanalyzeDuration)
	}
}

// reanalyzeOpenDocuments updates all currently open documents after background
// indexing completes. Workspace modules reuse the snapshot just built; other
// documents get a fresh analysis.
func (s *Server) reanalyzeOpenDocuments() {
	s.docsMu.RLock()
	docs := make([]*Document, 0, len(s.docs))
	for _, doc := range s.docs {
		docs = append(docs, doc)
	}
	s.docsMu.RUnlock()

	for _, doc := range docs {
		if doc == nil {
			continue
		}
		if _, isModule := s.LookupModuleByURI(doc.URI); isModule {
			// Snapshot was just built in workspace indexing — apply it directly
			// instead of rebuilding from scratch. If the user edited the doc
			// during indexing, the debounce timer already queued a re-analysis.
			if snapshot, ok := s.getModuleSnapshotByURI(doc.URI); ok {
				s.applySnapshotToOpenDocument(snapshot)
				s.publishDiagnostics(doc.URI, toDiagnostics(snapshot.LineIndex, snapshot.ParseErrs, snapshot.SemErrs))
				continue
			}
		}
		s.analyze(doc)
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
