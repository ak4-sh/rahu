package server

import (
	"testing"

	"rahu/analyser"
	"rahu/lsp"
	"rahu/parser"
)

// TestNameAtPos tests the core position-to-name lookup functionality
func TestNameAtPos(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		line         int
		col          int
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
			name:         "name in comparison",
			code:         "x = 1\nif x > 0:\n    pass",
			line:         2,
			col:          4,
			expectedName: "x",
		},
		{
			name:         "name in while loop",
			code:         "x = 1\nwhile x < 10:\n    x = x + 1",
			line:         2,
			col:          7,
			expectedName: "x",
		},
		{
			name:         "name in list",
			code:         "x = 1\ny = 2\nz = [x, y]",
			line:         3,
			col:          6,
			expectedName: "x",
		},
		{
			name:         "name in tuple",
			code:         "x = 1\ny = (x, 2)",
			line:         2,
			col:          6,
			expectedName: "x",
		},
		{
			name:         "name in boolean operation",
			code:         "x = True\ny = False\nz = x and y",
			line:         3,
			col:          5,
			expectedName: "x",
		},
		{
			name:         "name in function default argument",
			code:         "default_val = 10\ndef foo(x=default_val):\n    pass",
			line:         2,
			col:          14,
			expectedName: "default_val",
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
		{
			name:         "name at boundary",
			code:         "xyz = 1",
			line:         1,
			col:          3,
			expectedName: "xyz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(tt.code)
			module := p.Parse()

			pos := parser.Position{Line: tt.line, Col: tt.col}
			name := nameAtPos(module, pos)

			if tt.expectedName == "" {
				if name != nil {
					t.Errorf("expected nil, got name %q", name.ID)
				}
				return
			}

			if name == nil {
				t.Fatalf("expected name %q, got nil", tt.expectedName)
			}

			if name.ID != tt.expectedName {
				t.Errorf("expected name %q, got %q", tt.expectedName, name.ID)
			}
		})
	}
}

// TestNameAtPos_NilModule tests with nil module
func TestNameAtPos_NilModule(t *testing.T) {
	result := nameAtPos(nil, parser.Position{Line: 1, Col: 1})
	if result != nil {
		t.Error("expected nil for nil module")
	}
}

// TestNameInStmt_NilStmt tests with nil statement
func TestNameInStmt_NilStmt(t *testing.T) {
	result := nameInStmt(nil, parser.Position{Line: 1, Col: 1})
	if result != nil {
		t.Error("expected nil for nil statement")
	}
}

// TestContains tests the contains helper function
func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		rng      parser.Range
		pos      parser.Position
		expected bool
	}{
		{
			name:     "position inside range",
			rng:      parser.Range{Start: parser.Position{Line: 1, Col: 1}, End: parser.Position{Line: 1, Col: 5}},
			pos:      parser.Position{Line: 1, Col: 3},
			expected: true,
		},
		{
			name:     "position at start boundary",
			rng:      parser.Range{Start: parser.Position{Line: 1, Col: 1}, End: parser.Position{Line: 1, Col: 5}},
			pos:      parser.Position{Line: 1, Col: 1},
			expected: true,
		},
		{
			name:     "position at end boundary",
			rng:      parser.Range{Start: parser.Position{Line: 1, Col: 1}, End: parser.Position{Line: 1, Col: 5}},
			pos:      parser.Position{Line: 1, Col: 5},
			expected: true,
		},
		{
			name:     "position before range",
			rng:      parser.Range{Start: parser.Position{Line: 1, Col: 5}, End: parser.Position{Line: 1, Col: 10}},
			pos:      parser.Position{Line: 1, Col: 1},
			expected: false,
		},
		{
			name:     "position after range",
			rng:      parser.Range{Start: parser.Position{Line: 1, Col: 1}, End: parser.Position{Line: 1, Col: 5}},
			pos:      parser.Position{Line: 1, Col: 10},
			expected: false,
		},
		{
			name:     "position on different line",
			rng:      parser.Range{Start: parser.Position{Line: 1, Col: 1}, End: parser.Position{Line: 1, Col: 5}},
			pos:      parser.Position{Line: 2, Col: 3},
			expected: false,
		},
		{
			name:     "multi-line range",
			rng:      parser.Range{Start: parser.Position{Line: 1, Col: 1}, End: parser.Position{Line: 3, Col: 10}},
			pos:      parser.Position{Line: 2, Col: 5},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.rng, tt.pos)
			if result != tt.expected {
				t.Errorf("contains(%+v, %+v) = %v, expected %v", tt.rng, tt.pos, result, tt.expected)
			}
		})
	}
}

