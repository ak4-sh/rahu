package lsp

type ChangeAnnotation struct {
	Label             string `json:"label"`
	NeedsConfirmation bool   `json:"needsConfirmation,omitempty"`
	Description       string `json:"description,omitempty"`
}

type ChangeAnnotationIdentifier string

type WorkspaceEdit struct {
	Changes           map[DocumentURI][]TextEdit                      `json:"changes,omitempty"`
	DocumentChanges   []TextDocumentEdit                              `json:"documentChanges,omitempty"`
	ChangeAnnotations map[ChangeAnnotationIdentifier]ChangeAnnotation `json:"changeAnnotations,omitempty"`
}

type ResourceOperationKind string

const (
	CREATE ResourceOperationKind = "create"
	RENAME ResourceOperationKind = "rename"
	DELETE ResourceOperationKind = "delete"
)

type FailureHandlingKind string

type ChangeAnnotationSupport struct {
	GroupsOnLabel *bool `json:"groupsOnLabel,omitempty"`
}

const (
	ABORT                 FailureHandlingKind = "abort"
	TRANSACTIONAL         FailureHandlingKind = "transactional"
	TEXTONLYTRANSACTIONAL FailureHandlingKind = "textOnlyTransactional"
	UNDO                  FailureHandlingKind = "undo"
)

type WorkspaceEditClientCapabilities struct {
	DocumentChanges         *bool                    `json:"documentChanges,omitempty"`
	ResourceOperations      *[]ResourceOperationKind `json:"resourceOperations,omitempty"`
	FailureHandling         *FailureHandlingKind     `json:"failureHandlingKind,omitempty"`
	NormalizesLineEndings   *bool                    `json:"normalizesLineEndings,omitempty"`
	ChangeAnnotationSupport *ChangeAnnotationSupport `json:"changeAnnotationSupport,omitempty"`
}

type DidChangeConfigurationParams struct {
	Settings any `json:"settings"`
}

type DidChangeConfigurationClientCapabilities struct {
	DynamicRegistration *bool `json:"dynamicRegistration,omitempty"`
}

type DidChangeWatchedFilesClientCapabilities struct {
	DynamicRegistration    *bool `json:"dynamicRegistration,omitempty"`
	RelativePatternSupport *bool `json:"relativePatternSupport,omitempty"`
}

type WatchKind int

const (
	Create WatchKind = 1
	Change WatchKind = 2
	Delete WatchKind = 4
)

type WorkspaceFolder struct {
	URI  DocumentURI `json:"uri"`
	Name string      `json:"name"`
}

type Pattern string

type RelativePattern struct {
	BaseURI any     `json:"baseUri"`
	Pattern Pattern `json:"pattern"`
}

type GlobPattern any

type FileSystemWatcher struct {
	Kind        *WatchKind  `json:"kind,omitempty"`
	GlobPattern GlobPattern `json:"globPattern"`
}

type DidChangeWatchedFilesRegistrationOptions struct {
	Watchers []FileSystemWatcher `json:"watchers"`
}

type WorkspaceClientCapabilities struct {
	ApplyEdit              *bool                                     `json:"applyEdit,omitempty"`
	WorkspaceEdit          *WorkspaceEditClientCapabilities          `json:"workspaceEdit,omitempty"`
	DidChangeConfiguration *DidChangeConfigurationClientCapabilities `json:"didChangeConfiguration,omitempty"`
	DidChangeWatchedFiles  *DidChangeWatchedFilesClientCapabilities  `json:"didChangeWatchedFiles,omitempty"`
}
