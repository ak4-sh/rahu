# Rahu Roadmap

What we're working on and what's planned.

## Current Status

Rahu is **actively developed** and usable for daily Python development with Python 3.10-3.14.

## Completed ✅

### Language Support
- [x] Core Python syntax (assignments, control flow, functions, classes)
- [x] Import system (absolute, relative, star imports)
- [x] Exception handling (try/except/finally)
- [x] Decorators
- [x] Context managers (with statements)
- [x] Generators (yield, yield from)
- [x] Comprehensions (list, dict, set)
- [x] Generator expressions `(x for x in items)`
- [x] Conditional expressions `a if b else c`
- [x] PEP 448 extended unpacking
- [x] Type annotations
- [x] F-strings with complex expressions

### Editor Features
- [x] Go-to-definition
- [x] Hover information (type signatures)
- [x] Completion
- [x] Signature help
- [x] Document symbols
- [x] Workspace symbols
- [x] Semantic tokens
- [x] Find references
- [x] Rename symbol
- [x] Diagnostics (syntax + semantic errors)

### Infrastructure
- [x] Arena-backed AST
- [x] LRU module caching
- [x] Parallel workspace indexing
- [x] Incremental document sync
- [x] Typeshed integration (embedded stubs)
- [x] Builtin cache system (SHA256 verification)
- [x] Error recovery (best-effort, not stop-at-first-error)

## In Progress 🚧

These features are being worked on or are next in priority:

### High Priority

1. **Type stub member extraction** - Currently limited, needs more from typeshed
2. **Better completion ranking** - Context-aware suggestions
3. **String literal types** - For better dict key inference
4. **Generic type support** - list[T], dict[K, V], etc.

### Medium Priority

5. **Cross-file rename** - Currently limited to workspace
6. **Import organization** - Auto-add/remove imports
7. **Code actions** - Quick fixes for common issues
8. **Workspace configuration** - Settings file support

## Planned ⏳

### Language Features

The following Python features are **not yet implemented** but planned:

| Feature | Status | Notes |
|---------|--------|-------|
| Lambda expressions | ⏳ Planned | `lambda x: x + 1` |
| Walrus operator | ⏳ Planned | `:=` assignment expressions |
| Async/await | ⏳ Planned | `async def`, `await`, `async for`, `async with` |
| Match statements | ⏳ Planned | Python 3.10+ pattern matching |
| Bitwise operators | ⏳ Planned | `&`, `^`, `<<`, `>>`, `~` (currently only `\|` supported) |
| Positional-only parameters | ⏳ Planned | `def f(a, /, b):` |
| String escape sequences | ⏳ Planned | `\n`, `\t`, etc. in string literals |
| Complex number literals | ⏳ Planned | `3+4j` with full support |

### Typing Improvements

| Feature | Status | Notes |
|---------|--------|-------|
| Deeper return type inference | ⏳ Planned | Currently uses explicit annotations |
| More literal types | ⏳ Planned | Literal["foo"], Literal[1], etc. |
| Type narrowing | ⏳ Planned | `isinstance()` narrowing |
| Generic constraints | ⏳ Planned | `T: int` style constraints |
| Protocol support | ⏳ Planned | Structural subtyping |

### LSP Features

| Feature | Status | Notes |
|---------|--------|-------|
| Code formatting | ⏳ Planned | Would wrap Black/ruff |
| Code actions | ⏳ Planned | Quick fixes |
| Call hierarchy | ⏳ Planned | Who calls this function |
| Type hierarchy | ⏳ Planned | Class inheritance tree |
| Inlay hints | ⏳ Planned | Show types inline |
| Workspace folders | ⏳ Planned | Multi-root workspaces |

### Performance

| Feature | Status | Notes |
|---------|--------|-------|
| Incremental parsing | ⏳ Planned | Reuse unchanged AST parts |
| Persistent cache | ⏳ Planned | Save index to disk |
| Background validation | ⏳ Planned | Full-project type checking |
| Memory pressure handling | ⏳ Planned | Better OOM protection |

## Won't Do 🚫

These are intentionally out of scope:

- **Full type checker** like mypy - Rahu does inference only
- **Runtime debugging** - Use pdb or debuggers
- **Code execution** - Security risk, out of scope
- **Linting rules** - Use ruff, pylint, flake8
- **Import sorting** - Use isort, ruff
- **Complex refactorings** - Use rope, PyCharm

## Contributing Opportunities

Good first issues for new contributors:

### Parser
- Add missing Python 3.11+ syntax
- Improve error recovery for malformed input
- Better error messages

### Testing
- Add more test cases for edge cases
- Property-based testing
- Fuzzing

### Documentation
- Editor setup guides for specific editors
- Architecture deep-dives
- Troubleshooting scenarios

### Performance
- Profile and optimize hot paths
- Reduce memory allocations
- Parallelize more operations

## Version History

### v0.1.0 (Current)
- Core LSP features working
- Python 3.10-3.14 support
- Typeshed integration
- Builtin cache system
- 500+ symbols supported

## How Priorities Are Decided

1. **User Impact** - What helps the most users
2. **Language Adoption** - Python 3.11+ features as they stabilize
3. **Implementation Cost** - Quick wins vs. major refactors
4. **Maintenance Burden** - Will it be sustainable

## Feedback Welcome

Want something prioritized?

- File an issue explaining the use case
- Show how it would help your workflow
- Consider contributing it yourself!

The roadmap is a guide, not a promise. Things change based on:
- User feedback
- New Python features
- Technical discoveries
- Contributor interests
