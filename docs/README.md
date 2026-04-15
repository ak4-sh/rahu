# Rahu Documentation

Welcome to the rahu documentation! Rahu is a fast Python language server implemented in Go.

## Quick Links

- **[Getting Started](getting-started/)** - Installation, setup, and first steps
- **[User Guide](user-guide/)** - Configuration, features, and troubleshooting  
- **[Architecture](architecture/)** - How rahu works internally
- **[Development](development/)** - Contributing guide and internals
- **[Roadmap](../ROADMAP.md)** - What's planned and missing

## What is Rahu?

Rahu provides IDE features for Python through the [Language Server Protocol (LSP)](https://microsoft.github.io/language-server-protocol/):

- 🏃 **Fast** - Arena-backed AST, LRU caching, parallel indexing
- 🔍 **Accurate** - Typeshed integration for precise type information
- 🐍 **Pythonic** - Supports Python 3.10 through 3.14
- 🔧 **Featured** - Go-to-definition, hover, completion, rename, and more

## Architecture Overview

```
Python Source
    ↓
Lexer → Tokens
    ↓
Parser → AST (arena-backed)
    ↓
Scope Builder + Resolver → Symbol table
    ↓
Type Inference + Binding → Typed AST
    ↓
LSP Features (hover, completion, etc.)
```

Learn more in the [Architecture Guide](architecture/overview.md).

## Getting Help

- Check the [Troubleshooting Guide](user-guide/troubleshooting.md)
- See [Frequently Asked Questions](user-guide/faq.md)
- Review [Configuration Options](user-guide/configuration.md)

## Contributing

We welcome contributions! See the [Contributing Guide](development/contributing.md) to get started.

## License

Rahu is released under the [MIT License](../LICENSE).
