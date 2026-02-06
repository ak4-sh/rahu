package lsp

type Position struct {
	// Line represents the zero-based line number in a document.
	Line int `json:"line"`

	// Character represents the zero-based character offset within the line.
	Character int `json:"character"`
}

type DocumentURI string // DocumentURI is a string representing the URI of a document.

type Range struct {
	// Start represents the starting position of the range.
	Start Position `json:"start"`

	// End represents the ending position of the range.
	End Position `json:"end"`
}

type Location struct {
	URI   DocumentURI `json:"uri"`
	Range Range       `json:"range"`
}

type LocationLink struct {
	OriginSelectionRange *Range      `json:"originSelectionRange,omitempty"`
	TargetURI            DocumentURI `json:"targetUri"`
	TargetRange          Range       `json:"targetRange"`
	TargetSelectionRange Range       `json:"targetSelectionRange"`
}

type MarkupKind string

const (
	MarkupPlaintext MarkupKind = "plaintext"
	MarkupMarkdown  MarkupKind = "markdown"
)

type MarkupContent struct {
	Kind  MarkupKind `json:"kind"`
	Value string     `json:"value"`
}
