# rahu

A Python Language Server built in Go from scratch — no LSP libraries, no Python runtime, just static analysis end-to-end.

## Why?

Most language servers are powerful but hard to reason about. Rahu exists to make the core mechanics visible: tokenization, parsing, scoping, name resolution, and LSP wiring.

It's not trying to replace Pyright or pylsp. It's trying to be **understandable**. Every stage should be inspectable, every data structure traceable, and every feature grounded in clear compiler-style passes.

All internal source locations are stored as **byte offsets**. Line/column translation happens only at the LSP boundary.


<img src="assets/demo.gif" alt="LSP Demo" width="600">

## Current Status

### Lexer — robust tokenization

- Single and multi-character operators (`+`, `==`, `//`, `**`, `>>=`, `:=`, etc.)
- String literals (single-line and triple-quoted)
- Number literals (int, float, hex, binary, octal)
- Keywords and identifiers
- INDENT/DEDENT with tab/space consistency enforcement
- Positions stored as half-open byte ranges `[start, end)`

### Parser — recursive descent + Pratt over an arena-backed AST

- Assignments, augmented assignments, annotated assignments, `if` / `elif` / `else`, `for`, `while`, `def`, `class`, `return`, `break`, `continue`, `pass`
- `try` / `except` / `else` / `finally`
- `import` / `from ... import ...`, including relative `from` imports
- Function calls, keyword arguments, attribute access, list/tuple/dict literals, list comprehensions, subscripts, and slices
- Parameter annotations, return annotations, and variable annotations
- Bare tuple returns like `return a, b`
- Subscript assignment targets like `a[0] = x`
- Best-effort error recovery instead of stopping at the first syntax error

The AST is stored in a compact arena with stable `NodeID`s, contiguous node storage, sibling-linked children, and side tables for names/strings/numbers.

### Semantic Analyser — LEGB scopes, imports, classes, and lightweight types

- LEGB name resolution with definition tracking and resolved name/attribute maps keyed by `NodeID`
- Builtin constants, builtin types, and a useful slice of builtin functions
- Import binding against indexed workspace modules, including relative `from` imports
- Class inheritance, member promotion, and `self.x = ...` instance attribute discovery
- Except-alias binding and comprehension-local scope handling
- Lightweight explicit type model with inferred instances, unions, annotation-driven list/tuple/dict/set typing, subscript result typing, and `list.append(...)` mutation typing
- Captures default values for parameters and simple assignments so hover/signature help can surface them
- Typed hover, signature help, and smarter completion built on top of inferred values

**Catches today:**
- Undefined names
- Undefined attributes
- Undefined base classes
- Unresolved modules
- Missing imported names
- `return` outside a function
- `break` / `continue` outside a loop

### LSP Server — JSON-RPC 2.0 over stdio

- Initialize/shutdown lifecycle
- Document lifecycle (`didOpen`, `didChange`, `didClose`)
- Full document sync is advertised to clients; the server can also apply ranged edits internally
- Publishes diagnostics (syntax + semantic errors)
- **Go-to-definition**
- **Hover**
- **Completion**
- **Signature help**
- **Semantic tokens**
- **References**
- **Rename** + **prepare rename**
- **Document symbols** + **workspace symbols**
- Startup indexing progress via LSP work-done progress

Server-side analysis stores AST, definitions, resolved symbols, semantic diagnostics, inferred types, and indexed lookup structures for fast editor features. Re-analysis is debounced on document changes, and dependent modules are refreshed through the workspace graph.

### Workspace Indexing

- Indexes Python modules under the workspace root after `initialized`
- Builds semantic snapshots in parallel, prioritizing files near the active workspace area
- Extracts exports and import dependencies
- Tracks reverse dependencies so dependents can be refreshed on change
- Uses LRU module snapshot caching to bound resident analysis state
- Prefers open-buffer contents over on-disk files
- Discovers Python environment search roots and lazily resolves external modules outside the workspace when imports require them

### JSON-RPC Transport

- Content-Length framing
- Request/response correlation
- Notification dispatch
- Panic recovery in handlers

