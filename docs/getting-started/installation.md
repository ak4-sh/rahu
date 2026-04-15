# Installation

## Prerequisites

- **Go 1.26+** - Required for building from source
- **Python 3.10-3.14** - The version you want to analyze

## Building from Source

The recommended way to install rahu is to build from source:

```bash
# Clone the repository
git clone https://github.com/yourusername/rahu.git
cd rahu

# Build
go build ./...

# Run tests to verify everything works
go test ./...
```

## Installing the LSP Server

After building, you can run rahu directly:

```bash
# Run the LSP server (reads from stdin, writes to stdout)
go run ./cmd/lsp
```

For permanent use, build and install to your `$GOPATH/bin`:

```bash
go install ./cmd/lsp

# Now you can use 'lsp' command (if $GOPATH/bin is in your PATH)
lsp
```

## Verifying the Installation

Test that rahu can parse Python files:

```bash
# Create a test file
echo 'x = 1 + 2' > /tmp/test.py

# Run the dump utility (for debugging)
cp /tmp/test.py temp.py
go run ./utils/dump
```

You should see parsed output without errors.

## Python Environment Detection

Rahu automatically detects your Python environment:

1. **Python executable** - Uses the first `python3` found in PATH
2. **Module paths** - Discovers via `sys.path` from the detected interpreter
3. **Builtin modules** - Uses typeshed stubs + runtime introspection

To use a specific Python version, ensure it's first in your PATH:

```bash
# Use Python 3.11 specifically
export PATH="/usr/local/opt/python@3.11/bin:$PATH"
go run ./cmd/lsp
```

## Next Steps

- [Configure your editor](editor-setup.md)
- [Learn about features](../user-guide/features.md)
- [Check troubleshooting tips](../user-guide/troubleshooting.md) if you encounter issues
