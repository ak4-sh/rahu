# Goto Definition Implementation Roadmap

**Status: IMPLEMENTED** ✅

This document details the implementation plan for `textDocument/definition` LSP support.

**Prerequisites (Already Implemented):**
- AST with source positions on all nodes (`parser/ast.go`)
- Symbol resolution producing `Resolved map[*parser.Name]*Symbol` (`analyser/resolver.go`)
- Symbol storage on Document with definition spans (`analyser/symbols.go`)
- LSP `Location` type already defined (`lsp/common.go`)
- JSON-RPC request infrastructure (`jsonrpc/adapters.go`, `jsonrpc/handlers.go`)

---

## Implementation Summary

All 5 steps were implemented:

| Step | File(s) | Status |
|------|---------|--------|
| 1. LSP types | `lsp/initialize.go` — DefinitionProvider field added | ✅ |
| 2. Node-at-position | `server/locate.go` — nameAtPos function implemented | ✅ |
| 3. Definition handler | `server/handlers.go:81-110` — Definition method implemented | ✅ |
| 4. Wire route | `server/wiring.go:47-50` — textDocument/definition registered | ✅ |
| 5. Advertise capability | `server/document.go:189` — DefinitionProvider: true | ✅ |

---

## What Was Built

### AST Walker (`server/locate.go`)

The `nameAtPos` function traverses the AST to find a `*parser.Name` node at a given position:
- Walks all statements (Assign, FunctionDef, If, WhileLoop, ExprStmt, Return)
- Recursively visits expressions (Name, BinOp, Call, Tuple, List, Compare, BooleanOp)
- Position containment check via `contains()` function

### Handler (`server/handlers.go:81-110`)

```go
func (s *Server) Definition(p *lsp.DefinitionParams) (*lsp.Location, *jsonrpc.Error) {
    // 1. Get document
    // 2. Convert LSP position to parser position
    // 3. Find name at position via nameAtPos
    // 4. Look up resolved symbol in doc.Symbols
    // 5. Return Location with symbol's span
    // 6. Handle builtins (return nil for builtins/constants/types)
}
```

---

## Testing

Test file: `server/definition_test.go`

Covers:
- Variable reference → finds the Name node
- Function call → finds the Name node in Call.Func
- Cursor between tokens → returns nil
- Cursor on literal → returns nil

---

## See Also

- [Phase 4 in features.md](./features.md#phase-4--core-language-features) — Feature roadmap
- [Issue #14 in tbd.md](./tbd.md#14-no-go-to-definition-completion-or-references) — High-level tracking issue
- LSP Specification: [textDocument/definition](https://microsoft.github.io/language-server-protocol/specifications/specification-current/#textDocument_definition)
