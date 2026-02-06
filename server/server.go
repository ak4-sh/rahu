package server

import (
	"sync"

	"rahu/jsonrpc"
	"rahu/lsp"
)

type Server struct {
	mu           sync.RWMutex
	docs         map[lsp.DocumentURI]*Document
	capabilities lsp.ClientCapabilities
	conn         *jsonrpc.Conn
}

func New(conn *jsonrpc.Conn) *Server {
	return &Server{
		conn: conn,
		docs: make(map[lsp.DocumentURI]*Document),
	}
}
