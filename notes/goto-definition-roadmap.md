# Goto Definition Implementation Roadmap

This document details the implementation plan for `textDocument/definition` LSP support.

**Prerequisites (Already Implemented):**
- AST with source positions on all nodes (`parser/ast.go`)
- Symbol resolution producing `Resolved map[*parser.Name]*Symbol` (`analyser/resolver.go`)
- Symbol storage on Document with definition spans (`analyser/symbols.go`)
- LSP `Location` type already defined (`lsp/common.go`)
- JSON-RPC request infrastructure (`jsonrpc/adapters.go`, `jsonrpc/handlers.go`)

---

## Implementation Steps

### Step 1 — Add LSP Types

**Files:**
- `lsp/initialize.go` — Add `DefinitionProvider bool` field to `ServerCapabilities` struct
- `lsp/window.go` (or `lsp/text_document.go`) — Add `DefinitionParams` struct:
  ```go
  type DefinitionParams struct {
      TextDocument TextDocumentIdentifier `json:"textDocument"`
      Position     Position               `json:"position"`
  }
  ```

**Effort:** ~5 minutes  
**Dependencies:** None

---

### Step 2 — AST Node-at-Position Lookup (Core Work)

**New file:** `server/locate.go`

**Function signature:**
```go
func nameAtPosition(module *parser.Module, pos parser.Position) *parser.Name
```

**Responsibilities:**
- Walk the entire AST recursively
- Find the `*parser.Name` node whose `Pos` range contains the given cursor position
- Return `nil` if no name exists at that position

**Nodes to traverse:**
- **Statements:** `Assign`, `FunctionDef`, `ExprStmt`, `If`, `For`, `WhileLoop`, `Return`, `Break`, `Continue`, `ClassDef`
- **Expressions:** `Name` (check containment, return if match), `BinOp`, `UnaryOp`, `BooleanOp`, `Compare`, `Call`, `Tuple`, `List`
- **Literals:** `Number`, `String`, `Boolean` (skip, no names inside)

**Position containment check:**
```
pos >= range.Start && pos <= range.End
```
(lexicographic comparison on (line, col) pairs)

**Edge case — FunctionDef:** The function name string field is not a `*parser.Name` node. Handling "goto definition on the definition itself" (jump to self) is optional and can be deferred.

**New file:** `server/locate_test.go`

**Test cases:**
- Variable reference → should find the Name node
- Function call → should find the Name node in Call.Func
- Cursor between tokens → should return nil
- Cursor on literal → should return nil

**Effort:** ~30 minutes  
**Dependencies:** None

---

### Step 3 — Definition Handler

**File:** `server/handlers.go`

**Method signature:**
```go
func (s *Server) Definition(p *lsp.DefinitionParams) (*lsp.Location, *jsonrpc.Error)
```

**Algorithm:**
1. Get document: `doc := s.Get(p.TextDocument.URI)`
2. Return error if document not found
3. Convert LSP position to parser position: `pos := parser.Position{Line: p.Position.Line, Col: p.Position.Character}`
4. Find name at position: `name := nameAtPosition(doc.AST, pos)`
5. Return `nil` if no name found (valid LSP response)
6. Look up resolved symbol: `sym, ok := doc.Symbols[name]`
7. Return `nil` if not resolved (valid LSP response)
8. **Handle builtins:** If `sym.Kind` is `SymBuiltin`, `SymConstant`, or `SymType`, or if `sym.Span` is zero-value, return `nil` (can't jump to builtin definitions)
9. Return location:
   ```go
   &lsp.Location{
       URI:   doc.URI,
       Range: toRange(sym.Span),  // toRange() already exists in analysis.go
   }
   ```

**Effort:** ~10 minutes  
**Dependencies:** Steps 1, 2

---

### Step 4 — Wire the Route

**File:** `server/wiring.go`

**Addition in `Register()` function:**
```go
jsonrpc.RegisterRequest(
    "textDocument/definition",
    jsonrpc.AdaptRequest(s.Definition),
)
```

This follows the same pattern as `textDocument/hover` at lines 17-20.

**Effort:** ~2 minutes  
**Dependencies:** Step 3

---

### Step 5 — Advertise Capability

**File:** `server/document.go`

**Location:** `Initialize` method, lines 185-189

**Change:**
```go
Capabilities: lsp.ServerCapabilities{
    TextDocumentSync:   lsp.TDSKFull,
    HoverProvider:      true,
    DefinitionProvider: true,  // <-- add this
},
```

**Effort:** ~1 minute  
**Dependencies:** Step 1

---

## Summary

| Step | File(s) | Effort | Dependencies |
|------|---------|--------|--------------|
| 1. LSP types | `lsp/initialize.go`, `lsp/window.go` | ~5 min | None |
| 2. Node-at-position | `server/locate.go` (new), `server/locate_test.go` (new) | ~30 min | None |
| 3. Definition handler | `server/handlers.go` | ~10 min | Steps 1, 2 |
| 4. Wire route | `server/wiring.go` | ~2 min | Step 3 |
| 5. Advertise capability | `server/document.go` | ~1 min | Step 1 |

**Total estimated effort: ~45 minutes**

The only non-trivial work is Step 2 (AST walker). Steps 3-5 are mechanical wiring following established codebase patterns.

---

## See Also

- [Issue #14 in tbd.md](./tbd.md#14-no-go-to-definition-completion-or-references) — High-level tracking issue
- [Phase 4 in features.md](./features.md#phase-4--core-language-features) — Feature roadmap
- LSP Specification: [textDocument/definition](https://microsoft.github.io/language-server-protocol/specifications/specification-current/#textDocument_definition)
