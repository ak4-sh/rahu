# Configuration

Rahu configuration options and customization.

## Overview

Rahu uses a minimal configuration approach. Most behavior is automatic based on workspace structure and the detected Python environment.

## Python Version Selection

Rahu automatically detects the Python interpreter from your PATH. To use a specific version:

```bash
# Ensure desired Python is first in PATH
export PATH="/usr/local/opt/python@3.11/bin:$PATH"

# Then run rahu
rahu
```

## LSP Initialization Options

When starting the LSP server, your client can pass initialization options:

```json
{
  "python": {
    "pythonPath": "/path/to/python"
  }
}
```

## File Exclusions

Rahu automatically excludes these directories from indexing:

- `.git`, `node_modules`, `vendor`
- `.venv`, `venv`, `env`
- `dist`, `build`, `target`
- `.next`, `.turbo`, `.cache`, `coverage`
- `__pycache__`, `.pytest_cache`, `.mypy_cache`

Custom exclusions are not currently supported but may be added in the future.

## Cache Settings

Rahu uses hardcoded cache limits:

- **Max cached modules**: 256
- **LRU eviction**: Automatic when cache is full
- **Builtin cache**: Loaded from embedded JSON files, verified by SHA256 hash

Cache regeneration:

```bash
# Regenerate builtin caches after typeshed update
go run ./utils/generate_builtins_cache
```

## Logging and Debugging

To enable verbose logging, check your LSP client's documentation. Most clients have a setting like:

**VS Code**:
```json
{
  "python.trace.server": "verbose"
}
```

**Neovim**:
```lua
vim.lsp.set_log_level("debug")
```

**Emacs**:
```elisp
(setq lsp-log-io t)
```

## Environment Variables

Rahu respects these environment variables:

- `PATH` - Used to find Python interpreter and rahu binary
- `PYTHONPATH` - Not directly used by rahu (uses interpreter's sys.path)

## Workspace Configuration

Rahu does not currently support:
- `pyproject.toml` configuration
- `.rahu` configuration files
- Per-workspace settings files

Configuration must be passed through LSP initialization options or defaults are used.

## Future Configuration Options

Planned configuration (see [Roadmap](../../ROADMAP.md)):
- Custom file exclusion patterns
- Cache size limits
- Analysis depth control
- Type checking strictness levels
