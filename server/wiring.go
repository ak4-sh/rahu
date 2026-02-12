package server

import (
	"rahu/jsonrpc"
)

func Register(s *Server) {
	jsonrpc.RegisterRequest(
		"initialize",
		jsonrpc.AdaptRequest(s.Initialize),
	)
	jsonrpc.RegisterNotification(
		"textDocument/didOpen",
		jsonrpc.AdaptNotification(s.DidOpen),
	)

	jsonrpc.RegisterRequest(
		"textDocument/hover",
		jsonrpc.AdaptRequest(s.Hover),
	)

	jsonrpc.RegisterNotification(
		"textDocument/didChange",
		jsonrpc.AdaptNotification(s.DidChange),
	)

	jsonrpc.RegisterNotification(
		"textDocument/didClose",
		jsonrpc.AdaptNotification(s.DidClose),
	)

	jsonrpc.RegisterNotification(
		"initialized",
		jsonrpc.AdaptNotification(s.Initialized),
	)

	jsonrpc.RegisterRequest(
		"shutdown",
		jsonrpc.AdaptRequest(s.Shutdown),
	)

	jsonrpc.RegisterNotification(
		"exit",
		jsonrpc.AdaptNotification(s.Exit),
	)

	jsonrpc.RegisterRequest(
		"textDocument/definition",
		jsonrpc.AdaptRequest(s.Definition),
	)
}
