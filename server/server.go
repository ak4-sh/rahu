package server

import (
	"sync"
	"time"

	"rahu/analyser"
	"rahu/jsonrpc"
	"rahu/lsp"
	"rahu/parser"
	ast "rahu/parser/ast"
	"rahu/source"
)

type Server struct {
	mu                       sync.RWMutex
	docs                     map[lsp.DocumentURI]*Document
	capabilities             lsp.ClientCapabilities
	conn                     *jsonrpc.Conn
	debounce                 map[lsp.DocumentURI]*time.Timer
	rootURI                  lsp.DocumentURI
	rootPath                 string
	modulesByName            map[string]ModuleFile
	modulesByURI             map[lsp.DocumentURI]ModuleFile
	moduleImportsByURI       map[lsp.DocumentURI][]string
	reverseDepsByModule      map[string]map[lsp.DocumentURI]struct{}
	buildingModules          map[string]bool
	moduleSnapshotsByName    map[string]*ModuleSnapshot
	moduleSnapshotsByURI     map[lsp.DocumentURI]*ModuleSnapshot
	workspaceIndexedNotified bool
}

type ModuleFile struct {
	Name string
	URI  lsp.DocumentURI
	Path string
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
	Imports     []string
}

func New(conn *jsonrpc.Conn) *Server {
	return &Server{
		conn:                  conn,
		docs:                  make(map[lsp.DocumentURI]*Document),
		debounce:              make(map[lsp.DocumentURI]*time.Timer),
		modulesByName:         make(map[string]ModuleFile),
		modulesByURI:          make(map[lsp.DocumentURI]ModuleFile),
		moduleImportsByURI:    make(map[lsp.DocumentURI][]string),
		reverseDepsByModule:   make(map[string]map[lsp.DocumentURI]struct{}),
		buildingModules:       make(map[string]bool),
		moduleSnapshotsByName: make(map[string]*ModuleSnapshot),
		moduleSnapshotsByURI:  make(map[lsp.DocumentURI]*ModuleSnapshot),
	}
}
