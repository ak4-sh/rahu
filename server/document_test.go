package server

import (
	"testing"

	"rahu/lsp"
)

func TestDocumentOpen(t *testing.T) {
	s := &Server{
		docs: make(map[lsp.DocumentURI]*Document),
	}

	item := lsp.TextDocumentItem{
		URI:        "file:///test.py",
		LanguageID: "python",
		Version:    1,
		Text:       "x = 1\ny = 2",
	}

	s.Open(item)

	doc := s.Get(item.URI)
	if doc == nil {
		t.Fatal("document not found after open")
	}

	if doc.URI != item.URI {
		t.Errorf("expected URI %q, got %q", item.URI, doc.URI)
	}

	if doc.Version != item.Version {
		t.Errorf("expected version %d, got %d", item.Version, doc.Version)
	}

	if doc.Text != item.Text {
		t.Errorf("expected text %q, got %q", item.Text, doc.Text)
	}

	if len(doc.Lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(doc.Lines))
	}
}

func TestDocumentGet(t *testing.T) {
	s := &Server{
		docs: make(map[lsp.DocumentURI]*Document),
	}

	// Test getting non-existent document
	doc := s.Get("file:///nonexistent.py")
	if doc != nil {
		t.Error("expected nil for non-existent document")
	}

	// Test getting existing document
	s.Open(lsp.TextDocumentItem{
		URI:     "file:///test.py",
		Version: 1,
		Text:    "x = 1",
	})

	doc = s.Get("file:///test.py")
	if doc == nil {
		t.Fatal("expected document to exist")
	}

	// Verify we get a copy, not the original
	doc.Version = 999
	doc2 := s.Get("file:///test.py")
	if doc2.Version != 1 {
		t.Error("Get() should return a copy, not modify the original")
	}
}

func TestDocumentClose(t *testing.T) {
	s := &Server{
		docs: make(map[lsp.DocumentURI]*Document),
	}

	uri := lsp.DocumentURI("file:///test.py")
	s.Open(lsp.TextDocumentItem{
		URI:     uri,
		Version: 1,
		Text:    "x = 1",
	})

	// Verify document exists
	if s.Get(uri) == nil {
		t.Fatal("document should exist before close")
	}

	s.Close(uri)

	// Verify document is removed
	if s.Get(uri) != nil {
		t.Error("document should be removed after close")
	}

	// Closing non-existent document should not panic
	s.Close("file:///nonexistent.py")
}

func TestDocumentUpdate(t *testing.T) {
	tests := []struct {
		name          string
		initialText   string
		initialVer    int
		newText       string
		newVersion    int
		expectUpdate  bool
		expectedText  string
		expectedLines int
	}{
		{
			name:          "normal update with higher version",
			initialText:   "x = 1",
			initialVer:    1,
			newText:       "x = 2",
			newVersion:    2,
			expectUpdate:  true,
			expectedText:  "x = 2",
			expectedLines: 1,
		},
		{
			name:          "update with same version is ignored",
			initialText:   "x = 1",
			initialVer:    1,
			newText:       "x = 2",
			newVersion:    1,
			expectUpdate:  false,
			expectedText:  "x = 1",
			expectedLines: 1,
		},
		{
			name:          "update with lower version is ignored",
			initialText:   "x = 1",
			initialVer:    2,
			newText:       "x = 2",
			newVersion:    1,
			expectUpdate:  false,
			expectedText:  "x = 1",
			expectedLines: 1,
		},
		{
			name:          "update with multi-line text",
			initialText:   "x = 1",
			initialVer:    1,
			newText:       "x = 1\ny = 2\nz = 3",
			newVersion:    2,
			expectUpdate:  true,
			expectedText:  "x = 1\ny = 2\nz = 3",
			expectedLines: 3,
		},
		{
			name:          "update empty document",
			initialText:   "",
			initialVer:    1,
			newText:       "x = 1",
			newVersion:    2,
			expectUpdate:  true,
			expectedText:  "x = 1",
			expectedLines: 1,
		},
		{
			name:          "update non-existent document is ignored",
			initialText:   "",
			initialVer:    0,
			newText:       "x = 1",
			newVersion:    1,
			expectUpdate:  false,
			expectedText:  "",
			expectedLines: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				docs: make(map[lsp.DocumentURI]*Document),
			}

			uri := lsp.DocumentURI("file:///test.py")

			if tt.initialVer > 0 {
				s.Open(lsp.TextDocumentItem{
					URI:     uri,
					Version: tt.initialVer,
					Text:    tt.initialText,
				})
			}

			s.Update(uri, tt.newText, tt.newVersion)

			doc := s.Get(uri)
			if tt.initialVer == 0 {
				// Non-existent document case
				if doc != nil {
					t.Error("non-existent document should remain nil")
				}
				return
			}

			if doc.Text != tt.expectedText {
				t.Errorf("expected text %q, got %q", tt.expectedText, doc.Text)
			}

			expectedVersion := tt.initialVer
			if tt.expectUpdate {
				expectedVersion = tt.newVersion
			}
			if doc.Version != expectedVersion {
				t.Errorf("expected version %d, got %d", expectedVersion, doc.Version)
			}

			if len(doc.Lines) != tt.expectedLines {
				t.Errorf("expected %d lines, got %d", tt.expectedLines, len(doc.Lines))
			}
		})
	}
}

