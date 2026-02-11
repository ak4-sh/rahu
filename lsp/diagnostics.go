package lsp

type Severity int

const (
	_ Severity = iota
	SeverityError
	SeverityWarning
	SeverityInformation
	SeverityHint
)

type Diagnostic struct {
	Range    Range    `json:"range"`
	Severity Severity `json:"severity,omitempty"`
	Code     any      `json:"code,omitempty"`
	Source   string   `json:"source,omitempty"`
	Message  string   `json:"message"`
}

type DiagnosticError struct {
	Msg string
}

func (d *DiagnosticError) error() string {
	return d.Msg
}

type PublishDiagnosticsParams struct {
	URI         DocumentURI  `json:"uri"`
	Version     *int         `json:"version,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type DocumentDiagnosticParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type DocumentDiagnosticReport struct {
	Kind  string       `json:"kind"`
	Items []Diagnostic `json:"items"`
}
