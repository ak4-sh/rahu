# rahu

A Python Language Server Protocol (LSP) implementation written in Go.

## About

Rahu is a from-scratch Python language server — lexer, parser, semantic analyser,
JSON-RPC transport, and LSP server, all hand-written in Go with no third-party
LSP libraries. It communicates over stdin/stdout using the LSP specification and
can be connected to any editor that supports custom language servers.

The project prioritizes **correct semantics, clear architecture, and LSP-grade
source location tracking** over execution or runtime behavior. There is no
interpreter or runtime — this is purely a static analysis tool.

---

## Current Status

### What Works Today

**Lexer** — full Python tokenization

- All single and multi-character operators (`+`, `==`, `//`, `**`, `>>=`, etc.)
- String literals (single-line and triple-quoted multi-line)
- Number literals (integers and floats)
- Identifier and keyword recognition
- INDENT/DEDENT emission with tab/space consistency enforcement
- 1-based line and column tracking on every token

**Parser** — recursive descent with Pratt expression parsing

- Statements: assignment, `if`/`elif`/`else`, `for` (with `else`), `while`,
  `def` (with default arguments), `return`, `break`, `continue`
- Expressions: binary operations, comparison chaining (`1 < x < 10`), boolean
  `and`/`or`, unary `-`/`+`/`not`, function calls, list literals, tuple
  unpacking
- Right-associative `**` (power) operator
- Error recovery — parsing continues after syntax errors, producing partial ASTs
- Source ranges on every AST node
- 40+ parser tests

**Semantic Analyser** — two-pass scope builder and name resolver

- Lexical scope construction (builtin -> global -> function)
- Python-style LEGB name resolution
- Builtin scope with `print`, `range`, `len`, `input`, `int`, `str`, `float`,
  `bool`, `list`, `type`, `isinstance`, `abs`, `max`, `min`, `sum`, `sorted`,
  `enumerate`, `zip`, `map`, `filter`, `open`, `super`, `hasattr`, `getattr`,
  `setattr`
- Symbol kinds: variable, function, parameter, builtin
- Detects: undefined names, `return` outside function, `break`/`continue`
  outside loop
- Source spans on all symbols (LSP-ready)
- Deterministic `*Name -> *Symbol` resolution map

**LSP Server** — JSON-RPC 2.0 over stdio

- Initialization and capability negotiation (`initialize` / `initialized`)
- Document lifecycle (`didOpen`, `didChange`, `didClose`)
- Full and incremental text synchronization (server advertises full sync,
  but incremental edit application is implemented)
- Diagnostic publishing — parse errors and semantic errors appear in the editor
  on every change
- Hover handler (registered and wired, currently returns stub data)
- Graceful shutdown (`shutdown` / `exit`)

**JSON-RPC Transport** — hand-written, no dependencies

- LSP Content-Length framing
- Request/response correlation
- Notification dispatch
- Panic recovery in handlers
- Type-safe handler registration via generics

---

### Not Yet Implemented

These are the significant gaps, roughly ordered by impact:

**Python language features the parser does not handle:**
classes, imports, attribute access (`obj.attr`), subscripts/slicing (`a[0]`),
dictionaries, sets, `try`/`except`/`finally`, augmented assignment (`+=`),
`*args`/`**kwargs`, `with`, `lambda`, comprehensions, decorators,
`async`/`await`, `yield`, string escape sequences, bitwise operators

**LSP features not yet implemented:**
hover (stub exists, needs AST position lookup), go-to-definition, find
references, completion, code actions, formatting, rename

**Infrastructure gaps:**
no debouncing of `didChange` analysis (full reparse on every keystroke),
AST cache not persisted between edits (see `incremental-parsing.md`),
no structured logging, no CI/CD

---

## Project Structure

```
rahu/
├── cmd/lsp/        # Entry point — wires stdin/stdout to the server
├── jsonrpc/        # JSON-RPC 2.0 transport (framing, dispatch, handlers)
├── lsp/            # LSP protocol type definitions
├── server/         # LSP server logic (document storage, analysis, handlers)
├── lexer/          # Python tokenizer
├── parser/         # Recursive descent + Pratt parser, AST types
├── analyser/       # Scope builder, symbol tables, name resolver
└── utils/          # Debug printers
```

---

## Building and Running

### Prerequisites

- Go 1.23 or higher

### Build

```bash
go build -o rahu-lsp ./cmd/lsp
```

### Run Tests

```bash
go test ./...
```

### Connect to Neovim

Add to your Neovim configuration:

```lua
vim.api.nvim_create_autocmd("FileType", {
    pattern = "python",
    callback = function()
        vim.lsp.start({
            name = "rahu",
            cmd = { "/path/to/rahu-lsp" },
        })
    end,
})
```

Replace `/path/to/rahu-lsp` with the absolute path to the built binary.

---

## Architecture

The pipeline on every document change:

```
editor keystroke
  -> JSON-RPC notification (textDocument/didChange)
  -> update stored document text
  -> lex entire file (lexer.Lexer)
  -> parse token stream (parser.Parser -> *parser.Module)
  -> build scope tree (analyser.BuildScopes)
  -> resolve names (analyser.Resolve)
  -> convert errors to LSP diagnostics
  -> publish diagnostics to editor
```

The lexer is pull-based (`NextToken()`), driven by the parser. The parser
produces an AST with source ranges on every node. The analyser walks the AST
twice: once to build scopes and define symbols, once to resolve every name
reference to its definition. Parse errors and semantic errors are merged into
LSP diagnostics and pushed to the editor.

---

## Design Principles

- **No execution semantics** — static analysis only
- **Clear phase separation** — lex, parse, analyse, serve
- **No special-casing** — builtins are just a scope, not hardcoded checks
- **LSP-grade accuracy** for spans and diagnostics
- **Zero third-party dependencies** for the core (only `golang.org/x/term` for
  terminal handling)

---

## License

MIT

---

## Author

Akash Sivanandan
