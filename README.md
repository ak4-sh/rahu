# rahu

A Python Language Server built in Go from scratch — no LSP libraries, no Python runtime, just pure static goodness.

## Why?

Most language servers are... complicated. Rahu exists to understand Python at the source level — how does code actually flow, what names mean, how do classes inherit from each other?

It's not meant to replace Pyright or pylsp. It's meant to be **understandable**. Every line of code here should be readable, every data structure traceable. If you want to understand how a language server actually works under the hood, this is the repo.

All internal source locations are stored as **byte offsets**. Line and column only appear at the LSP boundary.


<img src="assets/demo.gif" alt="LSP Demo" width="600">

## Current Status

### Lexer — full Python tokenization

- Single and multi-character operators (`+`, `==`, `//`, `**`, `>>=`, `:=`, etc.)
- String literals (single-line and triple-quoted)
- Number literals (int, float, hex, binary, octal)
- Keywords and identifiers
- INDENT/DEDENT with tab/space consistency enforcement
- All positions stored as half-open byte ranges `[start, end)`

### Parser — recursive descent + Pratt

**Statements working:**
- Assignment and augmented assignment (`+=`, `-=`, etc.)
- `if` / `elif` / `else`
- `for` (with `else`) and `while`
- `def` with default arguments
- `class` with inheritance (base class support!)
- `return`, `break`, `continue`

**Expressions:**
- Binary ops, comparisons (including chained `1 < x < 10`)
- Boolean `and` / `or`, unary `-` / `+` / `not`
- Function calls, attribute access (`obj.attr`)
- List literals, tuple unpacking
- Right-associative `**` operator

Parser recovers from errors — it'll keep parsing even if something's syntactically wrong. Every AST node carries its byte span.

### Semantic Analyser — LEGB scoping + inheritance

- Lexical scopes: builtin → global → function → class
- Python-style name resolution (LEGB)
- Builtin functions: `print`, `range`, `len`, `int`, `str`, `bool`, `list`, `type`, `isinstance`, `abs`, `max`, `min`, `sum`, `sorted`, `enumerate`, `zip`, `map`, `filter`, `open`, `super`, `hasattr`, `getattr`, `setattr`, `input`, `float`

**Symbol kinds:**
- variable, function, class, parameter, builtin, attribute

**What it catches:**
- Undefined names
- `return` outside a function
- `break` / `continue` outside a loop

**Class inheritance:**
- Tracks base classes in class definitions
- Promotes base class members into child classes
- Overridden methods are respected (Dog's `speak` overrides Animal's)
- Instance attributes discovered via `self.x = ...` are tracked

### LSP Server — JSON-RPC 2.0 over stdio

- Initialize/shutdown lifecycle
- Document lifecycle (`didOpen`, `didChange`, `didClose`)
- Full and incremental text sync
- Publishes diagnostics (syntax + semantic errors)
- **Go-to-definition** — jumps to variables, functions, parameters, classes, attributes
- **Hover** — basic implementation showing symbol info
- `definitionProvider` and `hoverProvider` capabilities advertised

### JSON-RPC Transport

- Content-Length framing
- Request/response correlation
- Notification dispatch
- Panic recovery in handlers

## What's Missing

### Language features

- Imports (`import`, `from ... import`)
- Subscripts and slicing (`a[0]`, `a[1:3]`)
- Dictionaries and sets
- `try` / `except` / `finally`
- `*args` / `**kwargs`
- `with`, `lambda`, comprehensions, decorators
- `async` / `await`, `yield`
- Bitwise operators
- String escape sequences

### LSP features

- Find references
- Completion
- Rename
- Code actions / formatting

### Performance

- No analysis debouncing (re-parses everything on every keystroke)
- No incremental parsing
- No AST reuse across edits
- No structured logging

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
└── utils/             # Debug tools
    └── dump/          # CLI for dumping analysis output
```

## Architecture

Every time you type:

```
editor keystroke
  -> textDocument/didChange
  -> update Document text + line index
  -> lex entire file (byte offsets)
  -> parse tokens into AST
  -> build scopes
  -> resolve names
  -> promote class members (inheritance)
  -> bind attributes
  -> convert byte spans to LSP ranges
  -> publish diagnostics
```

Everything runs on byte offsets internally. Line/column only shows up when talking to the editor.

## Sample Output

Here's what Rahu produces when analysing code with inheritance:

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

The `UNBOUND` attributes are calls on instance variables (`d.speak()`) — we're not yet tracking that `d` is an instance of `Dog`, so we can't resolve method lookups on arbitrary variables. That's the next big piece.

## Getting Started

```bash
# Build
go build ./...

# Run tests
go test ./...

# Analyze a Python file
go run ./utils/dump path/to/file.py

# Use with your editor (LSP client required)
# Point your editor's Python language server to: go run ./cmd/lsp
```

## Tech Stack

- **Go 1.26** — that's it. One runtime, no dependencies worth mentioning.
- No external LSP libraries
- No Python interpreter — pure static analysis

## License

MIT

## Author

Akash Sivanandan