### Testing

- JSON-RPC transport/frame tests are consolidated in `jsonrpc/jsonrpc_test.go`
- Parser coverage lives in `parser/parser_test.go`
- Semantic analysis coverage lives in `analyser/analyser_test.go`
- LSP/editor behavior is covered in focused suites such as `server/signature_help_test.go`, `server/semantic_tokens_test.go`, `server/references_test.go`, `server/rename_test.go`, and `server/prepare_rename_test.go`
- Workspace/import/indexing behavior is covered in `server/imports_test.go`, `server/workspace_test.go`, and `server/lru_test.go`
- Indexed lookup behavior is covered in `server/locate/posindex_test.go`
- CI runs `go build ./...` and `go test ./...`

### Performance

- The AST is arena-backed: nodes live in a contiguous slice and carry stable `NodeID`s, while names/strings/numbers are interned in side tables
- Document analysis builds an indexed position map for fast symbol lookup at cursor positions
- Cross-file references are served from a reference index instead of rescanning every document
- Workspace analysis uses module snapshots, export hashing, and LRU caching to reduce unnecessary rebuilds and bound resident state
- Benchmark coverage in `server/benchmark_test.go` includes startup, single-file analysis, definition/hover lookup, completion, workspace symbols, module rebuilds, and cache-pressure scenarios

## What's Missing

### Language features

- Set literals
- `*args` / `**kwargs`
- `with`, `lambda`, decorators
- Dict/set/generator comprehensions
- `async` / `await`, `yield`
- Bitwise operators
- String escape sequences
- A few Python newline / line-joining edge cases

### Typing and semantics

- Deeper return type inference beyond explicit annotations and straightforward flows
- More literal inference for dicts and sets
- More mutation typing beyond `list.append(...)`
- Maybe-undefined member diagnostics on unions
- Richer `typing` module awareness beyond builtin generic forms like `list[int]` and `dict[str, int]`
- More complete stdlib modeling and symbol metadata for external packages

### LSP features

- Code actions / formatting
- More context-aware completion ranking and filtering
- Richer semantic token coverage and modifiers

### Performance and infrastructure

- No incremental parsing
- No AST reuse across edits
- No structured logging
- External module discovery is lazy rather than fully pre-indexed

### Testing depth

- No full in-memory end-to-end LSP session suite yet
- More semantic error-path coverage is still needed (resolver/binder edge cases)
- Parser recovery and malformed-input branch coverage can be expanded further

## Project Structure

```
rahu/
├── cmd/lsp/           # Entry point — stdin/stdout -> server
├── jsonrpc/           # JSON-RPC 2.0 transport layer
├── lsp/               # LSP protocol types
├── server/            # LSP server, indexing, handlers, caching
│   └── locate/        # Cursor-to-symbol lookup logic
├── source/            # LineIndex (byte offset <-> line/column)
├── lexer/             # Python tokenizer
├── parser/            # Recursive descent + Pratt, AST
│   └── ast/           # AST node definitions and helpers
├── analyser/          # Scope builder, resolver, binder, promoter, types
├── utils/             # Debug tools
│   └── dump/          # CLI for dumping analysis output
└── notes/             # Project notes and planning docs
```

## Architecture

At startup:

```
initialize
  -> record workspace root + client capabilities
initialized
  -> index Python modules under workspace root in background
  -> build semantic snapshots in parallel
  -> extract exports + import dependencies
  -> build reverse dependency graph
  -> re-analyze open documents against the completed workspace index
```

On edit:

```
editor keystroke
  -> textDocument/didChange
  -> update Document text + line index
  -> rebuild changed module
  -> skip dependent rebuilds when export signatures are unchanged
  -> refresh affected dependents
  -> publish diagnostics
```

Inside a single analysis pass:

```
source text
  -> lex
  -> parse into arena-backed AST
  -> build scopes + definitions
  -> resolve names
  -> bind attributes
  -> promote class members
  -> infer lightweight types
  -> build lookup/reference indexes
  -> store snapshot / document analysis
```

