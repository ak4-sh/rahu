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


## Current pipeline sample output

```text
=== SOURCE ===
class Base:
    def __init__(self):
        self.base_only = 1

    def base_method(self):
        return self.base_only


class Child(Base):
    def __init__(self):
        self.x = 10
        self.y = 20
        self.x = 99          # re-assign same attr; should still be a known attr

    def sum(self):
        return self.x + self.y

    def touch(self):
        z = self.x           # base is self, attr is x
        return z

    def unknown_attr_read(self):
        return self.nope     # should NOT be resolvable as attr yet unless you special-case self.* lookup

    def unknown_attr_write(self):
        self.new_attr = 123  # should be recorded as an instance attr in scope-building


def top_level():
    c = Child()
    a = c.x                 # base name "c" should resolve; member "x" not resolvable without types
    b = c.sum()             # base name "c" should resolve; member "sum" not resolvable without types
    return a


top_level()


=== SCOPES ===
Scope(global)
  Base : class
    Scope(unknown)
      __init__ : function
        Scope(function)
          self : parameter
      base_method : function
        Scope(function)
          self : parameter
  Child : class
    Scope(unknown)
      unknown_attr_read : function
        Scope(function)
          self : parameter
      unknown_attr_write : function
        Scope(function)
          self : parameter
      __init__ : function
        Scope(function)
          self : parameter
      sum : function
        Scope(function)
          self : parameter
      touch : function
        Scope(function)
          self : parameter
          z : variable
  top_level : function
    Scope(function)
      c : variable
      a : variable
      b : variable

=== RESOLVER STATS ===
names=31 attrs=9 pending=12 semErrs=1

=== SEMANTIC ERRORS ===
{486 490}: undefined attribute: nope

=== RESOLVED NAMES ===
top_level @ [947,956] -> top_level (function)
self @ [106,110] -> self (parameter)
Child @ [123,134] -> Child (class)
self @ [481,485] -> self (parameter)
c @ [838,839] -> c (variable)
Base @ [0,10] -> Base (class)
__init__ @ [150,158] -> __init__ (function)
c @ [738,739] -> c (variable)
self @ [194,198] -> self (parameter)
sum @ [296,299] -> sum (function)
self @ [331,335] -> self (parameter)
touch @ [347,352] -> touch (function)
z @ [430,431] -> z (variable)
Base @ [135,139] -> Base (class)
self @ [214,218] -> self (parameter)
self @ [372,376] -> self (parameter)
z @ [368,369] -> z (variable)
unknown_attr_write @ [581,599] -> unknown_attr_write (function)
__init__ @ [20,28] -> __init__ (function)
self @ [44,48] -> self (parameter)
self @ [174,178] -> self (parameter)
self @ [322,326] -> self (parameter)
unknown_attr_read @ [441,458] -> unknown_attr_read (function)
self @ [615,619] -> self (parameter)
Child @ [722,727] -> Child (class)
b @ [834,835] -> b (variable)
base_method @ [72,83] -> base_method (function)
top_level @ [701,710] -> top_level (function)
c @ [718,719] -> c (variable)
a @ [734,735] -> a (variable)
a @ [943,944] -> a (variable)

=== ATTRIBUTE BINDINGS ===
BOUND   attr base_only -> base_only (unknown) at [49,58]
BOUND   attr base_only -> base_only (unknown) at [111,120]
BOUND   attr x -> x (unknown) at [179,180]
BOUND   attr y -> y (unknown) at [199,200]
BOUND   attr x -> x (unknown) at [219,220]
BOUND   attr x -> x (unknown) at [327,328]
BOUND   attr y -> y (unknown) at [336,337]
BOUND   attr x -> x (unknown) at [377,378]
UNBOUND attr nope at [486,490]
BOUND   attr new_attr -> new_attr (unknown) at [620,628]
UNBOUND attr x at [740,741]
UNBOUND attr sum at [840,843]

=== ATTRIBUTES DISCOVERED (INSTANCE) ===
Class Base
  attr base_only
Class Child
  attr x
  attr y
  attr new_attr

=== PROMOTED CLASS MEMBERS ===
Class Base
  member __init__ : function
  member base_method : function
  member base_only : unknown
Class Child
  member unknown_attr_write : function
  member __init__ : function
  member sum : function
  member touch : function
  member unknown_attr_read : function
  member x : unknown
  member y : unknown
  member new_attr : unknown

```



## License

MIT


## Author

Akash Sivanandan
