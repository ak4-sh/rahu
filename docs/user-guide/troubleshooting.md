# Troubleshooting

Common issues and how to fix them.

## Table of Contents

- [Rahu Won't Start](#rahu-wont-start)
- [No LSP Features Working](#no-lsp-features-working)
- [Features Are Slow](#features-are-slow)
- [Import Errors](#import-errors)
- [Typeshed Cache Issues](#typeshed-cache-issues)
- [High Memory/CPU Usage](#high-memorycpu-usage)

## Rahu Won't Start

### Error: "command not found: rahu"

**Cause**: `rahu` is not in your PATH.

**Fix**:
```bash
# Option 1: Install to GOPATH/bin
go install ./cmd/lsp

# Option 2: Use full path
/path/to/rahu/cmd/lsp

# Option 3: Add to PATH
export PATH="$PATH:/path/to/rahu/cmd"
```

### Error: Build fails

**Cause**: Missing dependencies or wrong Go version.

**Fix**:
```bash
# Check Go version
go version  # Should be 1.26+

# Download dependencies
go mod download

# Try building
go build ./...
```

## No LSP Features Working

### Editor shows "No language server attached"

**Diagnosis**:
1. Check rahu process is running: `ps aux | grep rahu`
2. Check editor LSP logs (varies by editor)

**Common Fixes**:

**VS Code**:
- Check Output panel → Python Language Server

**Neovim**:
- Run `:LspInfo` to see attached servers
- Check `:messages` for errors

**Emacs**:
- Run `M-x lsp-doctor`
- Check `*lsp-log*` buffer

### Hover shows "Loading..." forever

**Cause**: Initial indexing is still running on large workspace.

**Fix**: Wait for indexing to complete. Check progress in status bar or LSP logs.

### Completions don't appear

**Cause 1**: File not yet analyzed.

**Fix**: Save the file or wait a moment after opening.

**Cause 2**: Trigger character not configured.

**Fix**: Trigger completion manually (Ctrl+Space) after typing `.`

## Features Are Slow

### Hover/completion takes seconds

**Cause 1**: Large workspace still indexing.

**Fix**: Wait for initial indexing. Check LSP progress notifications.

**Cause 2**: Very large file or deep import chain.

**Fix**: Rahu has LRU caching - features should speed up after initial use.

**Cause 3**: Debug mode or verbose logging.

**Fix**: Ensure you're running release build:
```bash
go build -ldflags="-s -w" ./cmd/lsp
```

### Go-to-definition is slow

**Cause**: Target module not yet indexed or in external library.

**Fix**: 
- For workspace files: They're indexed on startup
- For external libraries: Resolution is lazy (first use triggers indexing)

## Import Errors

### "Unresolved module 'xxx'"

**Cause 1**: Module not in Python path.

**Fix**: Ensure the module is installed or in your workspace:
```bash
pip install xxx
```

**Cause 2**: Wrong Python interpreter detected.

**Fix**: Check which Python rahu is using:
```bash
# Put desired Python first in PATH
export PATH="/path/to/python/bin:$PATH"
rahu
```

**Cause 3**: Virtual environment not activated.

**Fix**: Activate your venv before starting editor:
```bash
source .venv/bin/activate
# Or for Windows:
.venv\Scripts\activate
```

### "Missing imported name 'xxx' from module"

**Cause**: Module doesn't export that name (or uses dynamic exports).

**Fix**: Check the module has `__all__` defined or the name exists.

## Typeshed Cache Issues

### "[builtins] No cache found for Python X.Y"

**Cause**: Cache file missing for your Python version.

**Fix**: Rahu falls back to hardcoded builtins. If you need the cache, regenerate:
```bash
go run ./utils/generate_builtins_cache
```

### "[builtins] Cache hash mismatch"

**Cause**: Typeshed was updated but cache wasn't regenerated.

**Fix**:
```bash
# Regenerate caches
go run ./utils/generate_builtins_cache

# Rebuild rahu
go build ./...
```

This is a warning - rahu still works using fallback builtins.

### Builtin types not resolving correctly

**Symptom**: `str`, `int`, `list` show as unknown.

**Cause**: Cache loading failed or hash mismatch.

**Fix**: 
1. Check logs for `[builtins]` messages
2. Regenerate cache if needed
3. Verify Python version is 3.10-3.14

## High Memory/CPU Usage

### Memory keeps growing

**Cause**: Large workspace with many modules cached.

**Fix**: Rahu uses LRU cache with bounds. Memory should stabilize. If not:

```bash
# Check memory usage while running
# On macOS:
ps -o rss,vsz,comm -p $(pgrep rahu)

# On Linux:
cat /proc/$(pgrep rahu)/status | grep VmRSS
```

**Limits** (hardcoded in `server.go`):
- Max cached modules: 256
- Cache eviction when pressure threshold reached

### CPU at 100% on startup

**Cause**: Initial workspace indexing.

**Expected**: This is normal for large workspaces. Should complete in 10-30 seconds depending on workspace size.

**Check progress**:
- Editor status bar shows "Indexing X/Y files"
- LSP work-done progress notifications

### CPU stays high after indexing

**Cause 1**: Re-analysis triggered by file watcher events.

**Fix**: Check if something is touching files (build process, etc.)

**Cause 2**: Circular dependency causing re-analysis loop.

**Diagnosis**: Check logs for repeated "re-analyzing" messages.

**Fix**: Break the circular import.

## Getting More Help

If issues persist:

1. **Check logs** - Most editors have LSP log output
2. **Run tests** - `go test ./...` to verify installation
3. **Simplify** - Try with a single small Python file first
4. **Report** - File an issue with:
   - Python version
   - Editor and LSP client
   - Rahu version (git commit)
   - Minimal reproduction case
   - Relevant log output
