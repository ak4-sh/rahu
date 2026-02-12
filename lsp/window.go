package lsp

type ProgressToken any

type ProgressParams struct {
	Token ProgressToken `json:"token"`
	Value any           `json:"value"`
}

type HoverParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type MarkedString any

type Hover struct {
	Contents any    `json:"contents"`
	Range    *Range `json:"range,omitempty"`
}

type WorkDoneProgressParams struct {
	WorkDoneToken ProgressToken `json:"workDoneToken,omitempty"`
}

type PartialResultParams struct {
	PartialResultToken *ProgressToken `json:"partialResultToken,omitempty"`
}
