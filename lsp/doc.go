// Package lsp defines Go representations of the Language Server Protocol (LSP)
// data structures for JSON-RPC communication. The types in this package are
// designed to closely follow the LSP specification and preserve wire-level
// compatibility.
//
// This package provides a structured and type-safe way to handle LSP messages,
// covering various aspects of the protocol:
//
//   - Core Structures: Common types such as Position, Range, Location, and MarkupContent (common.go).
//   - Initialization: Types for the 'initialize' request, including ClientInfo and ClientCapabilities (initialize.go).
//   - Text Synchronization: Structures for handling text document identifiers, versions, content changes, and sync kinds (text_document.go).
//   - Diagnostics: Types for representing code diagnostics, including severity levels and error codes (diagnostics.go).
//   - Workspace Management: Support for workspace edits, configuration changes, and file system watchers (workspace.go).
//   - Window & UI: Structures for window interactions like progress reporting and hover information (window.go).
//
// These types facilitate the development of LSP servers and clients in Go by
// ensuring correct serialization and deserialization of protocol messages.
package lsp