Everything still runs on byte offsets internally. Line/column is only used for protocol I/O.

## Sample Output

The debug dump below is intentionally a low-level example of the analyser internals. It still shows the class/member model well, but it does not try to demonstrate newer editor-facing features like signature help, semantic tokens, external-module resolution, or cached workspace indexing.

```python
class Animal:
    def __init__(self, name):
        self.name = name
        self.alive = True

    def speak(self):
        return "..."

    def info(self):
        return self.name


class Dog(Animal):
    def __init__(self, name, breed):
        self.breed = breed
        self.energy = 100

    def speak(self):
        return "woof"

    def play(self):
        self.energy = self.energy - 10
        return self.energy

    def describe(self):
        return self.name + " the " + self.breed


class GuideDog(Dog):
    def __init__(self, name, breed, owner):
        self.owner = owner
        self.tasks = 0

    def assist(self):
        self.tasks = self.tasks + 1
        return self.owner

    def full_info(self):
        # inherited from Animal and Dog
        return self.name + " helps " + self.owner


def make_dog():
    d = Dog("Fido", "Labrador")
    sound = d.speak()
    remaining = d.play()
    desc = d.describe()
    return desc


def make_guide():
    g = GuideDog("Rex", "Golden", "Alice")
    a = g.assist()
    info = g.full_info()
    speech = g.speak()        # inherited override
    base = g.info()           # inherited from Animal
    return base


def zoo():
    a = Animal("Mystery")
    b = Dog("Buddy", "Poodle")
    c = GuideDog("Max", "Retriever", "Bob")

    animals = [a, b, c]

    for x in animals:
        name = x.info()
        print(name)

    return c.speak()


zoo()
```

Running this through Rahu gives:

```
=== RESOLVER STATS ===
names=76 attrs=17 pending=26 semErrs=0
```

```
=== ATTRIBUTE BINDINGS ===
BOUND   attr name -> name (unknown) at [57,61]
BOUND   attr alive -> alive (unknown) at [82,87]
...
UNBOUND attr speak at [881,886]    # instance attribute lookup
UNBOUND attr play at [907,911]     # not yet resolved via instance type
...
```

```
=== ATTRIBUTES DISCOVERED (INSTANCE) ===
Class Animal
  attr name
  attr alive
Class Dog
  attr breed
  attr energy
Class GuideDog
  attr owner
  attr tasks
```

```
=== PROMOTED CLASS MEMBERS ===
Class Dog
  member speak : function          # overrides Animal's speak
  member play : function
  member describe : function
  member breed : unknown
  member energy : unknown
  member info : function           # inherited from Animal
  member name : unknown           # inherited from Animal
  member alive : unknown          # inherited from Animal

Class GuideDog
  member assist : function
  member full_info : function
  member speak : function         # inherited from Dog (overrides Animal)
  member play : function          # inherited from Dog
  member info : function          # inherited from Animal
  ...
```

Notice how:
- `GuideDog` inherits from `Dog`, which inherits from `Animal`
- Members are properly promoted up the chain
- Overridden methods (`speak`) are correctly tagged
- Instance attributes via `self.x = ...` are tracked separately

Rahu still models inheritance, promoted members, and instance attributes the same way, but it now does a much better job with constructor-based instance typing, container element typing, and workspace-aware editor features.

## Getting Started

```bash
# Build
go build ./...

# Run tests
go test ./...

# Analyze a Python file with the debug dump tool
# (the current utility reads temp.py from the repo root)
cp path/to/file.py temp.py
go run ./utils/dump

# Run the server benchmark suite
go test -bench=. ./server

# Use with your editor (LSP client required)
# Point your editor's Python language server to: go run ./cmd/lsp
```

Tip: run `go test ./...` before wiring Rahu into an editor config so parser/analyser/server changes are validated first.

## Tech Stack

- **Go 1.26** - one runtime, minimal dependencies.
- No external LSP libraries
- No Python interpreter — pure static analysis

## License

MIT

## Author

Akash Sivanandan
