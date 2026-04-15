# Frequently Asked Questions

## General Questions

### What does "rahu" mean?

Rahu is named after a [figure in Hindu astrology](https://en.wikipedia.org/wiki/Rahu) - a celestial body that causes eclipses. It's a nod to Python's name (Monty Python) while being unique and memorable.

### How is rahu different from Pyright/Pylance?

**Rahu**:
- Implemented in Go (fast, single binary)
- No Node.js dependency
- Typeshed integration via embedded stubs
- Arena-backed AST for memory efficiency

**Pyright/Pylance**:
- Implemented in TypeScript
- Requires Node.js
- Microsoft's type checker with strictness options
- Part of VS Code ecosystem

### How is rahu different from Jedi?

**Rahu**:
- Static analysis with typeshed
- LSP-native from ground up
- Go implementation

**Jedi**:
- Dynamic analysis via Python introspection
- Can infer runtime types
- Python implementation
- Older, more established

### What Python versions are supported?

Python 3.10, 3.11, 3.12, 3.13, and 3.14.

Features are extracted from typeshed stubs versioned for each Python release.

## Features

### Does rahu support type checking?

**Partially**. Rahu does lightweight type inference for:
- Hover information
- Completion filtering
- Signature help

But it doesn't enforce type correctness like mypy or pyright. It shows types when they can be inferred, but won't error on type mismatches (unless they're obvious like undefined names).

### Does rahu support docstrings?

**Not yet**. Docstring extraction is on the roadmap. Currently hover shows only type signatures.

### Can rahu format my code?

**No**. Rahu doesn't implement formatting. Use Black, Ruff, or autopep8 for formatting.

### Does rahu support Jupyter notebooks?

**No**. Rahu works with `.py` files only. Jupyter support would require significant additional work.

### Can rahu find all references across the entire project?

**Yes**, but with caveats:
- ✅ Finds references within workspace
- ✅ Finds references in indexed external modules
- ⚠️ May miss references in unimported modules

The reference index is built during workspace indexing.

## Configuration

### Can I configure rahu behavior?

**Limited**. Rahu currently has minimal configuration:
- No settings file
- Configuration via LSP initialization options (if your client supports it)

Most behavior is automatic based on workspace structure.

### Can I exclude directories from indexing?

**Yes**, rahu automatically excludes common directories:
- `.git`, `node_modules`, `vendor`
- `.venv`, `venv`, `dist`, `build`
- `.cache`, `coverage`, `target`

**Custom exclusion** is not yet implemented but would be useful.

### Can I use rahu with multiple Python versions?

**Yes**, but rahu uses the first Python it finds in PATH. To switch:

```bash
# Use Python 3.11
export PATH="/usr/local/opt/python@3.11/bin:$PATH"
rahu

# Use Python 3.12  
export PATH="/usr/local/opt/python@3.12/bin:$PATH"
rahu
```

## Performance

### Why is rahu faster than other language servers?

Three main reasons:

1. **Go implementation** - Compiled, not interpreted (TypeScript/Python)
2. **Arena-backed AST** - Single allocation, cache-friendly
3. **Lazy indexing** - Only analyzes what's needed

### How much memory does rahu use?

**Typical**: 50-200MB depending on workspace size.

Rahu uses bounded caches:
- Max 256 module snapshots
- LRU eviction under memory pressure

### Can I use rahu on very large codebases?

**Yes**, but expect:
- Longer initial indexing (minutes for 10k+ files)
- Higher memory usage
- Some features may be slower

The LRU cache helps, but very large projects will hit cache pressure more often.

## Development

### Can I use rahu as a library?

**Not officially**. The Go packages (parser, analyser) are internal. No stable public API is provided yet.

If you want to build tools on top of rahu's parser, you'd need to import the packages directly, but APIs may change.

### Can I contribute to rahu?

**Yes!** See the [Contributing Guide](../development/contributing.md).

Good first issues:
- Parser support for missing Python features
- Test coverage improvements
- Documentation

### How do I debug rahu?

**Server logs**: Most LSP clients have a way to view server logs:
- VS Code: Output panel → Python Language Server
- Neovim: `:LspLog`
- Emacs: `*lsp-log*` buffer

**Dump tool**: For analyzing specific files:
```bash
cp myfile.py temp.py
go run ./utils/dump
```

**Tests**: Run specific test:
```bash
go test ./server -run TestFeatureName -v
```

### What's the development roadmap?

See [ROADMAP.md](../../ROADMAP.md) for:
- What's missing (lambda, walrus, async, match)
- Planned features
- Known limitations

## Comparison with Other Tools

### Should I use rahu or Pylance?

**Use Pylance if**:
- You're already in VS Code
- You want full type checking with strictness levels
- You need the most mature, feature-complete solution

**Use rahu if**:
- You want a fast, lightweight LSP server
- You prefer Go-based tooling
- You're using a non-VS Code editor
- You want to contribute to an open alternative

### Can I use rahu alongside other Python tools?

**Yes**:
- Use rahu for IDE features (completion, navigation)
- Use mypy/pyright for type checking
- Use Black/Ruff for formatting
- Use pylint/ruff for linting

Rahu focuses on IDE features, not linting or strict type enforcement.

## Troubleshooting

### Why isn't feature X working?

Check:
1. Is it in the ["What's Missing"](../../ROADMAP.md) list?
2. Is your file being analyzed? (Check LSP logs)
3. Is the feature enabled in your editor?

### How do I report a bug?

File an issue with:
- Minimal reproduction case
- Python version
- Rahu version (git commit hash)
- Expected vs actual behavior
- Editor and LSP client being used
