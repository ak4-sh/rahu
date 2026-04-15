package server

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// pythonEnvInfo holds the discovered Python environment
type pythonEnvInfo struct {
	Executable string
	Paths      []string `json:"path"`
	Builtins   []string `json:"builtins"`
}

// pythonEnvCache is the on-disk cache format
type pythonEnvCache struct {
	Executable      string             `json:"executable"`
	ExecutableMtime int64              `json:"executable_mtime"`
	PythonVersion   string             `json:"python_version"`
	Paths           []string           `json:"paths"`
	Builtins        []string           `json:"builtins"`
	VenvPath        string             `json:"venv_path,omitempty"`
	SystemPath      string             `json:"system_path,omitempty"`
	CachedAt        string             `json:"cached_at"`
	Typeshed        *typeshedCacheData `json:"typeshed,omitempty"`
}

// typeshedCacheData stores the parsed typeshed VERSIONS file data
type typeshedCacheData struct {
	MaxSupported   PythonVersion           `json:"max_supported"`
	StdlibVersions map[string]VersionRange `json:"stdlib_versions"`
	SkipModules    []string                `json:"skip_modules"`
	TypeshedMtime  int64                   `json:"typeshed_mtime"` // For invalidation when typeshed source changes
}

// Cache expiration period (7 days)
const cacheExpirationDays = 7

// discoverPythonEnvCached attempts to load from cache before doing full discovery
func discoverPythonEnvCached(rootPath string, server *Server) pythonEnvInfo {
	// Skip caching if no root path (single file mode)
	if rootPath == "" {
		return discoverPythonEnv(rootPath)
	}

	// First, discover the Python executable (this is fast, just file checks)
	python := discoverPythonExecutable(rootPath)
	if python == "" {
		return pythonEnvInfo{}
	}

	// Get Python version for cache file naming
	pyVersion := getPythonVersion(python)
	if pyVersion == "" {
		// Fallback to using executable path as identifier
		pyVersion = "unknown"
	}

	// Try to load from cache
	if cache := loadPythonEnvCache(rootPath, pyVersion); cache != nil {
		if validatePythonEnvCache(cache, python, pyVersion) {
			log.Printf("[python-env] Using cached environment for %s (version: %s)", cache.Executable, cache.PythonVersion)
			return pythonEnvInfo{
				Executable: cache.Executable,
				Paths:      cache.Paths,
				Builtins:   cache.Builtins,
			}
		}
		log.Printf("[python-env] Cache invalid or expired, re-discovering...")
	}

	// Cache miss or invalid - do full discovery (slow path)
	env := discoverPythonEnvWithPaths(rootPath, python)

	// Save to cache for next time
	if env.Executable != "" {
		cache := &pythonEnvCache{
			Executable:      env.Executable,
			ExecutableMtime: getFileMtime(env.Executable),
			PythonVersion:   pyVersion,
			Paths:           env.Paths,
			Builtins:        env.Builtins,
			VenvPath:        findVenvPython(rootPath),
			SystemPath:      findSystemPython(),
			CachedAt:        time.Now().UTC().Format(time.RFC3339),
		}
		savePythonEnvCache(rootPath, cache, server)
	}

	return env
}

// discoverPythonEnvWithPaths does the full Python subprocess query
func discoverPythonEnvWithPaths(rootPath, python string) pythonEnvInfo {
	cmd := exec.Command(python, "-c", `import json, sys; print(json.dumps({"path": sys.path, "builtins": sorted(sys.builtin_module_names)}))`)
	if rootPath != "" {
		cmd.Dir = rootPath
	}
	output, err := cmd.Output()
	if err != nil {
		log.Printf("[python-env] Failed to query Python: %v", err)
		return pythonEnvInfo{}
	}

	var env pythonEnvInfo
	if err := json.Unmarshal(output, &env); err != nil {
		log.Printf("[python-env] Failed to parse Python output: %v", err)
		return pythonEnvInfo{}
	}
	env.Executable = python
	return env
}

// discoverPythonEnv is the original slow path (kept for compatibility)
func discoverPythonEnv(rootPath string) pythonEnvInfo {
	python := discoverPythonExecutable(rootPath)
	if python == "" {
		return pythonEnvInfo{}
	}
	return discoverPythonEnvWithPaths(rootPath, python)
}

// discoverPythonExecutable searches for the Python executable
func discoverPythonExecutable(rootPath string) string {
	candidates := make([]string, 0, 4)
	if rootPath != "" {
		candidates = append(candidates,
			filepath.Join(rootPath, ".venv", "bin", "python"),
			filepath.Join(rootPath, "venv", "bin", "python"),
		)
	}
	candidates = append(candidates, "python3", "python")
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if filepath.IsAbs(candidate) {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
			continue
		}
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}
	return ""
}

// findVenvPython returns the venv python path if it exists
func findVenvPython(rootPath string) string {
	if rootPath == "" {
		return ""
	}
	venvPath := filepath.Join(rootPath, ".venv", "bin", "python")
	if _, err := os.Stat(venvPath); err == nil {
		return venvPath
	}
	venvPath = filepath.Join(rootPath, "venv", "bin", "python")
	if _, err := os.Stat(venvPath); err == nil {
		return venvPath
	}
	return ""
}

// findSystemPython returns the system python3 path
func findSystemPython() string {
	if path, err := exec.LookPath("python3"); err == nil {
		return path
	}
	if path, err := exec.LookPath("python"); err == nil {
		return path
	}
	return ""
}

