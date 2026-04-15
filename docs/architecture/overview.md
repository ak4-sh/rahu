# Architecture Overview

How rahu is designed and how the pieces fit together.

## High-Level Flow

```
┌─────────────────┐
│   Python File   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐     ┌──────────────┐
│     Lexer       │────▶│    Tokens    │
└─────────────────┘     └──────────────┘
         │
         ▼
┌─────────────────┐     ┌──────────────┐
│     Parser      │────▶│  AST (Arena) │
└─────────────────┘     └──────────────┘
         │
         ▼
┌─────────────────┐     ┌──────────────┐
│  Scope Builder    │────▶│ Symbol Table │
└─────────────────┘     └──────────────┘
         │
         ▼
┌─────────────────┐     ┌──────────────┐
│   Resolver      │────▶│  Typed AST   │
│   + Binder      │     │  + Indexes   │
└─────────────────┘     └──────────────┘
         │
         ▼
┌─────────────────┐
│   LSP Features  │ (hover, completion, etc.)
└─────────────────┘
```

## Core Components

### 1. Lexer (`lexer/`)

**Purpose**: Convert Python source code into tokens.

**Key Features**:
- Handles INDENT/DEDENT tokens
- Supports f-strings with nested expressions
- Tracks byte positions for accurate error reporting
- Tab/space consistency enforcement

**Output**: Stream of tokens with position information.

### 2. Parser (`parser/`)

**Purpose**: Build an Abstract Syntax Tree (AST) from tokens.

**Design**:
- Recursive descent for statements
- Pratt parsing (top-down operator precedence) for expressions
- Arena allocation: all nodes stored in a contiguous slice
- Stable `NodeID`s (indices into the arena)
- Side tables for names, strings, numbers

**Output**: Arena-backed AST with root at `tree.Root`.

### 3. Scope Builder (`analyser/`)

**Purpose**: Build lexical scopes and record definitions.

**What it does**:
- Walks AST
- Creates scopes for functions, classes, comprehensions
- Records symbol definitions with their spans
- Handles import aliases

**Output**: Hierarchical scope tree + definition map.

### 4. Resolver (`analyser/`)

**Purpose**: Resolve names to their definitions (LEGB: Local, Enclosing, Global, Builtin).

**What it does**:
- For each name reference, find its definition
- Build resolved name map (NodeID → Symbol)
- Report undefined names
- Handle import resolution against workspace index

**Output**: Resolved name map + semantic errors.

### 5. Binder (`analyser/`)

**Purpose**: Connect attribute access to class members.

**What it does**:
- For `obj.attr`, find which class `attr` belongs to
- Handle inheritance chains
- Build attribute binding map

**Output**: Attribute binding map.

### 6. Type Inference (`analyser/`)

**Purpose**: Lightweight type inference for editor features.

**What it infers**:
- Explicit annotations (variable, parameter, return)
- Constructor calls → instance types
- Container literals (list[int], dict[str, int])
- Simple assignments from known types
- `list.append()` mutation

**Not a full type checker** - for editor use only, not enforcement.

### 7. Workspace Indexer (`server/`)

**Purpose**: Build and maintain workspace module index.

**Process**:
1. Discover Python files in workspace
2. Prune irrelevant directories (`.git`, `node_modules`, etc.)
3. Parse and analyze modules in parallel
4. Extract exports (classes, functions, variables)
5. Build dependency graph (who imports whom)
6. Build reverse dependency graph (who depends on me)

**Output**: Module snapshots + import graph.

### 8. Typeshed Integration (`server/`)

**Purpose**: Provide accurate type information for stdlib and third-party packages.

**How it works**:
1. Embed typeshed stubs in binary at build time
2. At startup, load cache appropriate for Python version
3. Parse stubs to extract class/function signatures
4. Use for stdlib modules (os, sys, json, etc.)
5. Fallback to Python introspection if stub unavailable

**Benefits**:
- No need for Python environment to have stubs installed
- Consistent types across all users
- Version-appropriate (Python 3.10 vs 3.11 differences)

### 9. Builtin Cache System (`builtin_cache/`, `server/`)

**Purpose**: Fast resolution of builtin names (str, int, Exception, etc.).

