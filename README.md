# rahu

A Python Language Server Protocol (LSP) implementation written in Go.

## About

Rahu is a learning-focused project to understand how Python language servers work by building one from scratch. It implements a full frontend pipeline for Python code — from lexing and parsing to semantic analysis — and is now moving into LSP integration.

The project prioritizes **correct semantics, clear architecture, and LSP-grade source location tracking** over execution or runtime behavior.

---

## Current Status

🚧 **Active Development** 🚧

### Completed

* ✅ **Lexer**: Full Python tokenization with INDENT/DEDENT support

  * All Python operators (single and multi-character: `+`, `-`, `*`, `/`, `//`, `%`, `==`, `!=`, `<`, `>`, `<=`, `>=`, etc.)
  * String literals (single-line and multi-line with `"""` and `'''`)
  * Number, identifier, and keyword recognition
  * Proper indentation tracking with tab/space consistency checking
  * Accurate position tracking (line and column numbers)
  * Comprehensive test coverage

* ✅ **Parser**: Complete recursive descent parser with AST generation

  * **Statements**: assignments, if/elif/else, for loops, while loops, function definitions, return, break, continue
  * **Expressions**: binary operations, comparisons, boolean operations (`and`/`or`), unary operations, function calls, lists, tuples
  * **Operator precedence**: Correct handling of arithmetic, comparison, and boolean precedence
  * **Advanced parsing features**:

    * Function default arguments
    * Comparison chaining (`1 < x < 10`)
    * Tuple unpacking in `for` loops
    * Nested control structures
  * 40+ tests covering syntax and AST structure

* ✅ **Semantic Analysis (Analyzer)**

  * Lexical scope construction (builtin → global → function)
  * Symbol tables with ownership of scopes
  * Correct Python-style name resolution (LEGB)
  * Builtin scope support (`print`, `range`, etc.)
  * Read vs write name resolution
  * Shadowing and parameter binding
  * Control-flow legality checks (`return`, `break`, `continue`)
  * Accurate source spans for all symbols (LSP-ready)
  * Deterministic name → symbol resolution map
  * Extensive semantic tests

---

### In Progress

* 🔄 **Language Server (LSP) Integration**

  * Document lifecycle handling (open/change/close)
  * Incremental re-analysis on edits
  * Diagnostic publishing (somewhat stable; still a lot of work to do)
  * Initialization and capability negotiation are functional but evolving
  * Error recovery and resilience are being tightened across the stack

---

### Planned

* ⏳ Go-to-definition
* ⏳ Hover information
* ⏳ Find references
* ⏳ Symbol indexing
* ⏳ Incremental parsing optimizations
* ⏳ Language expansion (attributes, subscripts, classes)
* ⏳ Packaging and release automation

---

## Project Structure

```
rahu/
├── lexer/          # Tokenization
├── parser/         # AST generation
├── analyser/       # Scopes, symbols, name resolution
├── lsp/            # LSP protocol implementation (in progress)
├── utils/          # Debug printers and helpers
└── cmd/
    └── lsp/        # LSP entry point
```

---

## Building & Running

### Prerequisites

* Go 1.21 or higher

### Build

```bash
go build
```

To build the language server binary explicitly:

```bash
go build -o rahu-lsp ./cmd/lsp
```

### Run Tests

```bash
go test ./...
```

### Debug the Frontend Pipeline

```bash
go run main.go
```

This prints:

* the AST
* scope tree
* resolved names and symbols
* semantic diagnostics

---

## Example

Given this Python code:

```python
def add(x):
    return x + 1

add(4)
```

Rahu produces:

### Scopes

```
Scope(global)
  add : function
  Scope(function)
    x : parameter
```

### Name Resolution

```
USE x    parameter  at 2:12 → DEF x    parameter at 1:9
USE add  function   at 4:1  → DEF add  function  at 1:1
```

Builtins (e.g. `print`, `range`) resolve from a builtin scope per Python semantics.

---

## Design Principles

* **No execution semantics** — this is a static analysis tool
* **Clear phase separation**: lex → parse → analyze → LSP
* **No special-casing** in the resolver (builtins are just a scope)
* **LSP-grade accuracy** for spans and diagnostics
* **Tests protect invariants**, not just coverage

---

## Development Roadmap

1. Lexer ✅
2. Parser ✅
3. Semantic analysis ✅
4. LSP server 🔄
5. Language expansion ⏳

---

## Technical Highlights

* Hand-written lexer and parser
* Recursive descent + Pratt parsing
* Python-style indentation handling
* Explicit scope ownership model
* Deterministic symbol resolution
* Designed from day one for LSP consumption

---

## License

MIT

---

## Author

Akash Sivanandan