func TestApplyFullChange(t *testing.T) {
	tests := []struct {
		name         string
		initialText  string
		changes      []lsp.TextDocumentContentChangeEvent
		version      int
		expectedText string
	}{
		{
			name:        "full change replaces entire document",
			initialText: "x = 1\ny = 2",
			changes: []lsp.TextDocumentContentChangeEvent{
				{Text: "z = 3"},
			},
			version:      2,
			expectedText: "z = 3",
		},
		{
			name:         "empty changes does nothing",
			initialText:  "x = 1",
			changes:      []lsp.TextDocumentContentChangeEvent{},
			version:      2,
			expectedText: "x = 1",
		},
		{
			name:        "incremental change delegates to ApplyIncremental",
			initialText: "x = 1\ny = 2",
			changes: []lsp.TextDocumentContentChangeEvent{
				{
					Range: &lsp.Range{
						Start: lsp.Position{Line: 0, Character: 4},
						End:   lsp.Position{Line: 0, Character: 5},
					},
					Text: "999",
				},
			},
			version:      2,
			expectedText: "x = 999\ny = 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				docs: make(map[lsp.DocumentURI]*Document),
			}

			uri := lsp.DocumentURI("file:///test.py")
			s.Open(lsp.TextDocumentItem{
				URI:     uri,
				Version: 1,
				Text:    tt.initialText,
			})

			s.ApplyFullChange(uri, tt.changes, tt.version)

			doc := s.Get(uri)
			if doc.Text != tt.expectedText {
				t.Errorf("expected text %q, got %q", tt.expectedText, doc.Text)
			}
		})
	}
}

func TestApplyIncrementalChange(t *testing.T) {
	tests := []struct {
		name         string
		initialText  string
		changes      []lsp.TextDocumentContentChangeEvent
		version      int
		expectedText string
	}{
		{
			name:        "single character replacement",
			initialText: "hello world",
			changes: []lsp.TextDocumentContentChangeEvent{
				{
					Range: &lsp.Range{
						Start: lsp.Position{Line: 0, Character: 6},
						End:   lsp.Position{Line: 0, Character: 11},
					},
					Text: "Go",
				},
			},
			version:      2,
			expectedText: "hello Go",
		},
		{
			name:        "insert at beginning",
			initialText: "world",
			changes: []lsp.TextDocumentContentChangeEvent{
				{
					Range: &lsp.Range{
						Start: lsp.Position{Line: 0, Character: 0},
						End:   lsp.Position{Line: 0, Character: 0},
					},
					Text: "hello ",
				},
			},
			version:      2,
			expectedText: "hello world",
		},
		{
			name:        "insert at end",
			initialText: "hello",
			changes: []lsp.TextDocumentContentChangeEvent{
				{
					Range: &lsp.Range{
						Start: lsp.Position{Line: 0, Character: 5},
						End:   lsp.Position{Line: 0, Character: 5},
					},
					Text: " world",
				},
			},
			version:      2,
			expectedText: "hello world",
		},
		{
			name:        "delete text",
			initialText: "hello world",
			changes: []lsp.TextDocumentContentChangeEvent{
				{
					Range: &lsp.Range{
						Start: lsp.Position{Line: 0, Character: 5},
						End:   lsp.Position{Line: 0, Character: 11},
					},
					Text: "",
				},
			},
			version:      2,
			expectedText: "hello",
		},
		{
			name:        "multi-line replacement",
			initialText: "line1\nline2\nline3",
			changes: []lsp.TextDocumentContentChangeEvent{
				{
					Range: &lsp.Range{
						Start: lsp.Position{Line: 0, Character: 5},
						End:   lsp.Position{Line: 2, Character: 5},
					},
					Text: "X\nnewLine",
				},
			},
			version:      2,
			expectedText: "line1X\nnewLine",
		},
		{
			name:        "multiple sequential changes",
			initialText: "abc",
			changes: []lsp.TextDocumentContentChangeEvent{
				{
					Range: &lsp.Range{
						Start: lsp.Position{Line: 0, Character: 0},
						End:   lsp.Position{Line: 0, Character: 1},
					},
					Text: "X",
				},
				{
					Range: &lsp.Range{
						Start: lsp.Position{Line: 0, Character: 2},
						End:   lsp.Position{Line: 0, Character: 3},
					},
					Text: "Y",
				},
			},
			version:      2,
			expectedText: "XbY",
		},
		{
			name:        "full document change in incremental",
			initialText: "old text",
			changes: []lsp.TextDocumentContentChangeEvent{
				{Text: "completely new"},
			},
			version:      2,
			expectedText: "completely new",
		},
		{
			name:        "change with nil range falls back to full",
			initialText: "original",
			changes: []lsp.TextDocumentContentChangeEvent{
				{Range: nil, Text: "replacement"},
			},
			version:      2,
			expectedText: "replacement",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				docs: make(map[lsp.DocumentURI]*Document),
			}

			uri := lsp.DocumentURI("file:///test.py")
			s.Open(lsp.TextDocumentItem{
				URI:     uri,
				Version: 1,
				Text:    tt.initialText,
			})

			s.ApplyIncremental(uri, tt.changes, tt.version)

			doc := s.Get(uri)
			if doc.Text != tt.expectedText {
				t.Errorf("expected text %q, got %q", tt.expectedText, doc.Text)
			}
		})
	}
}

