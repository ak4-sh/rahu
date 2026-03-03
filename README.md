# rahu

A Python Language Server Protocol (LSP) implementation written in Go.

## About

Rahu is a from-scratch Python language server — lexer, parser, semantic analyser,
JSON-RPC transport, and LSP server, all hand-written in Go with no third-party
LSP libraries. It communicates over stdin/stdout using the LSP specification and
can be connected to any editor that supports custom language servers.

The project prioritizes **correct semantics, clear architecture, and precise
source mapping** over execution or runtime behavior. There is no interpreter or
interop with a Python runtime - this is purely a static analysis tool.

All internal source locations are represented as **byte offsets**, with line and
column information derived only at API boundaries (LSP, diagnostics, editor
integration).


## Current Status

### What Works Today

### Lexer — full Python tokenization (byte-offset based)

- All single and multi-character operators (`+`, `==`, `//`, `**`, `>>=`, etc.)
- String literals (single-line and triple-quoted multi-line)
- Number literals (integers and floats)
- Identifier and keyword recognition
- INDENT/DEDENT emission with tab/space consistency enforcement
- Token positions stored as half-open byte ranges `[start, end)`
- No line/column tracking in the lexer (derived later via line index)

### Parser — recursive descent with Pratt expression parsing

**Statements**
- Assignment
- Augmented assignment (`+=`, `-=`, etc.)
- `if` / `elif` / `else`
- `for` (with `else`)
- `while`
- `def` (with default arguments)
- `class` (ClassDef implemented)
- `return`, `break`, `continue`

**Expressions**
- Binary operations
- Comparison chaining (`1 < x < 10`)
- Boolean `and` / `or`
- Unary `-` / `+` / `not`
- Function calls
- Attribute access (`obj.attr`)
- List literals
- Tuple literals and unpacking

- Right-associative `**` (power) operator
- Error recovery — parsing continues after syntax errors
- Source ranges on every AST node (byte offsets)
- Extensive parser test coverage

### Semantic Analyser — two-pass scope builder and resolver

- Lexical scope construction:
  - builtin → global → function → class
- Python-style LEGB name resolution
- Builtin scope includes:
  `print`, `range`, `len`, `input`, `int`, `str`, `float`, `bool`, `list`,
  `type`, `isinstance`, `abs`, `max`, `min`, `sum`, `sorted`, `enumerate`,
  `zip`, `map`, `filter`, `open`, `super`, `hasattr`, `getattr`, `setattr`
- Symbol kinds:
  - variable
  - function
  - class
  - parameter
  - builtin
- Detects:
  - undefined names
  - `return` outside function
  - `break` / `continue` outside loop
- Deterministic `*Name → *Symbol` resolution map
- Source spans stored as byte ranges (LSP-ready)

### LSP Server — JSON-RPC 2.0 over stdio

- Initialization and capability negotiation
- Document lifecycle (`didOpen`, `didChange`, `didClose`)
- Full and incremental text synchronization
- Central `Document` model with:
  - raw text
  - persistent line index (offset ↔ position mapping)
- Diagnostic publishing (syntax + semantic errors)
- Go-to-definition (`textDocument/definition`)
  - variables
  - functions
  - parameters
  - class names
  - attribute access
- Graceful shutdown (`shutdown` / `exit`)
- DefinitionProvider capability advertised

### JSON-RPC Transport

- LSP Content-Length framing
- Request/response correlation
- Notification dispatch
- Panic recovery in handlers
- Type-safe handler registration


## Not Yet Implemented

### Language features missing or incomplete

- Imports (`import`, `from ... import`)
- Subscripts and slicing (`a[0]`, `a[1:3]`)
- Dictionaries and sets
- `try` / `except` / `finally`
- `*args` / `**kwargs`
- `with`
- `lambda`
- Comprehensions
- Decorators
- `async` / `await`
- `yield`
- Bitwise operators
- String escape sequence processing

### LSP features missing

- Hover (currently minimal / stub)
- Find references
- Completion
- Rename
- Code actions
- Formatting

### Infrastructure gaps

- No analysis debouncing (full reparse on every keystroke)
- No incremental parsing
- No AST reuse across edits
- No structured logging
- No CI


## Project Structure

```

rahu/
├── cmd/lsp/        # Entry point — wires stdin/stdout to the server
├── jsonrpc/        # JSON-RPC 2.0 transport
├── lsp/            # LSP protocol type definitions
├── server/         # LSP server logic, document model, handlers
├── source/         # LineIndex (offset <-> line/column mapping)
├── lexer/          # Python tokenizer (byte-offset based)
├── parser/         # Recursive descent + Pratt parser, AST
├── analyser/       # Scope builder and name resolver
└── utils/          # Debug / development helpers

```


## Architecture

Pipeline on every document change:

```

editor keystroke
-> textDocument/didChange
-> update Document text + line index
-> lex entire file (byte offsets)
-> parse tokens into AST
-> build scopes
-> resolve names
-> convert byte spans to LSP ranges
-> publish diagnostics

```

The lexer and parser operate entirely on byte offsets.
Line and column positions are derived only when interacting with the editor.


## License

MIT


## Author

Akash Sivanandan
