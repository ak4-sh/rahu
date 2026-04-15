# Parser Design

How rahu builds an AST from tokens.

## Overview

Rahu uses a hybrid parsing approach:
- **Recursive descent** for statements
- **Pratt parsing** (top-down operator precedence) for expressions

The output is an arena-backed AST with stable node IDs.

## Arena Allocation

Traditional ASTs use pointers and scattered heap allocations. Rahu uses an arena:

```go
type AST struct {
    Nodes   []Node     // All nodes in contiguous slice
    Names   []string   // Interned identifiers
    Strings []string   // String literals
    Numbers []float64  // Numeric literals
    Root    NodeID     // Index of root node
}

type NodeID uint32  // Index into Nodes slice
```

Benefits:
- Fewer allocations
- Better cache locality
- Stable references (NodeID never changes)
- Fast traversal

## AST Structure

Nodes are stored in the arena with linked children:

```go
type Node struct {
    Kind NodeKind      // What type of node
    Start uint32       // Byte position in source
    End uint32         // Byte position in source
    Data uint32        // Node-specific data
    Child NodeID       // First child (0 = none)
    Sibling NodeID     // Next sibling (0 = none)
}
```

Children are stored as a linked list via Sibling chain.

## Parsing Statements

Recursive descent handles Python's statement syntax:

```python
def parseStatement() NodeID {
    switch currentToken {
    case IF:
        return parseIfStatement()
    case FOR:
        return parseForStatement()
    case DEF:
        return parseFunctionDef()
    case CLASS:
        return parseClassDef()
    // ... etc
    }
}
```

Each statement type has its own parsing function that:
1. Consumes expected tokens
2. Creates AST nodes
3. Handles errors gracefully

## Parsing Expressions (Pratt)

Expressions use Pratt parsing for operator precedence:

```go
func parseExpression(minPrecedence int) NodeID {
    left := parsePrimary()
    
    for precedence(currentToken) > minPrecedence {
        op := currentToken
        advance()
        right := parseExpression(precedence(op))
        left = createBinaryOpNode(left, op, right)
    }
    
    return left
}
```

Precedence levels (low to high):
1. `if-else` (conditional expressions)
2. `or`
3. `and`  
4. Comparisons (`<`, `>`, `==`, etc.)
5. `|`
6. `+`, `-`
7. `*`, `/`, `%`, `//`
8. Unary `+`, `-`, `~`
9. `**` (right associative)
10. Subscripts, calls, attributes

## Grammar Coverage

Implemented:
- All basic statements (assignment, if, for, while, etc.)
- Function and class definitions
- Import statements (absolute, relative, star)
- Exception handling (try/except/finally)
- Decorators
- Context managers (with)
- Generators (yield, yield from)
- Comprehensions (list, dict, set)
- Generator expressions
- Conditional expressions
- Type annotations

Not yet implemented (see Roadmap):
- Lambda expressions
- Walrus operator
- Async/await
- Match statements

## Error Recovery

The parser uses best-effort error recovery:
- Reports errors but continues parsing
- Synchronizes at statement boundaries
- Attempts to skip to recovery point

This provides better UX than stopping at first error.

## F-String Parsing

F-strings are parsed in two stages:

1. **Lexer** identifies f-string with prefix `f"` or `F"`
2. **Parser** extracts embedded expressions from f-string token
3. Sub-parser parses each expression separately
4. Result is AST node with text parts and expression nodes

## Position Tracking

Every AST node has exact source positions:
- Byte offsets (not line/column)
- Half-open ranges `[start, end)`
- Used for error reporting and LSP features

Line/column conversion uses `LineIndex` at LSP boundary.

## Performance

Typical parsing speeds:
- 5,000-10,000 lines/second
- Linear time complexity
- Memory proportional to AST size

## Testing

Parser tests cover:
- Valid syntax cases
- Edge cases
- Error conditions
- AST structure verification

Example test:
```go
func TestParseFunctionDef(t *testing.T) {
    src := "def foo(x: int) -> str:\n    return str(x)\n"
    p := parser.New(src)
    tree := p.Parse()
    
    // Verify no errors
    if len(p.Errors()) > 0 {
        t.Fatal(p.Errors())
    }
    
    // Verify AST structure
    // ... check function name, parameters, body
}
```