func TestApplyRangeEdit(t *testing.T) {
	tests := []struct {
		name         string
		oldText      string
		rng          lsp.Range
		newText      string
		expectedText string
	}{
		{
			name:    "replace within single line",
			oldText: "hello world",
			rng: lsp.Range{
				Start: lsp.Position{Line: 0, Character: 6},
				End:   lsp.Position{Line: 0, Character: 11},
			},
			newText:      "Go",
			expectedText: "hello Go",
		},
		{
			name:    "replace across multiple lines",
			oldText: "line1\nline2\nline3",
			rng: lsp.Range{
				Start: lsp.Position{Line: 0, Character: 0},
				End:   lsp.Position{Line: 2, Character: 5},
			},
			newText:      "replacement",
			expectedText: "replacement",
		},
		{
			name:    "insert at position",
			oldText: "abcde",
			rng: lsp.Range{
				Start: lsp.Position{Line: 0, Character: 2},
				End:   lsp.Position{Line: 0, Character: 2},
			},
			newText:      "XYZ",
			expectedText: "abXYZcde",
		},
		{
			name:    "delete range",
			oldText: "hello world",
			rng: lsp.Range{
				Start: lsp.Position{Line: 0, Character: 5},
				End:   lsp.Position{Line: 0, Character: 11},
			},
			newText:      "",
			expectedText: "hello",
		},
		{
			name:    "replace with multi-line text",
			oldText: "line1\nline2",
			rng: lsp.Range{
				Start: lsp.Position{Line: 0, Character: 5},
				End:   lsp.Position{Line: 1, Character: 5},
			},
			newText:      "X\nmiddle\nY",
			expectedText: "line1X\nmiddle\nY",
		},
		{
			name:    "start line beyond text returns original",
			oldText: "short",
			rng: lsp.Range{
				Start: lsp.Position{Line: 10, Character: 0},
				End:   lsp.Position{Line: 10, Character: 5},
			},
			newText:      "new",
			expectedText: "short",
		},
		{
			name:    "start character beyond line length is clamped",
			oldText: "short",
			rng: lsp.Range{
				Start: lsp.Position{Line: 0, Character: 100},
				End:   lsp.Position{Line: 0, Character: 100},
			},
			newText:      "X",
			expectedText: "shortX",
		},
		{
			name:    "end character beyond line length is clamped",
			oldText: "short",
			rng: lsp.Range{
				Start: lsp.Position{Line: 0, Character: 2},
				End:   lsp.Position{Line: 0, Character: 100},
			},
			newText:      "X",
			expectedText: "shX",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyRangeEdit(tt.oldText, tt.rng, tt.newText)
			if result != tt.expectedText {
				t.Errorf("expected %q, got %q", tt.expectedText, result)
			}
		})
	}
}

