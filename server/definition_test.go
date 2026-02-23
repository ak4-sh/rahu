package server

import (
	"testing"

	"rahu/analyser"
	"rahu/lsp"
	"rahu/parser"
	"rahu/parser/ast"
	"rahu/source"
)

// --------------------
// Name lookup tests
// --------------------

func TestNameAtPos(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		line         int // 1-based (human)
		col          int // 1-based (human)
		expectedName string
	}{
		{"simple variable reference", "x = 1\ny = x", 2, 5, "x"},
		{"variable on first line", "foo = 42", 1, 1, "foo"},
		{"name in binary operation", "a = 1\nb = 2\nc = a + b", 3, 5, "a"},
		{"name in function call", "x = 1\nprint(x)", 2, 7, "x"},
		{"name in comparison", "x = 1\nif x > 0:\n    pass", 2, 4, "x"},
		{"name in while loop", "x = 1\nwhile x < 10:\n    x = x + 1", 2, 7, "x"},
		{"name in list", "x = 1\ny = 2\nz = [x, y]", 3, 6, "x"},
		{"name in tuple", "x = 1\ny = (x, 2)", 2, 6, "x"},
		{"name in boolean operation", "x = True\ny = False\nz = x and y", 3, 5, "x"},
		{"name in function default argument", "default_val = 10\ndef foo(x=default_val):\n    pass", 2, 14, "default_val"},
		{"position outside any name", "x = 1", 1, 10, ""},
		{"empty module", "", 1, 1, ""},
		{"name at boundary", "xyz = 1", 1, 3, "xyz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(tt.code)
			module := p.Parse()
			li := source.NewLineIndex(tt.code)

			// convert human (1-based) → LSP (0-based) → offset
			offset := li.PositionToOffset(tt.line-1, tt.col-1)

			name := nameAtPos(module, offset)

			if tt.expectedName == "" {
				if name != nil {
					t.Errorf("expected nil, got %q", name.ID)
				}
				return
			}

			if name == nil {
				t.Fatalf("expected %q, got nil", tt.expectedName)
			}

			if name.ID != tt.expectedName {
				t.Errorf("expected %q, got %q", tt.expectedName, name.ID)
			}
		})
	}
}

func TestNameAtPos_NilModule(t *testing.T) {
	if nameAtPos(nil, 0) != nil {
		t.Error("expected nil for nil module")
	}
}

// --------------------
// contains() tests
// --------------------

func TestContains(t *testing.T) {
	code := "hello\nworld"
	li := source.NewLineIndex(code)

	start := li.PositionToOffset(0, 0) // h
	end := li.PositionToOffset(0, 5)   // o

	rng := ast.Range{Start: start, End: end}

	tests := []struct {
		name     string
		line     int
		char     int
		expected bool
	}{
		{"inside", 0, 2, true},
		{"start boundary", 0, 0, true},
		{"end boundary", 0, 5, true},
		{"outside", 1, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos := li.PositionToOffset(tt.line, tt.char)
			if contains(rng, pos) != tt.expected {
				t.Fatalf("unexpected result")
			}
		})
	}
}

// --------------------
// Definition tests
// --------------------

func TestDefinition(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		line         int
		character    int
		expectError  bool
		expectedLine int
		expectedChar int
		expectNilDoc bool
	}{
		{"goto variable definition", "x = 1\ny = x", 1, 4, false, 0, 0, false},
		{"goto function definition", "def foo():\n    pass\nfoo()", 2, 0, false, 0, 4, false},
		{"builtin returns error", "print(1)", 0, 0, true, 0, 0, false},
		{"nil document", "", 0, 0, true, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{docs: map[lsp.DocumentURI]*Document{}}
			uri := lsp.DocumentURI("file:///test.py")

			if !tt.expectNilDoc {
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
			}

			loc, err := s.Definition(&lsp.DefinitionParams{
				TextDocument: lsp.TextDocumentIdentifier{URI: uri},
				Position: lsp.Position{
					Line:      tt.line,
					Character: tt.character,
				},
			})

			if tt.expectError {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if loc.Range.Start.Line != tt.expectedLine {
				t.Errorf("expected line %d, got %d", tt.expectedLine, loc.Range.Start.Line)
			}

			if loc.Range.Start.Character != tt.expectedChar {
				t.Errorf("expected char %d, got %d", tt.expectedChar, loc.Range.Start.Character)
			}
		})
	}
}