**Design**:
- Pre-generate JSON caches from typeshed builtins.pyi
- Embed caches in binary using `go:embed`
- Load at startup based on Python version
- SHA256 verification to detect stale caches
- Fallback to hardcoded minimal set

**Symbols included**: 499 per Python version (types, functions, classes, constants)

### 10. LSP Server (`server/`)

**Purpose**: Expose analysis via Language Server Protocol.

**Features**:
- TextDocumentSync (incremental)
- Go-to-definition
- Hover (type info)
- Completion
- Signature help
- Semantic tokens
- References
- Rename
- Diagnostics

**Architecture**:
- JSON-RPC transport over stdin/stdout
- Debounced re-analysis on changes
- LRU caching of module snapshots
- Parallel indexing with priority for open files

## Data Structures

### AST Arena

All AST nodes stored in a single slice:

```go
type AST struct {
    Nodes []Node       // All nodes here
    Names []string     // Interned names
    Strings []string   // String literals
    // ...
}

type NodeID uint32    // Index into Nodes slice
```

**Benefits**:
- Cache-friendly (contiguous memory)
- Stable references (NodeID doesn't change)
- Fast traversal

### Symbol Table

Hierarchical scope structure:

```
Module Scope
├── Function Scope (foo)
│   ├── Local variables
│   └── Nested Function Scope (bar)
├── Class Scope (MyClass)
│   └── Method Scope (method)
└── Global variables
```

Each scope contains:
- Symbols defined in that scope
- Parent scope reference (for LEGB lookup)
- Kind (module, function, class, builtin)

### Module Snapshot

Immutable analysis result for a module:

```go
type ModuleSnapshot struct {
    URI           DocumentURI
    Tree          *ast.AST
    GlobalScope   *Scope
    Resolutions   map[NodeID]*Symbol
    AttrBindings  map[NodeID]*Symbol
    Types         map[NodeID]*Type
    Exports       []Export
    Imports       []Import
}
```

Snapshots are cached and reused until the file changes.

## Key Design Decisions

### Why Go?

- **Performance**: Compiled, not interpreted
- **Simplicity**: Easy to understand and contribute
- **Tooling**: Great standard library (testing, json, http)
- **Deployment**: Single binary, no runtime dependencies

### Why Arena Allocation?

Traditional ASTs use pointers and separate allocations:
```go
type Node struct {
    Kind NodeKind
    Children []*Node  // Pointers to heap
}
```

Rahu uses arena:
```go
type Node struct {
    Kind NodeKind
    Child NodeID  // Index, not pointer
}

// All nodes in one slice
nodes := make([]Node, 0, estimatedCount)
```

**Benefits**:
- Fewer allocations (faster)
- Better cache locality
- No pointer chasing

### Why Typeshed Integration?

Alternatives considered:
1. **Runtime introspection only** - Slow, requires Python
2. **User-provided stubs** - Requires setup
3. **Embed typeshed** - Zero setup, always available ✅

### Why Not Full Type Checking?

Rahu does inference, not enforcement:
- ✅ Shows types when known
- ✅ Helps editor features
- ❌ Doesn't error on type mismatches

Reason: Full type checking (mypy-style) is complex and orthogonal to IDE features. Rahu focuses on fast, helpful IDE support.

## Performance Characteristics

### Startup

- **Small workspace** (< 100 files): ~1-2 seconds
- **Medium workspace** (100-1000 files): ~5-10 seconds  
- **Large workspace** (1000+ files): ~10-30 seconds

Startup is parallelized and prioritizes files near open documents.

### Re-analysis

- **Single file change**: ~10-100ms
- **Dependent modules**: Cascading updates, but debounced
- **Export signature unchanged**: Dependents skip rebuild

### Memory

- **Typical**: 50-200MB
- **Bounded**: LRU cache with max size
- **Stable**: Memory usage plateaus after startup

### CPU

- **Indexing**: Parallel goroutines (scales with cores)
- **Analysis**: Single-threaded per file
- **LSP operations**: O(1) from indexes

## Learn More

- [Lexer Design](lexer.md)
- [Parser Design](parser.md)  
- [Semantic Analysis](analyser.md)
- [Workspace Indexing](indexing.md)
- [Typeshed Integration](typeshed.md)
