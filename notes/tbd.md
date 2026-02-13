# Issues and Improvements

This document tracks bugs, correctness issues, and improvements across the entire codebase.

Last updated: 2026-02-12

---

## High Priority — Missing Core Python Features
---

### 5. No `*args` / `**kwargs` Support

**Location:** `parser/statements.go` (function parameter parsing)

**Problem:**
Function definitions cannot use variadic positional (`*args`) or keyword (`**kwargs`) parameters.

**Impact:** Cannot parse common Python function signatures.

---

### 6. No Class Definitions

**Problem:**
The `class` keyword is tokenized but not parsed. No AST node, parser rule, or analyser scope handling exists for classes.

**Impact:** Cannot parse any code that defines classes — a fundamental Python feature.

---

### 7. No Import Statements

**Problem:**
`import`, `from`, `as` keywords are tokenized but not parsed. No module resolution exists.

**Impact:** Cannot parse any code that uses imports. Single-file analysis only.

---

### 8. No Attribute Access (`obj.attr`)

**Problem:**
The parser has no support for dotted access expressions. Expressions like `self.x`, `os.path`, or `obj.method()` cannot be parsed.

**Impact:** Blocks class support, module member access, and method calls.

---

### 9. No Subscript / Slicing (`obj[key]`, `arr[1:3]`)

**Problem:**
No parser support for index access or slice expressions.

**Impact:** Cannot parse dictionary lookups, list indexing, or slicing — all very common Python patterns.

---

### 10. No Dictionary or Set Literals

**Problem:**
`{key: value}` and `{1, 2, 3}` are not supported in the parser.

**Impact:** Cannot parse code using dicts or sets.

---

### 11. No Try / Except / Finally

**Problem:**
Exception handling statements are not parsed.

**Impact:** Cannot parse any code with error handling.

---

### 12. No Augmented Assignment (`+=`, `-=`, etc.)

**Problem:**
Augmented assignment operators are not handled in the parser.

**Impact:** Very common Python pattern that will cause parse errors.

---

### 13. Hover Is Implemented ✅

**Location:** `server/handlers.go:19-79`

**What works:**
- Maps LSP position to parser position
- Finds Name node via nameAtPos
- Looks up symbol in doc.Symbols
- Returns hover with symbol kind (variable, function, parameter, etc.)
- Shows function signature for functions
- Shows definition line number

**Status:** IMPLEMENTED ✅

---

### 14. No Completion or References

**Problem:**
The most impactful LSP features are not implemented. The analyser already builds a `Resolved` map (`*Name -> *Symbol` with source spans) that could support find-references relatively quickly.

- ~~Go-to-definition~~ — **IMPLEMENTED** ✅
- Completion — not implemented
- Find references — not implemented

**Impact:** The LSP is not yet fully productive for daily use.

---

## Medium Priority — Code Quality

### 15. `panic()` Calls Instead of Error Returns

| Location | Description |
|----------|-------------|
| `jsonrpc/conn.go:32` | `NewConn` panics if `closeFn` is nil; should return `(*Conn, error)` |
| `utils/fileParser.go:9` | `ParseFile` panics on file read failure; should return `(string, error)` |

---

### 16. Silently Ignored Errors

| Location | Description |
|----------|-------------|
| `jsonrpc/dispatch.go:81` | `_ = conn.SendResponse(resp)` — ignores response send failure |
| `jsonrpc/conn.go:150,174` | `_ = c.closeFn()` — ignores close errors |
| `jsonrpc/conn.go:75-76` | `readLoop` silently returns on read error without reporting |
| `jsonrpc/conn.go:163-166` | `reportError` silently drops errors if channel is full |
| `jsonrpc/adapters.go:40-41` | `AdaptNotification` silently returns on unmarshal error |
| `server/handlers.go:48,62` | `_ = s.conn.Notify(...)` — ignores diagnostic notification send failures |
| `analyser/scopes.go:49,74,90,98` | `_ = b.current.Define(...)` — silently drops duplicate symbol errors (could be reported as diagnostics) |

---

### 17. Panic Recovery Discards Information

**Location:** `jsonrpc/dispatch.go:37-41`

**Problem:**
`dispatchNotification` recovers from panics but discards the panic value `r` entirely — no logging of what caused the panic.

---

### 18. Global Mutable Handler Maps

**Location:** `jsonrpc/handlers.go:10-12`

**Problem:**
`requestHandlers` and `notificationHandlers` are package-level globals. This prevents running multiple server instances and is not safe for concurrent registration.

**Fix:** Move handler maps to be per-`Conn` or per-`Server` fields.

---

### 19. Position Calculation Edge Cases

**Location:** `parser/parser.go:36-43`

**Problem:**
LSP position conversion can underflow when `Line` or `Col` is 0:
```go
Line: r.Start.Line - 1  // underflows if Line is 0
Col:  r.Start.Col - 1   // underflows if Col is 0
```

**Fix:** Clamp to 0 before subtracting.

---

### 20. Bitwise Operators Not Parsed

**Location:** `parser/operators.go`, `parser/expressions.go`

**Problem:**
Bitwise operators (`&`, `|`, `^`, `<<`, `>>`, `~`) are defined in the lexer but have no binding powers or handling in the parser.

**Impact:** Bitwise expressions cause syntax errors.

---

### 21. No Logging Infrastructure

