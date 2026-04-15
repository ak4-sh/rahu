# Semantic Analysis

How rahu understands Python code meaning.

## Overview

Semantic analysis has three main phases:

1. **Scope Building** - Create lexical scopes and record definitions
2. **Name Resolution** - Connect name references to definitions (LEGB)
3. **Type Inference** - Determine types for editor features

## Scope Building

Creates hierarchical scope structure:

```
Module Scope
├── Function Scope (foo)
│   ├── Local variables
│   └── Nested Function Scope (bar)
├── Class Scope (MyClass)
│   └── Method Scope (method)
└── Global variables
```

### Scope Types

- `ScopeModule` - File-level scope
- `ScopeFunction` - Function body
- `ScopeClass` - Class body
- `ScopeBuiltin` - Always present at bottom

### Definition Recording

As the scope builder walks the AST, it records:
- Variable assignments
- Function definitions
- Class definitions
- Import aliases

Each symbol tracks:
- Name
- Kind (variable, function, class, import)
- Defining position
- Type (if annotated or inferred)

## Name Resolution (LEGB)

Python uses LEGB scoping:
- **L**ocal - Current function
- **E**nclosing - Outer functions (for closures)
- **G**lobal - Module level
- **B**uiltin - Built-in names

Resolution algorithm:
```go
func resolveName(name string, scope *Scope) *Symbol {
    // Try local scope
    if sym, ok := scope.Lookup(name); ok {
        return sym
    }
    
    // Try enclosing scopes
    for parent := scope.Parent; parent != nil; parent = parent.Parent {
        if sym, ok := parent.Lookup(name); ok {
            return sym
        }
    }
    
    // Try builtins
    return builtinScope.Lookup(name)
}
```

### Import Resolution

Imports are resolved against the workspace index:

```python
from . import sibling      # Relative import
from ..parent import thing # Parent import
import os.path             # Absolute import
from collections import defaultdict
```

Resolution steps:
1. Determine import type (absolute, relative, star)
2. Resolve against workspace modules
3. Check external module index (typeshed or Python)
4. Record imported symbols

## Type Inference

Lightweight type system for editor features.

### What We Infer

**Explicit annotations**:
```python
def greet(name: str) -> str:
    return f"Hello, {name}"

x: int = 42
```

**Constructor calls**:
```python
items = list()           # list[unknown]
counts = dict()         # dict[unknown, unknown]
```

**Container literals**:
```python
nums = [1, 2, 3]        # list[int]
pairs = {"a": 1}        # dict[str, int]
```

**Simple assignments**:
```python
s = "hello"             # str
n = 42                  # int
```

**Mutations**:
```python
items = []
items.append(1)         # items becomes list[int]
```

### What We Don't Infer

Complex expressions:
```python
result = some_function()   # Unknown without annotation
values = [f(x) for x in items]  # Element type unknown
```

### Type Model

```go
type Type struct {
    Kind TypeKind       // TypeBuiltin, TypeInstance, TypeUnion, etc.
    Symbol *Symbol      // For instances
    Elem *Type          // For containers (list[T], dict[K,V])
    Items []*Type       // For unions, tuples
}
```

Type kinds:
- `TypeUnknown` - Could not infer
- `TypeBuiltin` - int, str, float, etc.
- `TypeInstance` - Instance of a class
- `TypeClass` - Class itself (not instance)
- `TypeList`, `TypeDict`, `TypeSet`, `TypeTuple` - Container types
- `TypeUnion` - Multiple possible types

## Attribute Binding

Connects `obj.attr` to the right member:

```python
class Dog:
    def bark(self): pass

d = Dog()
d.bark()  # Bind to Dog.bark
```

Binding process:
1. Determine type of `d` (Dog instance)
2. Look up `bark` in Dog class members
3. Handle inheritance (check parent classes)
4. Record binding

## Class Analysis

Special handling for classes:

### Member Promotion

Instance attributes defined in methods are promoted:
```python
class Person:
    def __init__(self, name):
        self.name = name  # Promoted to Person member
```

### Inheritance

Rahu builds inheritance chains:
```python
class Animal: ...
class Dog(Animal): ...
class Beagle(Dog): ...
```

Lookup order for members: Beagle -> Dog -> Animal -> object

### Method Resolution

Rahu does single inheritance only (matches Python). Method resolution is linear up the chain.

## Error Detection

The analyser catches:

- **Undefined names** - Reference to unknown variable
- **Undefined attributes** - Access to non-existent member
- **Unresolved imports** - Import of unknown module
- **Circular imports** - Detected during resolution
- **Return outside function** - Syntax error
- **Break/continue outside loop** - Syntax error

Errors are reported with source positions for LSP diagnostics.

## Performance

Analysis is fast:
- Single pass for scope building
- Single pass for resolution
- Lazy type inference (only when needed for editor features)

Typical: 1,000-5,000 lines/second for complete analysis.

## Output

The analysis produces:

1. **Symbol table** - All definitions with scopes
2. **Resolved names map** - NodeID -> Symbol
3. **Attribute bindings** - Attribute node -> Member symbol
4. **Type map** - NodeID -> Type
5. **Semantic errors** - List of errors with positions

This is stored in a `ModuleSnapshot` for caching and reuse.
