package lsp

import (
	"encoding/json"
	"testing"
)

func TestTextDocumentItemMarshalUnmarshal(t *testing.T) {
	original := TextDocumentItem{
		URI:        "file:///test.py",
		LanguageID: "python",
		Version:    1,
		Text:       "x = 1\ny = 2",
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal back
	var decoded TextDocumentItem
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify all fields
	if decoded.URI != original.URI {
		t.Errorf("URI mismatch: expected %q, got %q", original.URI, decoded.URI)
	}
	if decoded.LanguageID != original.LanguageID {
		t.Errorf("LanguageID mismatch: expected %q, got %q", original.LanguageID, decoded.LanguageID)
	}
	if decoded.Version != original.Version {
		t.Errorf("Version mismatch: expected %d, got %d", original.Version, decoded.Version)
	}
	if decoded.Text != original.Text {
		t.Errorf("Text mismatch: expected %q, got %q", original.Text, decoded.Text)
	}
}

func TestPositionMarshalUnmarshal(t *testing.T) {
	original := Position{Line: 10, Character: 5}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Position
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Line != original.Line {
		t.Errorf("Line mismatch: expected %d, got %d", original.Line, decoded.Line)
	}
	if decoded.Character != original.Character {
		t.Errorf("Character mismatch: expected %d, got %d", original.Character, decoded.Character)
	}
}

func TestRangeMarshalUnmarshal(t *testing.T) {
	original := Range{
		Start: Position{Line: 0, Character: 5},
		End:   Position{Line: 2, Character: 10},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Range
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Start.Line != original.Start.Line {
		t.Errorf("Start.Line mismatch: expected %d, got %d", original.Start.Line, decoded.Start.Line)
	}
	if decoded.Start.Character != original.Start.Character {
		t.Errorf("Start.Character mismatch: expected %d, got %d", original.Start.Character, decoded.Start.Character)
	}
	if decoded.End.Line != original.End.Line {
		t.Errorf("End.Line mismatch: expected %d, got %d", original.End.Line, decoded.End.Line)
	}
	if decoded.End.Character != original.End.Character {
		t.Errorf("End.Character mismatch: expected %d, got %d", original.End.Character, decoded.End.Character)
	}
}

func TestTextDocumentContentChangeEventFullChange(t *testing.T) {
	// Full change has no Range
	event := TextDocumentContentChangeEvent{
		Text: "completely new content",
	}

	if !event.IsFullChange() {
		t.Error("expected IsFullChange() to be true")
	}

	if event.IsIncremental() {
		t.Error("expected IsIncremental() to be false")
	}

	// Marshal and unmarshal
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded TextDocumentContentChangeEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Text != event.Text {
		t.Errorf("Text mismatch: expected %q, got %q", event.Text, decoded.Text)
	}

	if decoded.Range != nil {
		t.Error("expected Range to be nil for full change")
	}
}

func TestTextDocumentContentChangeEventIncrementalChange(t *testing.T) {
	// Incremental change has a Range
	event := TextDocumentContentChangeEvent{
		Range: &Range{
			Start: Position{Line: 5, Character: 10},
			End:   Position{Line: 5, Character: 20},
		},
		RangeLength: intPtr(10),
		Text:        "replacement",
	}

	if event.IsFullChange() {
		t.Error("expected IsFullChange() to be false")
	}

	if !event.IsIncremental() {
		t.Error("expected IsIncremental() to be true")
	}

	// Marshal and unmarshal
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded TextDocumentContentChangeEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Text != event.Text {
		t.Errorf("Text mismatch: expected %q, got %q", event.Text, decoded.Text)
	}

	if decoded.Range == nil {
		t.Fatal("expected Range to be non-nil for incremental change")
	}

	if decoded.Range.Start.Line != event.Range.Start.Line {
		t.Errorf("Range.Start.Line mismatch: expected %d, got %d",
			event.Range.Start.Line, decoded.Range.Start.Line)
	}

	if decoded.RangeLength == nil {
		t.Fatal("expected RangeLength to be non-nil")
	}

	if *decoded.RangeLength != *event.RangeLength {
		t.Errorf("RangeLength mismatch: expected %d, got %d",
			*event.RangeLength, *decoded.RangeLength)
	}
}

func TestDiagnosticMarshalUnmarshal(t *testing.T) {
	original := Diagnostic{
		Range: Range{
			Start: Position{Line: 10, Character: 5},
			End:   Position{Line: 10, Character: 10},
		},
		Severity: SeverityError,
		Code:     "E001",
		Source:   "rahu",
		Message:  "undefined variable 'foo'",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Diagnostic
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Severity != original.Severity {
		t.Errorf("Severity mismatch: expected %d, got %d", original.Severity, decoded.Severity)
	}
	if decoded.Code != original.Code {
		t.Errorf("Code mismatch: expected %q, got %q", original.Code, decoded.Code)
	}
	if decoded.Source != original.Source {
		t.Errorf("Source mismatch: expected %q, got %q", original.Source, decoded.Source)
	}
	if decoded.Message != original.Message {
		t.Errorf("Message mismatch: expected %q, got %q", original.Message, decoded.Message)
	}
}

func TestSeverityConstants(t *testing.T) {
	// Test that severity constants have expected values
	if SeverityError != 0 {
		t.Errorf("SeverityError should be 0, got %d", SeverityError)
	}
	if SeverityWarning != 1 {
		t.Errorf("SeverityWarning should be 1, got %d", SeverityWarning)
	}
	if SeverityInformation != 2 {
		t.Errorf("SeverityInformation should be 2, got %d", SeverityInformation)
	}
	if SeverityHint != 3 {
		t.Errorf("SeverityHint should be 3, got %d", SeverityHint)
	}
}

func TestInitializeParamsMarshalUnmarshal(t *testing.T) {
	processID := 12345
	rootURI := DocumentURI("file:///project")

	original := InitializeParams{
		ProcessID: &processID,
		RootURI:   &rootURI,
		Capabilities: ClientCapabilities{
			Workspace: &WorkspaceClientCapabilities{
				ApplyEdit: boolPtr(true),
			},
			TextDocument: &TextDocumentClientCapabilities{},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded InitializeParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.ProcessID == nil || *decoded.ProcessID != *original.ProcessID {
		t.Errorf("ProcessID mismatch: expected %d, got %v", *original.ProcessID, decoded.ProcessID)
	}
	if decoded.RootURI == nil || *decoded.RootURI != *original.RootURI {
		t.Errorf("RootURI mismatch: expected %q, got %v", *original.RootURI, decoded.RootURI)
	}

	if decoded.Capabilities.Workspace == nil {
		t.Fatal("expected Workspace capabilities")
	}
	if decoded.Capabilities.Workspace.ApplyEdit == nil || !*decoded.Capabilities.Workspace.ApplyEdit {
		t.Error("expected ApplyEdit to be true")
	}

	if decoded.Capabilities.TextDocument == nil {
		t.Fatal("expected TextDocument capabilities")
	}
}

func TestInitializeResultMarshalUnmarshal(t *testing.T) {
	original := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync:   TDSKFull,
			HoverProvider:      true,
			DiagnosticProvider: true,
		},
		ServerInfo: &ClientInfo{
			Name:    "rahu",
			Version: strPtr("1.0.0"),
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded InitializeResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Capabilities.TextDocumentSync != original.Capabilities.TextDocumentSync {
		t.Errorf("TextDocumentSync mismatch: expected %d, got %d",
			original.Capabilities.TextDocumentSync, decoded.Capabilities.TextDocumentSync)
	}
	if decoded.Capabilities.HoverProvider != original.Capabilities.HoverProvider {
		t.Errorf("HoverProvider mismatch: expected %v, got %v",
			original.Capabilities.HoverProvider, decoded.Capabilities.HoverProvider)
	}
	if decoded.ServerInfo == nil || decoded.ServerInfo.Name != "rahu" {
		t.Error("expected ServerInfo.Name to be 'rahu'")
	}
}

func TestTextDocumentIdentifierMarshalUnmarshal(t *testing.T) {
	original := TextDocumentIdentifier{URI: "file:///test.py"}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded TextDocumentIdentifier
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.URI != original.URI {
		t.Errorf("URI mismatch: expected %q, got %q", original.URI, decoded.URI)
	}
}

func TestVersionedDocumentIdentifierMarshalUnmarshal(t *testing.T) {
	original := VersionedDocumentIdentifier{
		URI:     "file:///test.py",
		Version: 5,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded VersionedDocumentIdentifier
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.URI != original.URI {
		t.Errorf("URI mismatch: expected %q, got %q", original.URI, decoded.URI)
	}
	if decoded.Version != original.Version {
		t.Errorf("Version mismatch: expected %d, got %d", original.Version, decoded.Version)
	}
}

func TestTextDocumentPositionParamsMarshalUnmarshal(t *testing.T) {
	original := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///test.py"},
		Position:     Position{Line: 10, Character: 5},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded TextDocumentPositionParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.TextDocument.URI != original.TextDocument.URI {
		t.Errorf("TextDocument.URI mismatch: expected %q, got %q",
			original.TextDocument.URI, decoded.TextDocument.URI)
	}
	if decoded.Position.Line != original.Position.Line {
		t.Errorf("Position.Line mismatch: expected %d, got %d",
			original.Position.Line, decoded.Position.Line)
	}
}

func TestTextEditMarshalUnmarshal(t *testing.T) {
	original := TextEdit{
		Range: Range{
			Start: Position{Line: 0, Character: 0},
			End:   Position{Line: 0, Character: 5},
		},
		NewText: "hello",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded TextEdit
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.NewText != original.NewText {
		t.Errorf("NewText mismatch: expected %q, got %q", original.NewText, decoded.NewText)
	}
	if decoded.Range.Start.Character != original.Range.Start.Character {
		t.Errorf("Range.Start.Character mismatch: expected %d, got %d",
			original.Range.Start.Character, decoded.Range.Start.Character)
	}
}

func TestDidOpenTextDocumentParamsMarshalUnmarshal(t *testing.T) {
	original := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        "file:///test.py",
			LanguageID: "python",
			Version:    1,
			Text:       "x = 1",
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded DidOpenTextDocumentParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.TextDocument.URI != original.TextDocument.URI {
		t.Errorf("TextDocument.URI mismatch: expected %q, got %q",
			original.TextDocument.URI, decoded.TextDocument.URI)
	}
	if decoded.TextDocument.Text != original.TextDocument.Text {
		t.Errorf("TextDocument.Text mismatch: expected %q, got %q",
			original.TextDocument.Text, decoded.TextDocument.Text)
	}
}

func TestDidChangeTextDocumentParamsMarshalUnmarshal(t *testing.T) {
	original := DidChangeTextDocumentParams{
		TextDocument: VersionedDocumentIdentifier{
			URI:     "file:///test.py",
			Version: 2,
		},
		ContentChanges: []TextDocumentContentChangeEvent{
			{
				Range: &Range{
					Start: Position{Line: 0, Character: 4},
					End:   Position{Line: 0, Character: 5},
				},
				Text: "2",
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded DidChangeTextDocumentParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.TextDocument.Version != original.TextDocument.Version {
		t.Errorf("TextDocument.Version mismatch: expected %d, got %d",
			original.TextDocument.Version, decoded.TextDocument.Version)
	}

	if len(decoded.ContentChanges) != 1 {
		t.Fatalf("expected 1 content change, got %d", len(decoded.ContentChanges))
	}

	if decoded.ContentChanges[0].Text != "2" {
		t.Errorf("ContentChanges[0].Text mismatch: expected %q, got %q",
			"2", decoded.ContentChanges[0].Text)
	}
}

func TestDidCloseTextDocumentParamsMarshalUnmarshal(t *testing.T) {
	original := DidCloseTextDocumentParams{
		TextDocument: TextDocumentIdentifier{
			URI: "file:///test.py",
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded DidCloseTextDocumentParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.TextDocument.URI != original.TextDocument.URI {
		t.Errorf("TextDocument.URI mismatch: expected %q, got %q",
			original.TextDocument.URI, decoded.TextDocument.URI)
	}
}

func TestLocationMarshalUnmarshal(t *testing.T) {
	original := Location{
		URI: "file:///test.py",
		Range: Range{
			Start: Position{Line: 10, Character: 5},
			End:   Position{Line: 10, Character: 10},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Location
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.URI != original.URI {
		t.Errorf("URI mismatch: expected %q, got %q", original.URI, decoded.URI)
	}
	if decoded.Range.Start.Line != original.Range.Start.Line {
		t.Errorf("Range.Start.Line mismatch: expected %d, got %d",
			original.Range.Start.Line, decoded.Range.Start.Line)
	}
}

func TestMarkupContentMarshalUnmarshal(t *testing.T) {
	original := MarkupContent{
		Kind:  MarkupMarkdown,
		Value: "```python\nfunction foo()\n```",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded MarkupContent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Kind != original.Kind {
		t.Errorf("Kind mismatch: expected %q, got %q", original.Kind, decoded.Kind)
	}
	if decoded.Value != original.Value {
		t.Errorf("Value mismatch: expected %q, got %q", original.Value, decoded.Value)
	}
}

func TestMarkupKindConstants(t *testing.T) {
	if MarkupPlaintext != "plaintext" {
		t.Errorf("MarkupPlaintext should be 'plaintext', got %q", MarkupPlaintext)
	}
	if MarkupMarkdown != "markdown" {
		t.Errorf("MarkupMarkdown should be 'markdown', got %q", MarkupMarkdown)
	}
}

func TestTextDocumentSyncKindConstants(t *testing.T) {
	if TDSKNone != 0 {
		t.Errorf("TDSKNone should be 0, got %d", TDSKNone)
	}
	if TDSKFull != 1 {
		t.Errorf("TDSKFull should be 1, got %d", TDSKFull)
	}
	if TDSKIncremental != 2 {
		t.Errorf("TDSKIncremental should be 2, got %d", TDSKIncremental)
	}
}

func TestPublishDiagnosticsParamsMarshalUnmarshal(t *testing.T) {
	version := 5
	original := PublishDiagnosticsParams{
		URI:     "file:///test.py",
		Version: &version,
		Diagnostics: []Diagnostic{
			{
				Range: Range{
					Start: Position{Line: 0, Character: 0},
					End:   Position{Line: 0, Character: 5},
				},
				Severity: SeverityError,
				Message:  "test error",
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded PublishDiagnosticsParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.URI != original.URI {
		t.Errorf("URI mismatch: expected %q, got %q", original.URI, decoded.URI)
	}
	if decoded.Version == nil || *decoded.Version != version {
		t.Errorf("Version mismatch: expected %d, got %v", version, decoded.Version)
	}
	if len(decoded.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(decoded.Diagnostics))
	}
	if decoded.Diagnostics[0].Message != "test error" {
		t.Errorf("Diagnostic.Message mismatch: expected %q, got %q",
			"test error", decoded.Diagnostics[0].Message)
	}
}

func intPtr(i int) *int {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}

func strPtr(s string) *string {
	return &s
}
