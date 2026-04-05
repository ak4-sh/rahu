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

- Assignments, augmented assignments, `if` / `elif` / `else`, `for`, `while`, `def`, `class`, `return`, `break`, `continue`
- `import` / `from ... import ...`, including relative `from` imports
- Function calls, keyword arguments, attribute access, list/tuple/dict literals, subscripts, and slices
- Bare tuple returns like `return a, b`
- Subscript assignment targets like `a[0] = x`
- Best-effort error recovery instead of stopping at the first syntax error

The AST is stored in a compact arena with stable `NodeID`s, contiguous node storage, sibling-linked children, and side tables for names/strings/numbers.

### Semantic Analyser — LEGB scopes, imports, classes, and lightweight types

- LEGB name resolution with definition tracking and resolved name/attribute maps keyed by `NodeID`
- Builtin constants, builtin types, and a useful slice of builtin functions
- Import binding against indexed workspace modules, including relative `from` imports
- Class inheritance, member promotion, and `self.x = ...` instance attribute discovery
- Lightweight explicit type model with inferred instances, unions, list/tuple/subscript element typing, and `list.append(...)` mutation typing
- Typed hover and smarter completion built on top of inferred values

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
- **References**
- **Rename** + **prepare rename**
- **Document symbols** + **workspace symbols**
- **Workspace-aware completion**
- Startup indexing progress via LSP work-done progress

Server-side analysis stores AST, definitions, resolved symbols, semantic diagnostics, and inferred types. Re-analysis is debounced on document changes, and dependent modules are refreshed through the workspace graph.

### Workspace Indexing

- Indexes Python modules under the workspace root at startup
- Builds semantic snapshots for every indexed module
- Extracts exports and import dependencies
- Tracks reverse dependencies so dependents can be refreshed on change
- Prefers open-buffer contents over on-disk files

### JSON-RPC Transport

- Content-Length framing
- Request/response correlation
- Notification dispatch
- Panic recovery in handlers

### Testing

- JSON-RPC transport/frame tests are consolidated in `jsonrpc/jsonrpc_test.go`
- Parser coverage lives in `parser/parser_test.go`
- Semantic analysis coverage lives in `analyser/analyser_test.go`
- Workspace/indexing/LSP behavior is covered in `server/*_test.go`
- CI runs `go build ./...` and `go test ./...`

### Performance

- The AST is now arena-backed: nodes live in a contiguous slice and carry stable `NodeID`s, while names/strings/numbers are interned in side tables
- This reduces allocation pressure and improves cache locality in parse, lookup, and semantic analysis passes
- Benchmark coverage in `server/benchmark_test.go` includes startup, analysis at multiple file sizes, definition/hover lookup, throughput-style repeated analysis, parser-only cost, and full-pipeline cost

Current benchmark snapshot (`benchstat test_results_pointers.txt test_results_arena.txt`):

