package lsp

type MessageType int

const (
	MessageTypeError   MessageType = 1
	MessageTypeWarning MessageType = 2
	MessageTypeInfo    MessageType = 3
	MessageTypeLog     MessageType = 4
)

type ShowMessageParams struct {
	Type    MessageType `json:"type"`
	Message string      `json:"message"`
}

type MessageActionItem struct {
	Title string `json:"title"`
}

type ShowMessageRequestParams struct {
	Type    MessageType         `json:"type"`
	Message string              `json:"message"`
	Actions []MessageActionItem `json:"actions,omitempty"`
}

type WorkDoneProgressCreateParams struct {
	Token ProgressToken `json:"token"`
}

type WorkDoneProgressBegin struct {
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Message     string `json:"message,omitempty"`
	Cancellable bool   `json:"cancellable,omitempty"`
	Percentage  *int   `json:"percentage,omitempty"`
}

type WorkDoneProgressReport struct {
	Kind        string  `json:"kind"` // Always "report"
	Message     string  `json:"message,omitempty"`
	Percentage  *uint32 `json:"percentage,omitempty"`
	Cancellable bool    `json:"cancellable,omitempty"`
}

type WorkDoneProgressEnd struct {
	Kind    string `json:"kind"`
	Message string `json:"message,omitempty"`
}

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
