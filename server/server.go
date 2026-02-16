package server

import (
	"sync"
	"time"

	"rahu/jsonrpc"
	"rahu/lsp"
)

type Server struct {
	mu           sync.RWMutex
	docs         map[lsp.DocumentURI]*Document
	capabilities lsp.ClientCapabilities
	conn         *jsonrpc.Conn
	debounce     map[lsp.DocumentURI]*time.Timer
}

func New(conn *jsonrpc.Conn) *Server {
	return &Server{
		conn:     conn,
		docs:     make(map[lsp.DocumentURI]*Document),
		debounce: make(map[lsp.DocumentURI]*time.Timer),
	}
}
