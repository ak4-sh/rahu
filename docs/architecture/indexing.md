# Workspace Indexing

How rahu builds and maintains the workspace module index.

## Overview

Rahu indexes Python files in the workspace to provide:
- Cross-file navigation (go-to-definition, find references)
- Import resolution
- Workspace-wide symbol search
- Dependency tracking

## Indexing Process

### 1. Discovery

Rahu walks the workspace directory:

```go
func discoverModules(root string) []string {
    var modules []string
    
    filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
        // Skip irrelevant directories
        if shouldSkipDir(path) {
            return filepath.SkipDir
        }
        
        // Check if Python file
        if strings.HasSuffix(path, ".py") || strings.HasSuffix(path, ".pyi") {
            modules = append(modules, path)
        }
        
        return nil
    })
    
    return modules
}
```

Directories skipped:
- `.git`, `node_modules`, `vendor`
- `.venv`, `venv`, `env`, `.env`
- `dist`, `build`, `target`, `.cache`
- `__pycache__`, `.pytest_cache`, `.mypy_cache`
- `coverage`, `.next`, `.turbo`

### 2. Parallel Analysis

Modules are analyzed in parallel goroutines:

```go
func indexModules(modules []string) {
    var wg sync.WaitGroup
    semaphore := make(chan struct{}, runtime.NumCPU())
    
    for _, mod := range modules {
        wg.Add(1)
        semaphore <- struct{}{}  // Acquire
        
        go func(path string) {
            defer wg.Done()
            defer func() { <-semaphore }()  // Release
            
            snapshot := analyzeModule(path)
            storeSnapshot(snapshot)
        }(mod)
    }
    
    wg.Wait()
}
```

Workers limited to CPU count to avoid overwhelming the system.

### 3. Priority Indexing

Files are prioritized based on:
- Open documents (highest priority)
- Files near open documents in directory tree
- Recently modified files
- Alphabetical order (for consistency)

### 4. Export Extraction

After analysis, rahu extracts public exports:

```python
# mymodule.py
def public_func(): pass        # Exported
class PublicClass: pass        # Exported
_internal = 42                 # Not exported (leading _)

__all__ = ['public_func']      # If present, limits exports
```

Export rules:
- All top-level definitions are exports by default
- Names starting with `_` are private (not exported)
- If `__all__` exists, only listed names are exported

### 5. Dependency Graph

Rahu builds import dependency graphs:

```
main.py
├── utils.py
│   └── helpers.py
└── models.py
    └── database.py
```

Two graphs are maintained:
- **Forward graph**: Module -> its imports
- **Reverse graph**: Module -> modules that import it

Used for:
- Incremental re-analysis (only update changed + dependents)
- Circular import detection

## Module Snapshots

Immutable analysis results stored for each module:

```go
type ModuleSnapshot struct {
    // Identification
    URI         DocumentURI
    Name        string
    
    // Analysis results
    Tree        *ast.AST
    GlobalScope *Scope
    Resolutions map[NodeID]*Symbol
    Types       map[NodeID]*Type
    
    // Exports and imports
    Exports     []Export
    Imports     []Import
    
    // Cache metadata
    Hash        uint64  // Content hash for change detection
    Timestamp   time.Time
}
```

## Caching

### LRU Cache

Module snapshots are cached with LRU eviction:

```go
type cache struct {
    maxSize int
    entries map[DocumentURI]*ModuleSnapshot
    lru     list.List
}
```

Limits:
- Max 256 cached modules (hardcoded)
- Eviction when limit reached
- Open documents never evicted

### Cache Invalidation

Caches are invalidated when:
- File content changes (hash mismatch)
- Dependencies change (re-analyze dependents)
- Explicit cache clear (e.g., workspace reload)

Smart invalidation:
- If exports unchanged, dependents don't need re-analysis
- Only re-analyze if public API changes

## Incremental Updates

When a file changes:

1. **Check if content actually changed** (hash comparison)
2. **Re-analyze the changed file**
3. **Compare exports** to previous version
4. **If exports changed**, re-analyze dependent modules
5. **Update dependency graphs**

This minimizes unnecessary work.

## External Modules

Not all modules are in the workspace:

### Standard Library

Rahu uses typeshed stubs for stdlib:
- Embedded in binary
- No need for Python installation to have stubs
- Version-appropriate (3.10, 3.11, etc.)

See [Typeshed Integration](typeshed.md).

### Third-Party Packages

Resolution order:
1. Check typeshed stubs (for common packages)
2. Fall back to Python introspection
3. Generate synthetic module snapshot

### Builtin Modules

Modules like `sys`, `_frozen_importlib` have no file:
- Generated from Python introspection
- Synthetic URIs: `builtin:///sys`

## Performance

### Startup Time

Typical workspace indexing times:

| Workspace Size | Time |
|---------------|------|
| Small (< 100 files) | 1-2 seconds |
| Medium (100-1000) | 5-10 seconds |
| Large (1000+) | 10-30 seconds |

Factors:
- Number of files
- File sizes
- Import complexity
- Available CPU cores

### Memory Usage

Typical memory consumption:

| Workspace Size | Memory |
|---------------|--------|
| Small | 50-100 MB |
| Medium | 100-300 MB |
| Large | 300-800 MB |

Bounded by:
- LRU cache limits
- Arena allocation efficiency
- Automatic eviction under pressure

### Re-Analysis Time

When a file changes:

| Operation | Time |
|-----------|------|
| Single file | 10-100 ms |
| With dependents | 50-500 ms |
| Large cascade | 1-2 seconds |

## Progress Reporting

During initial indexing, rahu reports progress via LSP:

```
WorkDoneProgressBegin: "Indexing Python files"
WorkDoneProgressReport: "Indexed 45/200 files"
WorkDoneProgressEnd: "Indexing complete"
```

Editors show this in status bar.

## Troubleshooting

### Indexing Too Slow

- Check if large directories are being excluded
- Verify CPU utilization (should use all cores)
- Consider if workspace is too large for rahu

### High Memory Usage

- LRU cache evicts automatically
- Close unused files to free snapshots
- Restart server if memory leaks suspected

### Missing Modules

- Check import paths are correct
- Verify Python environment is detected
- Check for circular imports blocking resolution