```text
goos: darwin
goarch: arm64
pkg: rahu/server
cpu: Apple M2 Pro
                            │ test_results_pointers.txt │        test_results_arena.txt         │
                            │          sec/op           │    sec/op     vs base                 │
ServerStartup-10                           16.84n ± ∞ ¹   16.67n ± ∞ ¹        ~ (p=0.400 n=3) ²
AnalysisSmall-10                           84.09µ ± ∞ ¹   55.19µ ± ∞ ¹        ~ (p=0.100 n=3) ²
AnalysisMedium-10                          183.5µ ± ∞ ¹   116.9µ ± ∞ ¹        ~ (p=0.100 n=3) ²
AnalysisLarge-10                           2.151m ± ∞ ¹   1.849m ± ∞ ¹        ~ (p=0.100 n=3) ²
AnalysisExtraLarge-10                      8.795m ± ∞ ¹   7.954m ± ∞ ¹        ~ (p=0.100 n=3) ²
DefinitionLookup-10                       205.30n ± ∞ ¹   30.35n ± ∞ ¹        ~ (p=0.100 n=3) ²
HoverLookup-10                            207.10n ± ∞ ¹   34.14n ± ∞ ¹        ~ (p=0.100 n=3) ²
ThroughputAnalysisSmall-10                 86.34µ ± ∞ ¹   74.49µ ± ∞ ¹        ~ (p=0.100 n=3) ²
ThroughputAnalysisMedium-10                187.5µ ± ∞ ¹   174.6µ ± ∞ ¹        ~ (p=0.100 n=3) ²
ThroughputAnalysisLarge-10                 2.163m ± ∞ ¹   2.345m ± ∞ ¹        ~ (p=0.100 n=3) ²
DefinitionLookupAll-10                    12.169µ ± ∞ ¹   1.362µ ± ∞ ¹        ~ (p=0.100 n=3) ²
HoverLookupAll-10                         12.241µ ± ∞ ¹   1.416µ ± ∞ ¹        ~ (p=0.100 n=3) ²
ColdStartAnalysis-10                       86.74µ ± ∞ ¹   86.25µ ± ∞ ¹        ~ (p=0.700 n=3) ²
ParserOnly-10                              1.238m ± ∞ ¹   1.474m ± ∞ ¹        ~ (p=0.700 n=3) ²
FullPipeline-10                            2.299m ± ∞ ¹   2.574m ± ∞ ¹        ~ (p=0.100 n=3) ²
geomean                                    57.81µ         31.62µ        -45.31%

¹ need >= 6 samples for confidence interval at level 0.95
² need >= 4 samples to detect a difference at alpha level 0.05
```

Takeaway: the arena-backed AST is directionally faster overall, with a `-45.31%` geomean on this benchmark set and especially large wins in definition/hover lookup paths. These numbers are still preliminary because each case has only `n=3`, so they should be read as indicative rather than statistically conclusive.

## What's Missing

### Language features

- Set literals
- `try` / `except` / `finally`
- `*args` / `**kwargs`
- `with`, `lambda`, comprehensions, decorators
- `async` / `await`, `yield`
- Bitwise operators
- String escape sequences
- A few Python newline / line-joining edge cases

### Typing and semantics

- Return type inference
- Dict and set typing
- More mutation typing beyond `list.append(...)`
- Maybe-undefined member diagnostics on unions
- Annotation support
- `typing`-aware behavior
- Stdlib and external package resolution outside the workspace root

### LSP features

- Semantic tokens
- Signature help
- Code actions / formatting
- Attribute/member references
- Attribute/member rename

### Performance and infrastructure

- No incremental parsing
- No AST reuse across edits
- No structured logging
- No external environment indexing yet

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
├── server/            # LSP server, document model, handlers
│   └── locate/        # Go-to-definition lookup logic
├── source/            # LineIndex (byte offset <-> line/column)
├── lexer/             # Python tokenizer
├── parser/            # Recursive descent + Pratt, AST
│   └── ast/           # AST node definitions
├── analyser/          # Scope builder, name resolver, class promotion
│   ├── scopes.go      # Scope chain and symbol tables
│   ├── resolver.go    # Name resolution
│   ├── promoter.go    # Inheritance member promotion
│   └── binder.go      # Attribute binding
├── utils/             # Debug tools
│   └── dump/          # CLI for dumping analysis output
└── notes/             # Project notes and planning docs
```

## Architecture

At startup:

```
initialize
  -> index Python modules under workspace root
  -> build semantic snapshots
  -> extract exports + import dependencies
  -> build reverse dependency graph
```

On edit:

```
editor keystroke
  -> textDocument/didChange
  -> update Document text + line index
  -> rebuild changed module
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
  -> store snapshot / document analysis
```

Everything still runs on byte offsets internally. Line/column is only used for protocol I/O.

## Sample Output

The debug dump below is older than the current implementation. It is still useful for showing the class/member model, but it does not reflect the newer workspace import support, references/rename/completion work, or the lightweight type system.

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