// TestDefinition tests the main Definition handler
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
		{
			name:         "goto variable definition",
			code:         "x = 1\ny = x",
			line:         1, // 0-indexed in LSP
			character:    4, // cursor on 'x' in 'y = x'
			expectError:  false,
			expectedLine: 0, // definition on line 0
			expectedChar: 0,
		},
		{
			name:         "goto function definition",
			code:         "def foo():\n    pass\nfoo()",
			line:         2,
			character:    0,
			expectError:  false,
			expectedLine: 0,
			expectedChar: 4,
		},
		{
			name:         "goto definition in expression",
			code:         "a = 1\nb = 2\nc = a + b",
			line:         2,
			character:    4,
			expectError:  false,
			expectedLine: 0,
			expectedChar: 0,
		},
		{
			name:        "builtin function returns error",
			code:        "print(1)",
			line:        0,
			character:   0,
			expectError: true,
		},
		{
			name:        "constant returns error",
			code:        "x = True",
			line:        0,
			character:   4,
			expectError: true,
		},
		{
			name:        "type returns error",
			code:        "x = int(5)",
			line:        0,
			character:   4,
			expectError: true,
		},
		{
			name:        "position without name returns error",
			code:        "x = 1",
			line:        0,
			character:   20,
			expectError: true,
		},
		{
			name:        "undefined variable returns error",
			code:        "y = x",
			line:        0,
			character:   4,
			expectError: true,
		},
		{
			name:         "nil document returns error",
			expectNilDoc: true,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				docs: make(map[lsp.DocumentURI]*Document),
			}

			uri := lsp.DocumentURI("file:///test.py")

			if !tt.expectNilDoc {
				// Parse and analyze the code
				p := parser.New(tt.code)
				module := p.Parse()
				global := analyser.BuildScopes(module)
				_, resolved := analyser.Resolve(module, global)

				// Create document with analysis results
				s.docs[uri] = &Document{
					URI:     uri,
					Version: 1,
					Text:    tt.code,
					AST:     module,
					Symbols: resolved,
				}
			}

			params := &lsp.DefinitionParams{
				TextDocument: lsp.TextDocumentIdentifier{URI: uri},
				Position: lsp.Position{
					Line:      tt.line,
					Character: tt.character,
				},
			}

			location, err := s.Definition(params)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if location == nil {
				t.Fatal("expected location, got nil")
			}

			if location.URI != uri {
				t.Errorf("expected URI %q, got %q", uri, location.URI)
			}

			if location.Range.Start.Line != tt.expectedLine {
				t.Errorf("expected line %d, got %d", tt.expectedLine, location.Range.Start.Line)
			}

			if location.Range.Start.Character != tt.expectedChar {
				t.Errorf("expected character %d, got %d", tt.expectedChar, location.Range.Start.Character)
			}
		})
	}
}

// TestDefinitionWithFunctionScope tests goto definition within function scopes
func TestDefinitionWithFunctionScope(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		line         int
		character    int
		expectedLine int
		expectedChar int
	}{
		{
			name:         "parameter reference in function",
			code:         "def foo(x):\n    return x",
			line:         1,
			character:    11,
			expectedLine: 0,
			expectedChar: 8,
		},
		{
			name:         "local variable reference",
			code:         "def foo():\n    x = 1\n    return x",
			line:         2,
			character:    11,
			expectedLine: 1,
			expectedChar: 4,
		},
		{
			name:         "global variable from function",
			code:         "x = 1\ndef foo():\n    return x",
			line:         2,
			character:    11,
			expectedLine: 0,
			expectedChar: 0,
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
				URI:     uri,
				Version: 1,
				Text:    tt.code,
				AST:     module,
				Symbols: resolved,
			}

			params := &lsp.DefinitionParams{
				TextDocument: lsp.TextDocumentIdentifier{URI: uri},
				Position: lsp.Position{
					Line:      tt.line,
					Character: tt.character,
				},
			}

			location, err := s.Definition(params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if location.Range.Start.Line != tt.expectedLine {
				t.Errorf("expected line %d, got %d", tt.expectedLine, location.Range.Start.Line)
			}

			if location.Range.Start.Character != tt.expectedChar {
				t.Errorf("expected character %d, got %d", tt.expectedChar, location.Range.Start.Character)
			}
		})
	}
}