**Problem:**
The server uses raw `log.Println` in a few places. No structured logging, log levels, or `window/logMessage` support.

**Impact:** Hard to debug issues in production. Editors have no visibility into server state.

---

### 22. Missing `with`, `lambda`, Comprehensions, Decorators, `async`/`await`, `yield`

**Problem:**
Several Python language features have no parser support:
- `with` statement (context managers)
- `lambda` expressions
- List/dict/set comprehensions and generator expressions
- Decorators (`@decorator`)
- `async def` / `await`
- `yield` / `yield from`

**Impact:** Common Python patterns cause parse errors.

---

## Low Priority — Cleanup

### 23. Typo: `RegisterNofication`

**Location:** `jsonrpc/handlers.go:18` and 5 call sites in `server/wiring.go`

**Problem:** Missing 'i' — should be `RegisterNotification`.

---

### 24. Dead Code

The following functions/types are defined but never called or used:

| Location | Name |
|----------|------|
| `parser/operators.go:47` | `isOperator()` |
| `parser/parser.go:112-116` | `advanceBy()` |
| `analyser/symbols.go:162-168` | `NewSymbol()` |
| `analyser/symbols.go:187-189` | `LookupLocal()` |
| `server/tokenize.go:5-19` | `tokenize()` — entire file |
| `utils/fileParser.go:7-17` | `ParseFile()` and `check()` |
| `lsp/diagnostics.go:20-26` | `DiagnosticError` — also has lowercase `error()` method that doesn't satisfy the `error` interface |
| `lexer/lexer.go:36` | `Token.File` field — declared but never assigned |

Additionally, 15+ LSP types in `lsp/workspace.go`, `lsp/window.go`, `lsp/text_document.go`, and `lsp/common.go` are forward-declared but unused.

---

### 25. Broken Benchmarks

**Location:** `parser/benchmark_test.go`

**Problem:** References `mega.py` and `longerScript.py` which were deleted in commit `c5183dc`. The benchmarks will fail.

**Fix:** Either recreate the fixture files or remove the benchmark file.

---

### 26. Thin Test Coverage

| Area | Gap |
|------|-----|
| **Lexer** | Only basic tokenization tested. Missing: strings, floats, comments, multiple dedents, empty input, reserved keywords |
| **Analyser** | Only 2 tests. Missing: undefined names, break/continue/return outside scope, builtins, duplicate definitions, nested scopes |
| **Integration** | No test exercises the full pipeline: JSON-RPC message -> dispatch -> handler -> parse+analyze -> publish diagnostics |
| **Parser error recovery** | 40+ happy-path tests but zero tests for malformed input |
| **CI/CD** | No GitHub Actions, Makefile, or linting configuration |

---

## Previously Fixed Issues (removed from tracking)

The following issues from the original `tbd.md` have been resolved:

| Issue | Fix Commit |
|-------|------------|
| Wrong token mapping for `>>` / missing `<<` | `2a5eb69` |
| Right-associative `**` not handled | `757be07` |
| Type assertion panic in `parseCall` | `dc6e9aa` |
| No float support in number tokenization | `aa3664c` |
| Missing position info on ILLEGAL tokens | `f45b302` |
| Expression statements limited in starting tokens | `ac37a55` |
| Tuple unpacking not supported in assignments | `f668b2c` |
| For-else clause not supported | `a93d796` |
| Go-to-definition not implemented | 2026-02-12 |

---

## Summary Table

| # | Priority | Issue | Category | Status |
|---|----------|-------|----------|--------|
| 1 | Critical | Inverted nil check in `DidOpen` | Server bug | ? |
| 2 | Critical | `DiagnosticSeverity` off by one | LSP spec violation | ? |
| 3 | Critical | JSON tag casing on `DiagnosticProvider` | LSP spec violation | ? |
| 4 | High | No escape sequences in strings | Lexer | ? |
| 5 | High | No `*args` / `**kwargs` | Parser | Open |
| 6 | High | No class definitions | Parser | Open |
| 7 | High | No import statements | Parser | Open |
| 8 | High | No attribute access | Parser | Open |
| 9 | High | No subscript / slicing | Parser | Open |
| 10 | High | No dict / set literals | Parser | Open |
| 11 | High | No try / except / finally | Parser | Open |
| 12 | High | No augmented assignment | Parser | Open |
| 13 | High | Hover is a stub | Server | Open |
| 14 | High | Go-to-definition | Server | **DONE** ✅ |
| 14b | High | Completion | Server | Open |
| 14c | High | Find references | Server | Open |
| 15 | Medium | `panic()` instead of error returns | Code quality | Open |
| 16 | Medium | Silently ignored errors (9 instances) | Code quality | Open |
| 17 | Medium | Panic recovery discards info | Code quality | Open |
| 18 | Medium | Global mutable handler maps | Architecture | Open |
| 19 | Medium | Position calculation edge cases | Parser | Open |
| 20 | Medium | Bitwise operators not parsed | Parser | Open |
| 21 | Medium | No logging infrastructure | Server | Open |
| 22 | Medium | Missing `with`, `lambda`, comprehensions, etc. | Parser | Open |
| 23 | Low | Typo: `RegisterNofication` | Cleanup | Open |
| 24 | Low | Dead code (6 functions, 15+ types) | Cleanup | Open |
| 25 | Low | Broken benchmarks | Testing | Open |
| 26 | Low | Thin test coverage | Testing | Open |
