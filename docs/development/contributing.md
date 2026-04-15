# Contributing to Rahu

Thank you for your interest in contributing to rahu.

## Getting Started

### Prerequisites

- Go 1.26 or later
- Python 3.10-3.14 (for testing)
- Git

### Setting Up Development Environment

```bash
# Clone the repository
git clone https://github.com/yourusername/rahu.git
cd rahu

# Build
go build ./...

# Run tests
go test ./...

# Run specific package tests
go test ./lexer
go test ./parser
go test ./analyser
go test ./server
```

## Development Workflow

### Making Changes

1. **Create a branch**:
   ```bash
   git checkout -b feature/my-feature
   ```

2. **Make your changes** with appropriate test coverage

3. **Run tests**:
   ```bash
   go test ./...
   ```

4. **Commit** with clear messages:
   ```bash
   git commit -m "feat(parser): add support for XYZ syntax"
   ```

5. **Push and create a pull request**

### Commit Message Format

We follow conventional commits:

- `feat(parser): add lambda expression support`
- `fix(resolver): handle circular imports correctly`
- `docs(readme): update installation instructions`
- `test(analyser): add coverage for class inheritance`
- `refactor(server): simplify indexing logic`

## Areas to Contribute

### Parser

Missing Python features to implement:
- Lambda expressions
- Walrus operator (`:=`)
- Async/await syntax
- Match statements

Files: `parser/*.go`

### Semantic Analysis

Improvements needed:
- Better type inference
- Generic type support
- Type narrowing

Files: `analyser/*.go`

### Testing

- Add edge case tests
- Property-based testing
- Fuzzing

Files: `*_test.go`

### Documentation

- Editor setup guides
- Troubleshooting scenarios
- Architecture deep-dives

Files: `docs/**/*.md`

### Performance

- Profile hot paths
- Reduce allocations
- Optimize indexing

## Code Guidelines

### Go Code

- Follow standard Go conventions
- Use `gofmt` for formatting
- Add tests for new functionality
- Keep functions focused and small
- Document public APIs

### Error Handling

- Use explicit error returns
- Provide helpful error messages
- Include position information where possible

### Testing

- Write unit tests for all new features
- Include edge cases
- Test error paths
- Use table-driven tests where appropriate

Example:
```go
func TestMyFeature(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"simple case", "x = 1", "int"},
        {"complex case", "x = [1, 2]", "list[int]"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := analyze(tt.input)
            if result != tt.expected {
                t.Errorf("got %q, want %q", result, tt.expected)
            }
        })
    }
}
```

## Architecture

Understanding the codebase:

1. **Lexer** (`lexer/`) - Tokenizes Python source
2. **Parser** (`parser/`) - Builds AST from tokens
3. **Analyser** (`analyser/`) - Semantic analysis (scopes, types)
4. **Server** (`server/`) - LSP implementation

See [Architecture Documentation](../architecture/) for details.

## Debugging

### Using the Dump Tool

```bash
# Analyze a specific file
cp myfile.py temp.py
go run ./utils/dump
```

### Running Specific Tests

```bash
# Run one test
go test ./analyser -run TestSpecificFeature -v

# Run with race detector
go test ./... -race

# Run benchmarks
go test ./server -bench=. -benchmem
```

### Logging

Rahu uses standard Go logging. Set your LSP client to capture logs:

- VS Code: Output panel
- Neovim: `:LspLog`
- Emacs: `*lsp-log*` buffer

## Submitting Changes

### Pull Request Process

1. Ensure tests pass
2. Update documentation if needed
3. Add changelog entry if applicable
4. Request review from maintainers

### What We Look For

- Correctness
- Test coverage
- Performance impact
- Code clarity
- Documentation

## Questions?

- Check existing issues and PRs
- Open a discussion for design questions
- Ask in development channels

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