// getPythonEnvCachePath returns the cache file path for a given Python version
func getPythonEnvCachePath(rootPath, pyVersion string) string {
	// Sanitize version for filename (replace dots and spaces)
	safeVersion := sanitizeFilename(pyVersion)
	return filepath.Join(rootPath, ".rahu", fmt.Sprintf("python_env_%s.json", safeVersion))
}

// sanitizeFilename makes a string safe for use in filenames
func sanitizeFilename(s string) string {
	// Replace problematic characters
	result := make([]byte, 0, len(s))
	for _, c := range s {
		switch c {
		case '.', ' ', '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			result = append(result, '_')
		default:
			result = append(result, byte(c))
		}
	}
	return string(result)
}

// loadPythonEnvCache attempts to load the cache from disk
func loadPythonEnvCache(rootPath, pyVersion string) *pythonEnvCache {
	cachePath := getPythonEnvCachePath(rootPath, pyVersion)
	data, err := os.ReadFile(cachePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[python-env] Failed to read cache: %v", err)
		}
		return nil
	}

	var cache pythonEnvCache
	if err := json.Unmarshal(data, &cache); err != nil {
		log.Printf("[python-env] Failed to parse cache: %v", err)
		return nil
	}

	return &cache
}

// validatePythonEnvCache checks if the cached environment is still valid
func validatePythonEnvCache(cache *pythonEnvCache, currentPython, currentVersion string) bool {
	// Check Python version matches
	if cache.PythonVersion != currentVersion {
		log.Printf("[python-env] Cache version mismatch: cached=%s, current=%s", cache.PythonVersion, currentVersion)
		return false
	}

	// Check 7-day expiration
	if isCacheExpired(cache.CachedAt, cacheExpirationDays) {
		log.Printf("[python-env] Cache expired (older than %d days)", cacheExpirationDays)
		return false
	}

	// Check executable still exists
	if _, err := os.Stat(cache.Executable); err != nil {
		log.Printf("[python-env] Cached executable no longer exists: %s", cache.Executable)
		return false
	}

	// Check executable mtime unchanged (detects upgrades)
	currentMtime := getFileMtime(cache.Executable)
	if currentMtime != cache.ExecutableMtime {
		log.Printf("[python-env] Executable modified (mtime changed), cache invalid")
		return false
	}

	// If venv was used, verify it still exists
	if cache.VenvPath != "" {
		if _, err := os.Stat(cache.VenvPath); err != nil {
			log.Printf("[python-env] Venv no longer exists: %s", cache.VenvPath)
			return false
		}
	}

	return true
}

// isCacheExpired checks if the cache is older than specified days
func isCacheExpired(cachedAt string, days int) bool {
	t, err := time.Parse(time.RFC3339, cachedAt)
	if err != nil {
		// If we can't parse the time, consider it expired
		return true
	}
	return time.Since(t) > time.Duration(days)*24*time.Hour
}

// savePythonEnvCache saves the cache to disk with atomic write
func savePythonEnvCache(rootPath string, cache *pythonEnvCache, server *Server) bool {
	// Ensure .rahu directory exists
	rahuDir := filepath.Join(rootPath, ".rahu")
	if err := os.MkdirAll(rahuDir, 0755); err != nil {
		msg := fmt.Sprintf("Failed to create .rahu directory: %v", err)
		log.Printf("[python-env] %s", msg)
		if server != nil {
			server.showWarningMessage(msg)
		}
		return false
	}

	// Ensure .rahu/.tmp directory exists
	tmpDir := filepath.Join(rahuDir, ".tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		msg := fmt.Sprintf("Failed to create .rahu/.tmp directory: %v", err)
		log.Printf("[python-env] %s", msg)
		if server != nil {
			server.showWarningMessage(msg)
		}
		return false
	}

	// Write to temp file
	cachePath := getPythonEnvCachePath(rootPath, cache.PythonVersion)
	safeVersion := sanitizeFilename(cache.PythonVersion)
	tmpPath := filepath.Join(tmpDir, fmt.Sprintf("python_env_%s.json.tmp", safeVersion))

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		msg := fmt.Sprintf("Failed to marshal Python env cache: %v", err)
		log.Printf("[python-env] %s", msg)
		if server != nil {
			server.showWarningMessage(msg)
		}
		return false
	}

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		msg := fmt.Sprintf("Failed to write Python env cache: %v", err)
		log.Printf("[python-env] %s", msg)
		if server != nil {
			server.showWarningMessage(msg)
		}
		return false
	}

	// Atomic rename
	if err := os.Rename(tmpPath, cachePath); err != nil {
		msg := fmt.Sprintf("Failed to finalize Python env cache: %v", err)
		log.Printf("[python-env] %s", msg)
		if server != nil {
			server.showWarningMessage(msg)
		}
		// Clean up temp file
		os.Remove(tmpPath)
		return false
	}

	log.Printf("[python-env] Saved cache to %s", cachePath)
	return true
}

// getFileMtime returns the modification time of a file as Unix timestamp
func getFileMtime(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.ModTime().Unix()
}

// normalizeExternalSearchRoots filters and deduplicates Python paths
func normalizeExternalSearchRoots(rootPath string, paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	roots := make([]string, 0, len(paths))
	rootAbs := ""
	if rootPath != "" {
		rootAbs, _ = filepath.Abs(rootPath)
	}
	for _, p := range paths {
		if p == "" {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if rootAbs != "" && abs == rootAbs {
			continue
		}
		info, err := os.Stat(abs)
		if err != nil || !info.IsDir() {
			continue
		}
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		roots = append(roots, abs)
	}
	return roots
}
