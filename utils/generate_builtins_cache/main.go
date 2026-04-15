// generate_builtins_cache parses typeshed builtins.pyi and generates JSON cache files.
// Uses simple line-based parsing to extract class/function definitions.
// Usage: go run ./utils/generate_builtins_cache
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// BuiltinSymbol represents a single builtin symbol
type BuiltinSymbol struct {
	Name  string   `json:"name"`
	Kind  string   `json:"kind"`            // "class", "constant", "function"
	Bases []string `json:"bases,omitempty"` // for classes, single inheritance only
}

// BuiltinCache represents the complete cache for a Python version
type BuiltinCache struct {
	PythonVersion   string          `json:"python_version"`
	TypeshedVersion string          `json:"typeshed_version"`
	SourceHash      string          `json:"source_hash"`
	GeneratedAt     string          `json:"generated_at"`
	Symbols         []BuiltinSymbol `json:"symbols"`
}

var (
	// class Name(Base): ... or class Name: ... or class Name(Base[T]): ...
	// Captures class name and base name (without generics)
	classRegex = regexp.MustCompile(`^class\s+(\w+)\s*(?:\(\s*([\w\[][^)]*))?`)

	// def name(...): ... or def name(...): ...
	funcRegex = regexp.MustCompile(`^def\s+(\w+)\s*\(`)

	// Name = ... (for constants/type aliases)
	constRegex = regexp.MustCompile(`^(\w+)\s*=`)
)

func main() {
	typeshedDir := "third_party/typeshed/stdlib"
	builtinsPath := filepath.Join(typeshedDir, "builtins.pyi")

	// Read and hash the source file
	sourceData, err := os.ReadFile(builtinsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading builtins.pyi: %v\n", err)
		os.Exit(1)
	}

	sourceHash := sha256.Sum256(sourceData)
	hashStr := hex.EncodeToString(sourceHash[:])

	// Get typeshed version
	typeshedVersion := getTypeshedVersion()

	// Parse builtins.pyi using line-based parsing
	fmt.Printf("Parsing %s...\n", builtinsPath)
	symbols := parseBuiltinsPyi(string(sourceData))
	fmt.Printf("Extracted %d symbols\n", len(symbols))

	// Generate caches for each Python version
	pythonVersions := []string{"3.10", "3.11", "3.12", "3.13", "3.14"}
	for _, pyVer := range pythonVersions {
		cache := BuiltinCache{
			PythonVersion:   pyVer,
			TypeshedVersion: typeshedVersion,
			SourceHash:      hashStr,
			GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
			Symbols:         filterSymbolsForVersion(symbols, pyVer),
		}

		filename := filepath.Join("builtin_cache", fmt.Sprintf("builtins-%s.json", pyVer))
		data, err := json.MarshalIndent(cache, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling cache for %s: %v\n", pyVer, err)
			os.Exit(1)
		}

		if err := os.WriteFile(filename, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing cache file %s: %v\n", filename, err)
			os.Exit(1)
		}

		fmt.Printf("Generated %s (%d symbols)\n", filename, len(cache.Symbols))
	}
}

func getTypeshedVersion() string {
	// Try to get from git
	if data, err := os.ReadFile("third_party/typeshed/.git/HEAD"); err == nil {
		return strings.TrimSpace(string(data))
	}
	return "unknown"
}

func parseBuiltinsPyi(content string) []BuiltinSymbol {
	var symbols []BuiltinSymbol
	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments, imports, empty lines
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "import") ||
			strings.HasPrefix(line, "from") || strings.HasPrefix(line, "if sys.version") ||
			strings.HasPrefix(line, "if typing") || strings.HasPrefix(line, "_") ||
			strings.HasPrefix(line, "@") || strings.HasPrefix(line, "TypeVar") ||
			strings.HasPrefix(line, "ParamSpec") || strings.HasPrefix(line, "Protocol") ||
			strings.HasPrefix(line, "Generic") || strings.HasPrefix(line, "Literal") ||
			strings.HasPrefix(line, "Final") || strings.HasPrefix(line, "overload") ||
			strings.HasPrefix(line, "deprecated") || strings.HasPrefix(line, "Concatenate") ||
			strings.HasPrefix(line, "Self") || strings.HasPrefix(line, "TypeAlias") ||
			strings.HasPrefix(line, "TypeGuard") || strings.HasPrefix(line, "TypeIs") ||
			strings.HasPrefix(line, "disjoint_base") {
			continue
		}

		// Try to match class definition
		if matches := classRegex.FindStringSubmatch(line); matches != nil {
			name := matches[1]
			bases := []string{}
			if len(matches) > 2 && matches[2] != "" {
				// Extract base name without generics (e.g., Sequence[str] -> Sequence)
				base := matches[2]
				if idx := strings.Index(base, "["); idx != -1 {
					base = base[:idx]
				}
				bases = append(bases, base)
			}

			// Builtin types (int, str, bool, etc.) should be marked as "type"
			// Exception classes and other classes should be "class"
			builtinTypes := map[string]bool{
				"bool": true, "int": true, "str": true, "float": true,
				"list": true, "tuple": true, "dict": true, "set": true,
				"frozenset": true, "bytes": true, "bytearray": true,
				"complex": true, "object": true, "type": true,
			}

			kind := "class"
			if builtinTypes[name] {
				kind = "type"
			}

			symbols = append(symbols, BuiltinSymbol{
				Name:  name,
				Kind:  kind,
				Bases: bases,
			})
			continue
		}

		// Try to match function definition
		if matches := funcRegex.FindStringSubmatch(line); matches != nil {
			name := matches[1]

			// Skip dunder functions (usually special methods)
			if strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__") {
				continue
			}

			symbols = append(symbols, BuiltinSymbol{
				Name: name,
				Kind: "function",
			})
			continue
		}

		// Try to match constants (True, False, None, etc.)
		if matches := constRegex.FindStringSubmatch(line); matches != nil {
			name := matches[1]

			// Only include known constants
			knownConstants := map[string]bool{
				"True": true, "False": true, "None": true,
				"__debug__": true, "NotImplemented": true, "Ellipsis": true,
			}

			if knownConstants[name] {
				symbols = append(symbols, BuiltinSymbol{
					Name: name,
					Kind: "constant",
				})
			}
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning file: %v\n", err)
	}

	return symbols
}

func filterSymbolsForVersion(symbols []BuiltinSymbol, pyVer string) []BuiltinSymbol {
	// For now, return all symbols (version-specific filtering can be added later)
	return symbols
}
