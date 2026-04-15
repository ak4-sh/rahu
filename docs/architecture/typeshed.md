# Typeshed Integration

How rahu uses typeshed for accurate type information.

## What is Typeshed?

Typeshed is a collection of type stubs for the Python standard library and popular third-party packages. It's maintained by the Python community and used by type checkers like mypy, pyright, and rahu.

## Why Embed Typeshed?

Three approaches were considered:

1. **Use system typeshed**
   - Requires users to install typeshed
   - Version mismatches possible
   - Extra setup step

2. **Download on demand**
   - Requires network
   - First startup slow
   - Cache management complexity

3. **Embed at build time** (chosen)
   - Zero user setup
   - Always available offline
   - Version controlled
   - Fast startup

## Architecture

### Build-Time Integration

Typeshed is added as a git submodule:

```bash
third_party/typeshed/
├── stdlib/          # Python standard library stubs
│   ├── builtins.pyi
│   ├── os/
│   ├── sys.pyi
│   └── ...
└── stubs/           # Third-party stubs
    ├── requests/
    ├── urllib3/
    └── ...
```

### Version Filtering

Different Python versions have different stdlib modules:

```python
# stdlib/VERSIONS file format:
# module: min_version[-max_version]
asyncio: 3.4-
dataclasses: 3.7-
contextvars: 3.7-
importlib.metadata: 3.8-
tomllib: 3.11-   # Only in 3.11+
```

Rahu parses this file and filters modules based on detected Python version.

### Resolution Order

When resolving an import, rahu checks in order:

1. **Workspace stubs** (`.pyi` files in project)
2. **Embedded typeshed** (stdlib and third-party)
3. **Python introspection** (runtime inspection)

Example:
```python
import urllib3.util.parse_url
```

Resolution:
1. Not in workspace
2. Found in `stubs/urllib3/urllib3/util/__init__.pyi`
3. Type information extracted from stub

## Stub Parsing

Typeshed stubs are `.pyi` files (Python interface files):

```python
# urllib3/util/url.pyi
class Url:
    host: str | None
    port: int | None
    scheme: str | None
    
def parse_url(url: str) -> Url: ...
```

Rahu parses these using the same parser as regular Python, but with relaxed rules (no function bodies needed).

## Builtin Cache System

### Problem

The `builtins` module has 499+ symbols (int, str, Exception, etc.).

Parsing `builtins.pyi` on every startup would be slow.

### Solution: Pre-Generated Caches

1. Parse `builtins.pyi` once
2. Extract all class/function/constant definitions
3. Generate JSON cache files
4. Embed in binary using `go:embed`

Cache structure:
```json
{
  "python_version": "3.11",
  "typeshed_version": "2024.1.5",
  "source_hash": "sha256:abc123...",
  "symbols": [
    {"name": "str", "kind": "class", "bases": ["object"]},
    {"name": "DeprecationWarning", "kind": "class", "bases": ["Warning"]},
    ...
  ]
}
```

### Cache Verification

At startup, rahu verifies the cache:

```go
func verifyCache(cache *BuiltinCache) bool {
    // Compute hash of current builtins.pyi
    currentHash := sha256(builtins.pyi content)
    
    // Compare with cached hash
    return cache.SourceHash == currentHash
}
```

If stale:
- Log warning
- Use fallback hardcoded symbols
- Still functional, just less complete

### Regeneration

Update caches after typeshed update:

```bash
# 1. Update typeshed
cd third_party/typeshed
git pull
cd ../..

# 2. Regenerate caches
go run ./utils/generate_builtins_cache

# 3. Rebuild rahu
go build ./...
```

Caches are versioned per Python minor version (3.10, 3.11, etc.).

## Supported Modules

### Standard Library

All stdlib modules in typeshed are supported:
- `os`, `sys`, `json`, `collections`
- `typing`, `typing_extensions`
- `pathlib`, `subprocess`, `socket`
- etc.

### Third-Party Packages

Common packages with stubs:
- `requests`
- `urllib3`
- `numpy` (partial)
- `pandas` (partial)

Packages without stubs fall back to Python introspection.

## Limitations

### Not All Packages Have Stubs

If typeshed doesn't have stubs for a package:
- Rahu falls back to Python introspection
- Type information may be incomplete
- Consider contributing stubs to typeshed

### Version Drift

Embedded typeshed may lag behind:
- New Python features not yet in typeshed
- Package API changes not reflected

Regenerate caches regularly to minimize drift.

### Complex Generics

Typeshed uses complex generic types:
```python
def map(func: Callable[[T], S], iter: Iterable[T]) -> Iterator[S]: ...
```

Rahu's type inference is lightweight and may not fully resolve complex generics.

## Performance

### Startup Time

Typeshed integration adds minimal overhead:

| Component | Time |
|-----------|------|
| Loading builtin cache | < 10ms |
| Resolving stdlib module | ~5-20ms per module |
| Total impact | Negligible |

### Memory

Embedded stubs add to binary size:

| File | Size |
|------|------|
| Builtin caches (5 versions) | ~500KB |
| Typeshed stubs (compressed) | ~2MB |
| Total binary increase | ~2.5MB |

Acceptable for zero-setup experience.

## Troubleshooting

### "[typeshed] Module not found"

Stub doesn't exist for that module:
- Check if it's a stdlib module
- Check if it's a known third-party package
- May need to fall back to introspection

### "[builtins] Cache hash mismatch"

Typeshed was updated but caches not regenerated:
```bash
go run ./utils/generate_builtins_cache
go build ./...
```

Or ignore - rahu falls back to hardcoded builtins.

### Missing New Python Features

Typeshed may lag behind Python releases:
- Check typeshed version
- Consider updating submodule
- Wait for typeshed community updates

## Contributing to Typeshed

If you find missing or incorrect stubs:

1. Check [typeshed repository](https://github.com/python/typeshed)
2. Follow their contribution guidelines
3. Submit PR with improvements
4. Regenerate rahu caches after merge

This benefits the entire Python typing ecosystem.
