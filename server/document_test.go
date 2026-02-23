package server

import (
	"testing"

	"rahu/analyser"
	"rahu/lsp"
	"rahu/parser"
	"rahu/parser/ast"
	"rahu/source"
)

// -------------------------
// nameAtPos tests
// -------------------------

func TestNameAtPos_LSP(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		line         int // 1-based (human readable)
		col          int // 1-based
		expectedName string
	}{
		{
			name:         "simple variable reference",
			code:         "x = 1\ny = x",
			line:         2,
			col:          5,
			expectedName: "x",
		},
		{
			name:         "variable on first line",
			code:         "foo = 42",
			line:         1,
			col:          1,
			expectedName: "foo",
		},
		{
			name:         "name in binary operation",
			code:         "a = 1\nb = 2\nc = a + b",
			line:         3,
			col:          5,
			expectedName: "a",
		},
		{
			name:         "name in function call",
			code:         "x = 1\nprint(x)",
			line:         2,
			col:          7,
			expectedName: "x",
		},
		{
			name:         "position outside any name",
			code:         "x = 1",
			line:         1,
			col:          10,
			expectedName: "",
		},
		{
			name:         "empty module",
			code:         "",
			line:         1,
			col:          1,
			expectedName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(tt.code)
			module := p.Parse()

			li := source.NewLineIndex(tt.code)
			offset := li.PositionToOffset(tt.line-1, tt.col-1)

			name := nameAtPos(module, offset)

			if tt.expectedName == "" {
				if name != nil {
					t.Fatalf("expected nil, got %q", name.ID)
				}
				return
			}

			if name == nil {
				t.Fatalf("expected %q, got nil", tt.expectedName)
			}

			if name.ID != tt.expectedName {
				t.Fatalf("expected %q, got %q", tt.expectedName, name.ID)
			}
		})
	}
}

func TestNameAtPos_NilModule_LSP(t *testing.T) {
	if nameAtPos(nil, 0) != nil {
		t.Fatal("expected nil for nil module")
	}
}

// -------------------------
// contains tests
// -------------------------

func TestContains_LSP(t *testing.T) {
	rng := ast.Range{Start: 10, End: 20}

	tests := []struct {
		pos      int
		expected bool
	}{
		{10, true},
		{15, true},
		{20, true},
		{9, false},
		{21, false},
	}

	for _, tt := range tests {
		if got := contains(rng, tt.pos); got != tt.expected {
			t.Fatalf("contains(%d) = %v, expected %v", tt.pos, got, tt.expected)
		}
	}
}

// -------------------------
// Definition tests
// -------------------------

func TestDefinition_LSP(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		line         int
		character    int
		expectError  bool
		expectedLine int
		expectedChar int
	}{
		{
			name:         "goto variable definition",
			code:         "x = 1\ny = x",
			line:         1,
			character:    4,
			expectedLine: 0,
			expectedChar: 0,
		},
		{
			name:         "goto function definition",
			code:         "def foo():\n    pass\nfoo()",
			line:         2,
			character:    0,
			expectedLine: 0,
			expectedChar: 4,
		},
		{
			name:        "builtin function returns error",
			code:        "print(1)",
			line:        0,
			character:   0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				docs: make(map[lsp.DocumentURI]*Document),
			}

			uri := lsp.DocumentURI("file:///test.py")

			p := parser.New(tt.code)
			module := p.Parse()
			global := analyser.BuildScopes(module)
			_, resolved := analyser.Resolve(module, global)

			s.docs[uri] = &Document{
				URI:       uri,
				Version:   1,
				Text:      tt.code,
				LineIndex: source.NewLineIndex(tt.code),
				AST:       module,
				Symbols:   resolved,
			}

			params := &lsp.DefinitionParams{
				TextDocument: lsp.TextDocumentIdentifier{URI: uri},
				Position: lsp.Position{
					Line:      tt.line,
					Character: tt.character,
				},
			}

			loc, err := s.Definition(params)

			if tt.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if loc.Range.Start.Line != tt.expectedLine {
				t.Fatalf("expected line %d, got %d", tt.expectedLine, loc.Range.Start.Line)
			}

			if loc.Range.Start.Character != tt.expectedChar {
				t.Fatalf("expected char %d, got %d", tt.expectedChar, loc.Range.Start.Character)
			}
		})
	}
}

// -------------------------
// Shadowing
// -------------------------

func TestDefinition_Shadowing(t *testing.T) {
	code := `x = 1
def foo():
    x = 2
    return x
y = x
`

	s := &Server{
		docs: make(map[lsp.DocumentURI]*Document),
	}

	uri := lsp.DocumentURI("file:///test.py")

	p := parser.New(code)
	module := p.Parse()
	global := analyser.BuildScopes(module)
	_, resolved := analyser.Resolve(module, global)

	s.docs[uri] = &Document{
		URI:       uri,
		Version:   1,
		Text:      code,
		LineIndex: source.NewLineIndex(code),
		AST:       module,
		Symbols:   resolved,
	}

	tests := []struct {
		line         int
		character    int
		expectedLine int
		expectedChar int
	}{
		{3, 11, 2, 4},
		{4, 4, 0, 0},
	}

	for _, tt := range tests {
		params := &lsp.DefinitionParams{
			TextDocument: lsp.TextDocumentIdentifier{URI: uri},
			Position: lsp.Position{
				Line:      tt.line,
				Character: tt.character,
			},
		}

		loc, err := s.Definition(params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if loc.Range.Start.Line != tt.expectedLine {
			t.Fatalf("expected line %d, got %d", tt.expectedLine, loc.Range.Start.Line)
		}
	}
}