// TestDefinitionWithControlFlow tests goto definition within control flow structures
func TestDefinitionWithControlFlow(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		line         int
		character    int
		expectedLine int
		expectedChar int
	}{
		{
			name:         "variable in if condition",
			code:         "x = 1\nif x > 0:\n    pass",
			line:         1,
			character:    3,
			expectedLine: 0,
			expectedChar: 0,
		},
		{
			name:         "variable in if body",
			code:         "x = 1\nif True:\n    y = x",
			line:         2,
			character:    8,
			expectedLine: 0,
			expectedChar: 0,
		},
		{
			name:         "variable in else body",
			code:         "x = 1\nif False:\n    pass\nelse:\n    y = x",
			line:         4,
			character:    8,
			expectedLine: 0,
			expectedChar: 0,
		},
		{
			name:         "variable in while condition",
			code:         "x = 1\nwhile x < 10:\n    pass",
			line:         1,
			character:    6,
			expectedLine: 0,
			expectedChar: 0,
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
				URI:     uri,
				Version: 1,
				Text:    tt.code,
				AST:     module,
				Symbols: resolved,
			}

			params := &lsp.DefinitionParams{
				TextDocument: lsp.TextDocumentIdentifier{URI: uri},
				Position: lsp.Position{
					Line:      tt.line,
					Character: tt.character,
				},
			}

			location, err := s.Definition(params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if location.Range.Start.Line != tt.expectedLine {
				t.Errorf("expected line %d, got %d", tt.expectedLine, location.Range.Start.Line)
			}

			if location.Range.Start.Character != tt.expectedChar {
				t.Errorf("expected character %d, got %d", tt.expectedChar, location.Range.Start.Character)
			}
		})
	}
}

// TestDefinitionComplexExpressions tests goto definition in complex expressions
func TestDefinitionComplexExpressions(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		line         int
		character    int
		expectedLine int
		expectedChar int
	}{
		{
			name:         "nested function calls",
			code:         "x = 1\ny = max(x, 2)",
			line:         1,
			character:    8,
			expectedLine: 0,
			expectedChar: 0,
		},
		{
			name:         "chained comparisons",
			code:         "x = 1\ny = 10\nif 0 < x < y:\n    pass",
			line:         2,
			character:    7,
			expectedLine: 0,
			expectedChar: 0,
		},
		{
			name:         "list comprehension-like",
			code:         "x = 1\ny = 2\nz = [x, y]",
			line:         2,
			character:    5,
			expectedLine: 0,
			expectedChar: 0,
		},
		{
			name:         "tuple unpacking reference",
			code:         "x = 1\na, b = (x, 2)",
			line:         1,
			character:    8,
			expectedLine: 0,
			expectedChar: 0,
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
				URI:     uri,
				Version: 1,
				Text:    tt.code,
				AST:     module,
				Symbols: resolved,
			}

			params := &lsp.DefinitionParams{
				TextDocument: lsp.TextDocumentIdentifier{URI: uri},
				Position: lsp.Position{
					Line:      tt.line,
					Character: tt.character,
				},
			}

			location, err := s.Definition(params)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if location.Range.Start.Line != tt.expectedLine {
				t.Errorf("expected line %d, got %d", tt.expectedLine, location.Range.Start.Line)
			}

			if location.Range.Start.Character != tt.expectedChar {
				t.Errorf("expected character %d, got %d", tt.expectedChar, location.Range.Start.Character)
			}
		})
	}
}