func TestCloseReopen(t *testing.T) {
	s := &Server{
		docs: make(map[lsp.DocumentURI]*Document),
	}

	uri := lsp.DocumentURI("file:///test.py")

	// Open document
	s.Open(lsp.TextDocumentItem{
		URI:     uri,
		Version: 1,
		Text:    "x = 1",
	})

	// Close document
	s.Close(uri)

	// Reopen with different content
	s.Open(lsp.TextDocumentItem{
		URI:     uri,
		Version: 1,
		Text:    "y = 2\nz = 3",
	})

	doc := s.Get(uri)
	if doc == nil {
		t.Fatal("document should exist after reopen")
	}

	if doc.Text != "y = 2\nz = 3" {
		t.Errorf("expected reopened text, got %q", doc.Text)
	}

	if len(doc.Lines) != 2 {
		t.Errorf("expected 2 lines after reopen, got %d", len(doc.Lines))
	}
}

func TestMultipleSequentialEdits(t *testing.T) {
	s := &Server{
		docs: make(map[lsp.DocumentURI]*Document),
	}

	uri := lsp.DocumentURI("file:///test.py")

	// Open with initial content
	s.Open(lsp.TextDocumentItem{
		URI:     uri,
		Version: 1,
		Text:    "def foo():\n    pass",
	})

	// Edit 1: Add a new line
	s.ApplyFullChange(uri, []lsp.TextDocumentContentChangeEvent{
		{
			Range: &lsp.Range{
				Start: lsp.Position{Line: 1, Character: 8},
				End:   lsp.Position{Line: 1, Character: 8},
			},
			Text: "\n    x = 1",
		},
	}, 2)

	doc := s.Get(uri)
	expected1 := "def foo():\n    pass\n    x = 1"
	if doc.Text != expected1 {
		t.Errorf("after edit 1: expected %q, got %q", expected1, doc.Text)
	}

	// Edit 2: Modify the function name
	s.ApplyFullChange(uri, []lsp.TextDocumentContentChangeEvent{
		{
			Range: &lsp.Range{
				Start: lsp.Position{Line: 0, Character: 4},
				End:   lsp.Position{Line: 0, Character: 7},
			},
			Text: "bar",
		},
	}, 3)

	doc = s.Get(uri)
	expected2 := "def bar():\n    pass\n    x = 1"
	if doc.Text != expected2 {
		t.Errorf("after edit 2: expected %q, got %q", expected2, doc.Text)
	}

	// Edit 3: Add another function
	s.ApplyFullChange(uri, []lsp.TextDocumentContentChangeEvent{
		{Text: "def baz():\n    return 42\n\n" + doc.Text},
	}, 4)

	doc = s.Get(uri)
	expected3 := "def baz():\n    return 42\n\ndef bar():\n    pass\n    x = 1"
	if doc.Text != expected3 {
		t.Errorf("after edit 3: expected %q, got %q", expected3, doc.Text)
	}

	if doc.Version != 4 {
		t.Errorf("expected version 4, got %d", doc.Version)
	}
}

func TestServerStateAlwaysMatchesEditor(t *testing.T) {
	s := &Server{
		docs: make(map[lsp.DocumentURI]*Document),
	}

	uri := lsp.DocumentURI("file:///test.py")

	// Simulate editor workflow
	s.Open(lsp.TextDocumentItem{
		URI:     uri,
		Version: 1,
		Text:    "# Initial\nx = 1",
	})

	// Simulate user typing
	changes := []struct {
		version      int
		change       lsp.TextDocumentContentChangeEvent
		expectedText string
	}{
		{
			version: 2,
			change: lsp.TextDocumentContentChangeEvent{
				Range: &lsp.Range{
					Start: lsp.Position{Line: 1, Character: 4},
					End:   lsp.Position{Line: 1, Character: 5},
				},
				Text: "2",
			},
			expectedText: "# Initial\nx = 2",
		},
		{
			version: 3,
			change: lsp.TextDocumentContentChangeEvent{
				Range: &lsp.Range{
					Start: lsp.Position{Line: 1, Character: 0},
					End:   lsp.Position{Line: 1, Character: 0},
				},
				Text: "y = 0\n",
			},
			expectedText: "# Initial\ny = 0\nx = 2",
		},
		{
			version: 4,
			change: lsp.TextDocumentContentChangeEvent{
				Range: &lsp.Range{
					Start: lsp.Position{Line: 0, Character: 0},
					End:   lsp.Position{Line: 1, Character: 0},
				},
				Text: "",
			},
			expectedText: "y = 0\nx = 2",
		},
	}

	for _, ch := range changes {
		s.ApplyFullChange(uri, []lsp.TextDocumentContentChangeEvent{ch.change}, ch.version)

		doc := s.Get(uri)
		if doc.Text != ch.expectedText {
			t.Errorf("version %d: expected %q, got %q",
				ch.version, ch.expectedText, doc.Text)
		}
		if doc.Version != ch.version {
			t.Errorf("version %d: expected version %d, got %d",
				ch.version, ch.version, doc.Version)
		}
	}
}
