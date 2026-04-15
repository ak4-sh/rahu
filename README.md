# Rahu

A fast Python language server implemented in Go

Rahu provides IDE features for Python through the [Language Server Protocol](https://microsoft.github.io/language-server-protocol/).

![Demo](assets/demo.gif)

## Features

- Arena-backed AST with stable node IDs for efficient memory usage
- Parallel workspace indexing with LRU caching
- Typeshed integration for accurate Python standard library types
- Support for Python 3.10 through 3.14
- Full LSP feature set: completion, hover, go-to-definition, rename, diagnostics, semantic tokens
- Embedded typeshed stubs require no external Python dependencies

## Quick Start

```bash
# Build from source
git clone https://github.com/yourusername/rahu.git
cd rahu
go build ./...

# Run tests
go test ./...

# Start LSP server
go run ./cmd/lsp
```

Then [configure your editor](docs/getting-started/editor-setup.md) to use `rahu`.

## Documentation

- **[Getting Started](docs/getting-started/)** - Installation and setup
- **[User Guide](docs/user-guide/)** - Features, configuration, troubleshooting
- **[Architecture](docs/architecture/)** - How rahu works internally
- **[Development](docs/development/)** - Contributing guide
- **[Roadmap](ROADMAP.md)** - What's planned and known limitations

## What's Different?

**Rahu vs Pyright/Pylance**:
- Native Go binary with no Node.js dependency
- Embedded typeshed requires no setup
- Focus on IDE speed over strict type checking

**Rahu vs Jedi**:
- Static analysis using typeshed
- LSP-native design
- Parallel workspace indexing

## Current Limitations

Not yet supported:
- Lambda expressions
- Walrus operator (`:=`)
- Async/await
- Match statements
- Full type checking (we do inference, not enforcement)

See [ROADMAP.md](ROADMAP.md) for complete list.

## Contributing

We welcome contributions. See [Contributing Guide](docs/development/contributing.md).

Good first issues:
- Parser support for missing Python features
- Editor setup guides
- Test coverage improvements
- Documentation

## License

MIT License - see [LICENSE](LICENSE)
