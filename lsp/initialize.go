package lsp

import (
	j "encoding/json"
)

type ClientInfo struct {
	Name    string  `json:"name"`
	Version *string `json:"version,omitempty"`
}

type TextDocumentClientCapabilities struct{}

type ClientCapabilities struct {
	Workspace    *WorkspaceClientCapabilities    `json:"workspace,omitempty"`
	TextDocument *TextDocumentClientCapabilities `json:"textDocument,omitempty"`
	Window       j.RawMessage                    `json:"window,omitempty"`
	General      j.RawMessage                    `json:"general,omitempty"`
}

type InitializeParams struct {
	WorkDoneToken         ProgressToken      `json:"workDoneToken,omitempty"`
	ProcessID             *int               `json:"processId,omitempty"`
	ClientInfo            *ClientInfo        `json:"clientInfo,omitempty"`
	Locale                *string            `json:"locale,omitempty"`
	RootPath              *string            `json:"rootPath,omitempty"`
	RootURI               *DocumentURI       `json:"rootUri,omitempty"`
	InitializationOptions j.RawMessage       `json:"initializationOptions,omitempty"`
	Capabilities          ClientCapabilities `json:"capabilities"`
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   *ClientInfo        `json:"serverInfo,omitempty"`
}

type ServerCapabilities struct {
	TextDocumentSync   TextDocumentSyncKind `json:"textDocumentSync"`
	HoverProvider      bool                 `json:"hoverProvider"`
	CompletionProvider map[string]any       `json:"completionProvider,omitempty"`
	DefinitionProvider bool                 `json:"definitionProvider"`
}
