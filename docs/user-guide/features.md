# Features

Detailed explanation of rahu's IDE features.

## Code Intelligence

### Go to Definition

Navigate from a symbol reference to its definition.

**Supported**:
- Variables and functions
- Class definitions
- Methods and attributes
- Import aliases

**Usage**: Trigger via LSP client (usually F12 or Ctrl/Cmd+Click)

**Example**:
```python
def greet(name: str) -> str:
    return f"Hello, {name}"

message = greet("World")  # Jump to line 1
```

### Hover Information

Display type signatures and documentation for symbols.

**Shows**:
- Type annotations
- Inferred types
- Parameter names

**Not yet showing**:
- Docstrings (planned)
- Full documentation

**Example**:
```python
def process(data: list[str]) -> dict[str, int]:
    return {item: len(item) for item in data}

# Hover over 'process' shows:
# function process(data: list[str]) -> dict[str, int]
```

### Completion

Context-aware code completion.

**Triggers**:
- After typing `.` (member access)
- After typing import statements
- Manually (usually Ctrl+Space)

**Provides**:
- Class and function names
- Variable names in scope
- Member functions and attributes

**Example**:
```python
items = [1, 2, 3]
items.  # Shows: append, extend, insert, etc.
```

### Signature Help

Display function signatures while typing calls.

**Shows**:
- Parameter names and types
- Default values
- Current parameter highlighting

**Example**:
```python
def configure(host: str, port: int = 8080, debug: bool = False) -> None:
    pass

configure("localhost",  # Shows signature, highlights 'port' parameter
```

## Navigation

### Document Symbols

Outline of the current file (classes, functions, variables).

**Usage**: Open document symbol outline (usually Ctrl/Cmd+Shift+O)

**Shows**:
- Classes and methods
- Functions
- Top-level variables

### Workspace Symbols

Search for symbols across the entire workspace.

**Usage**: Open workspace symbol search (usually Ctrl/Cmd+T)

**Searches**:
- Class names
- Function names
- Variable names

### Find References

Find all usages of a symbol.

**Usage**: Trigger "Find References" (usually Shift+F12)

**Scope**: Workspace-wide search through indexed modules

## Refactoring

### Rename Symbol

Rename a symbol and update all references.

**Supported**:
- Variables
- Functions
- Classes
- Method names

**Scope**: Within workspace (external references not updated)

**Usage**: Trigger rename (usually F2)

**Example**:
```python
class Person:  # Rename to 'User'
    pass

p = Person()   # Updates to 'User()'
```

### Prepare Rename

Preview what will be renamed before committing.

**Usage**: Usually triggered automatically by rename command

## Diagnostics

### Error Detection

Real-time error reporting as you type.

**Catches**:
- Syntax errors
- Undefined names
- Undefined attributes
- Unresolved imports
- Missing imported names
- Return outside function
- Break/continue outside loop

**Does NOT catch**:
- Type mismatches (only infers types, doesn't enforce)
- Style violations (use pylint/ruff)

**Example**:
```python
print(undefined_variable)  # Error: undefined name
x = [1, 2, 3]
x.nonexistent()          # Error: undefined attribute
```

## Visual Enhancements

### Semantic Tokens

Syntax highlighting based on semantic analysis.

**Highlights**:
- Keywords
- Classes (definitions and references)
- Methods
- Parameters
- Variables
- Operators

**Supported Modifiers**:
- Read-only
- Deprecated

**Usage**: Automatic when editor supports semantic highlighting

## Code Understanding

### Import Resolution

Rahu resolves imports against the workspace index:

- Absolute imports (`import os`, `from collections import defaultdict`)
- Relative imports (`from . import sibling`, `from ..parent import thing`)
- Star imports (`from module import *`) with `__all__` awareness

### Type Inference

Lightweight type inference for editor features.

**Infers**:
- Explicit annotations
- Constructor return types
- Container element types (`[1, 2, 3]` → `list[int]`)
- Simple assignments from known types
- `list.append()` mutations

**Does NOT infer**:
- Complex expressions
- Function return types without annotations
- Dynamic typing

## What's Not Included

Features intentionally not implemented:

- **Code formatting** - Use Black, ruff, or autopep8
- **Import sorting** - Use isort or ruff
- **Linting rules** - Use pylint, flake8, or ruff
- **Type checking** - Use mypy or pyright for strict enforcement
- **Debugging** - Use pdb or a debugger

See [Roadmap](../../ROADMAP.md) for planned features.
