# Lexer Design

How rahu's Python tokenizer works.

## Overview

The lexer converts Python source code into a stream of tokens. It handles:

- Operators and punctuation
- Identifiers and keywords
- String literals (including f-strings)
- Numeric literals
- Indentation (INDENT/DEDENT tokens)
- Comments

## Key Features

### Indentation Handling

Python uses significant whitespace. The lexer tracks indentation levels:

```python
if True:
    x = 1  # INDENT token before 'x'
    y = 2
z = 3      # DEDENT token before 'z'
```

Implementation details:
- Tracks tab/space consistency per file
- Maintains stack of indentation levels
- Generates INDENT/DEDENT tokens
- Reports mixed tabs and spaces as errors

### F-String Support

F-strings require special handling for nested braces:

```python
f"Hello {name.upper()}!"
```

The lexer:
1. Identifies f-string prefix
2. Scans text content
3. Detects expression start `{`
4. Handles nested braces
5. Properly pairs braces for expressions

### Position Tracking

All tokens include byte positions (not line/column):

```go
type Token struct {
    Type TokenType
    Literal string
    Start uint32  // Byte offset
    End uint32    // Byte offset
}
```

Line/column conversion happens at the LSP boundary using `LineIndex`.

## Token Types

### Single-Character Tokens

```
+  -  *  /  %  //  **  <<  >>  &  |  ^  ~  <  >  <=  >=  ==  !=
(  )  [  ]  {  }  :  ;  ,  .  @  =  ->  +=  -=  *=  ...
```

### Multi-Character Tokens

- `==` `!=` `<=` `>=` (comparisons)
- `**` `//` `<<` `>>` `+=` `-=` etc. (augmented assignment)
- `->` (return annotation)
- `...` (ellipsis)

### Special Tokens

- `INDENT` / `DEDENT` - Whitespace significance
- `NEWLINE` - Statement separator
- `EOF` - End of file
- `COMMENT` - `# ...` (usually skipped by parser)

### Literals

- `STRING` - Single, double, triple-quoted
- `FSTRING` - F-string with embedded expressions
- `NUMBER` - Integers, floats, hex, binary, octal
- `BYTES` - Byte strings `b"..."`

### Keywords

Reserved words: `and`, `as`, `assert`, `async`, `await`, `break`, `class`, `continue`, `def`, `del`, `elif`, `else`, `except`, `False`, `finally`, `for`, `from`, `global`, `if`, `import`, `in`, `is`, `lambda`, `None`, `nonlocal`, `not`, `or`, `pass`, `raise`, `return`, `True`, `try`, `while`, `with`, `yield`

## Implementation

The lexer is a single-pass scanner with lookahead:

```go
type Lexer struct {
    input        string
    position     uint32  // Current position
    readPosition uint32  // Next position
    ch           byte    // Current char
    atLineStart  bool    // For indent detection
    indentStack  []uint32
}
```

### Scanning Process

1. Skip whitespace (except at line start for indent)
2. Check for comments (`#`) and skip
3. Identify token type based on current char
4. Read complete token
5. Return token with position
6. Advance position

### Number Parsing

Handles multiple formats:
- Decimal: `123`, `1.5`, `1e10`
- Hex: `0xFF`
- Binary: `0b1010`
- Octal: `0o755`

### String Parsing

Complex due to f-strings and escape sequences:
- Track quote type (single/double/triple)
- Handle f-string expression braces
- Line continuation (backslash)

## Error Handling

The lexer reports errors with positions:
- Invalid characters
- Unterminated strings
- Mixed tabs and spaces
- Invalid numeric literals

## Performance

The lexer is designed for speed:
- Single pass through source
- Minimal allocations
- Direct string indexing
- No regex in hot paths

Typical performance: lexes 10,000+ lines/second.
