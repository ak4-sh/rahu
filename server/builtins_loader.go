package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"rahu"
)

// BuiltinSymbol represents a single builtin symbol from the cache
type BuiltinSymbol struct {
	Name  string   `json:"name"`
	Kind  string   `json:"kind"`
	Bases []string `json:"bases,omitempty"`
}

// BuiltinCache represents the complete cache for a Python version
type BuiltinCache struct {
	PythonVersion   string          `json:"python_version"`
	TypeshedVersion string          `json:"typeshed_version"`
	SourceHash      string          `json:"source_hash"`
	GeneratedAt     string          `json:"generated_at"`
	Symbols         []BuiltinSymbol `json:"symbols"`
}

// LoadBuiltinCache loads the appropriate cache for the Python version.
// Returns the cache and true if loaded successfully, false if not found.
func LoadBuiltinCache(pyVersion string) (*BuiltinCache, bool) {
	filename := fmt.Sprintf("builtin_cache/builtins-%s.json", pyVersion)

	// Try to read from embedded FS
	data, err := rahu.BuiltinCacheFS.ReadFile(filename)
	if err != nil {
		return nil, false
	}

	var cache BuiltinCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, false
	}

	return &cache, true
}

// VerifySourceHash checks if the cache matches the current typeshed builtins.pyi
func (c *BuiltinCache) VerifySourceHash() (bool, string) {
	typeshedDir := "third_party/typeshed/stdlib"
	builtinsPath := filepath.Join(typeshedDir, "builtins.pyi")

	// If running in development, check the file
	// In production, the embedded cache is assumed correct
	if _, err := os.Stat(builtinsPath); err != nil {
		// File doesn't exist (e.g., in production binary), assume cache is valid
		return true, ""
	}

	// Read and hash the current source file
	data, err := os.ReadFile(builtinsPath)
	if err != nil {
		return false, ""
	}

	hash := sha256.Sum256(data)
	currentHash := hex.EncodeToString(hash[:])

	return c.SourceHash == currentHash, currentHash
}

// GetSymbolNames returns all symbol names from the cache
func (c *BuiltinCache) GetSymbolNames() []string {
	names := make([]string, len(c.Symbols))
	for i, sym := range c.Symbols {
		names[i] = sym.Name
	}
	return names
}

// FindSymbol looks up a symbol by name
func (c *BuiltinCache) FindSymbol(name string) *BuiltinSymbol {
	for _, sym := range c.Symbols {
		if sym.Name == name {
			return &sym
		}
	}
	return nil
}

// CopyCacheToWriter copies the embedded cache to the writer (for debugging)
func CopyCacheToWriter(pyVersion string, w io.Writer) error {
	filename := fmt.Sprintf("builtin_cache/builtins-%s.json", pyVersion)
	data, err := rahu.BuiltinCacheFS.ReadFile(filename)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
