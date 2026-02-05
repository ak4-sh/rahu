package lsp

type TextDocumentIdentifier struct {
	// URI is the unique identifier of the text document.
	URI DocumentURI `json:"uri"`
}

type VersionedDocumentIdentifier struct {
	// URI is the unique identifier of the text document.
	URI DocumentURI `json:"uri"`

	// Version is the version number of the document.
	Version int `json:"version"`
}

type TextDocumentItem struct {
	// URI is the unique identifier of the text document.
	URI DocumentURI `json:"uri"`

	// LanguageID specifies the language of the document (e.g., "python").
	LanguageID string `json:"languageId"`

	// Version is the version number of the document.
	Version int `json:"version"`

	// Text contains the full content of the document.
	Text string `json:"text"`
}

type TextDocumentPositionParams struct {
	// TextDocument identifies the document in which the position is specified.
	TextDocument TextDocumentIdentifier `json:"textDocument"`

	// Position specifies the location within the document.
	Position Position `json:"position"`
}

type DocumentFilter struct {
	// Language specifies the language of the document (e.g., "go").
	Language string `json:"language"`

	// Scheme specifies the URI scheme (e.g., "file").
	Scheme string `json:"scheme"`

	// Pattern specifies the glob pattern to match document paths.
	Pattern string `json:"pattern"`
}

type DocumentSelector []DocumentFilter // DocumentSelector is a list of filters used to match documents.

type TextEdit struct {
	// Range specifies the range of text to be replaced.
	Range Range `json:"range"`

	// NewText is the text to replace the specified range with.
	NewText string `json:"newText"`
}

type OptionalVersionedTextDocumentIdentifier struct {
	URI     DocumentURI `json:"uri"`
	Version *int        `json:"version,omitempty"`
}

type AnnotatedTextEdit struct {
	Range        Range                      `json:"range"`
	NewText      string                     `json:"newText"`
	AnnotationID ChangeAnnotationIdentifier `json:"annotationId"`
}

type TextDocumentEdit struct {
	TextDocument OptionalVersionedTextDocumentIdentifier `json:"textDocument"`
	Edits        any                                     `json:"edits"` // []TextEdit | []AnnotatedTextEdit
}

type TextDocumentSyncKind int

const (
	TDSKNone        TextDocumentSyncKind = 0
	TDSKFull        TextDocumentSyncKind = 1
	TDSKIncremental TextDocumentSyncKind = 2
)

type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type TextDocumentContentChangeEvent struct {
	Range       *Range `json:"range,omitempty"`
	RangeLength *int   `json:"rangeLength,omitempty"`
	Text        string `json:"text"`
}

func (e TextDocumentContentChangeEvent) IsFullChange() bool {
	return e.Range == nil
}

func (e TextDocumentContentChangeEvent) IsIncremental() bool {
	return e.Range != nil
}

type DidChangeTextDocumentParams struct {
	TextDocument   VersionedDocumentIdentifier      `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}
