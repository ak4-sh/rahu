package server

import (
	"context"
	"errors"
	"sync"
	"time"

	"rahu/analyser"
	"rahu/jsonrpc"
	"rahu/lsp"
	"rahu/parser"
	ast "rahu/parser/ast"
	"rahu/source"
)

// ErrIndexingTimeout is returned when WaitForIndexing times out.
var ErrIndexingTimeout = errors.New("indexing timeout")

type Server struct {
	// Document map lock - protects docs map (add/remove documents)
	docsMu sync.RWMutex
	docs   map[lsp.DocumentURI]*Document

	// Module index lock - protects module file index (read-heavy, written at startup)
	indexMu                sync.RWMutex
	modulesByName          map[string]ModuleFile
	modulesByURI           map[lsp.DocumentURI]ModuleFile
	externalModulesByName  map[string]ModuleFile
	externalModulesByURI   map[lsp.DocumentURI]ModuleFile
	pythonBuiltinNames     map[string]struct{}
	pythonModuleInfoByName map[string]pythonModuleInfo
	externalSearchRoots    []string
	pythonExecutable       string

	// Snapshots lock - protects module analysis cache (read-heavy)
	snapshotsMu           sync.RWMutex
	moduleSnapshotsByName map[string]*ModuleSnapshot
	moduleSnapshotsByURI  map[lsp.DocumentURI]*ModuleSnapshot
	buildingModules       map[string]chan struct{}
	openModuleCounts      map[lsp.DocumentURI]int
	snapshotLRU           *snapshotLRU
	maxCachedModules      int

	// Dependencies lock - protects import/dependency graph
	depsMu              sync.RWMutex
	moduleImportsByURI  map[lsp.DocumentURI][]string
	reverseDepsByModule map[string]map[lsp.DocumentURI]struct{}

	// Misc lock - protects low-contention miscellaneous state
	miscMu                   sync.Mutex
	debounce                 map[lsp.DocumentURI]*time.Timer
	capabilities             lsp.ClientCapabilities
	rootURI                  lsp.DocumentURI
	rootPath                 string
	priorityDir              string
	workspaceIndexedNotified bool
	indexingCtx              context.Context
	indexingCancel           context.CancelFunc
	indexingDone             chan struct{}

	// Reference index for fast O(1) reference lookups
	refIndex *RefIndex

	conn *jsonrpc.Conn
}

type ModuleFile struct {
	Name string
	URI  lsp.DocumentURI
	Path string
	Kind string
}

type ModuleSnapshot struct {
	Name        string
	URI         lsp.DocumentURI
	Path        string
	LineIndex   *source.LineIndex
	Tree        *ast.AST
	ParseErrs   []parser.Error
	Symbols     map[ast.NodeID]*analyser.Symbol
	AttrSymbols map[ast.NodeID]*analyser.Symbol
	Defs        map[ast.NodeID]*analyser.Symbol
	SemErrs     []analyser.SemanticError
	Global      *analyser.Scope
	Exports     map[string]*analyser.Symbol
	ExportHash  uint64
	Imports     []string
}

func New(conn *jsonrpc.Conn) *Server {
	return &Server{
		conn:                   conn,
		docs:                   make(map[lsp.DocumentURI]*Document),
		debounce:               make(map[lsp.DocumentURI]*time.Timer),
		modulesByName:          make(map[string]ModuleFile),
		modulesByURI:           make(map[lsp.DocumentURI]ModuleFile),
		externalModulesByName:  make(map[string]ModuleFile),
		externalModulesByURI:   make(map[lsp.DocumentURI]ModuleFile),
		pythonBuiltinNames:     make(map[string]struct{}),
		pythonModuleInfoByName: make(map[string]pythonModuleInfo),
		moduleImportsByURI:     make(map[lsp.DocumentURI][]string),
		reverseDepsByModule:    make(map[string]map[lsp.DocumentURI]struct{}),
		buildingModules:        make(map[string]chan struct{}),
		openModuleCounts:       make(map[lsp.DocumentURI]int),
		moduleSnapshotsByName:  make(map[string]*ModuleSnapshot),
		moduleSnapshotsByURI:   make(map[lsp.DocumentURI]*ModuleSnapshot),
		snapshotLRU:            newSnapshotLRU(),
		maxCachedModules:       defaultMaxCachedModules,
		refIndex:               NewRefIndex(),
	}
}

// WaitForIndexing blocks until background indexing completes or times out (30s).
// Returns nil if indexing completed, ErrIndexingTimeout if timed out.
func (s *Server) WaitForIndexing() error {
	s.miscMu.Lock()
	done := s.indexingDone
	s.miscMu.Unlock()

	if done == nil {
		// Indexing never started (no workspace) or already complete
		return nil
	}

	select {
	case <-done:
		return nil
	case <-time.After(30 * time.Second):
		return ErrIndexingTimeout
	}
}
