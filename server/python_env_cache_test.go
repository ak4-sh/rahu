package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPythonEnvCache(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Test cache path generation
	cachePath := getPythonEnvCachePath(tmpDir, "3.11.4")
	expectedPath := filepath.Join(tmpDir, ".rahu", "python_env_3_11_4.json")
	if cachePath != expectedPath {
		t.Errorf("Expected cache path %s, got %s", expectedPath, cachePath)
	}

	// Test sanitizeFilename
	safe := sanitizeFilename("3.11.4")
	if safe != "3_11_4" {
		t.Errorf("Expected sanitized name 3_11_4, got %s", safe)
	}

	// Test isCacheExpired
	recent := time.Now().UTC().Format(time.RFC3339)
	if isCacheExpired(recent, 7) {
		t.Error("Recent cache should not be expired")
	}

	old := time.Now().Add(-8 * 24 * time.Hour).UTC().Format(time.RFC3339)
	if !isCacheExpired(old, 7) {
		t.Error("Old cache should be expired")
	}

	// Test save and load
	server := &Server{}
	cache := &pythonEnvCache{
		Executable:      "/usr/bin/python3",
		ExecutableMtime: 1234567890,
		PythonVersion:   "3.11.4",
		Paths:           []string{"/usr/lib/python3.11", "/usr/local/lib"},
		Builtins:        []string{"int", "str", "list"},
		CachedAt:        time.Now().UTC().Format(time.RFC3339),
	}

	if !savePythonEnvCache(tmpDir, cache, server) {
		t.Error("Failed to save cache")
	}

	// Verify file was created
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Errorf("Cache file was not created at %s", cachePath)
	}

	// Load and verify
	loaded := loadPythonEnvCache(tmpDir, "3.11.4")
	if loaded == nil {
		t.Fatal("Failed to load cache")
	}

	if loaded.Executable != cache.Executable {
		t.Errorf("Expected executable %s, got %s", cache.Executable, loaded.Executable)
	}

	if len(loaded.Paths) != len(cache.Paths) {
		t.Errorf("Expected %d paths, got %d", len(cache.Paths), len(loaded.Paths))
	}
}

func TestTypeshedCacheIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	server := &Server{}

	// Create a cache with typeshed data
	cache := &pythonEnvCache{
		Executable:      "/usr/bin/python3",
		ExecutableMtime: 1234567890,
		PythonVersion:   "3_10", // sanitized
		CachedAt:        time.Now().UTC().Format(time.RFC3339),
		Typeshed: &typeshedCacheData{
			MaxSupported: PythonVersion{3, 14},
			StdlibVersions: map[string]VersionRange{
				"os": {Min: PythonVersion{3, 7}, Max: nil},
			},
			SkipModules: []string{"json"},
		},
	}

	// Save cache
	if !savePythonEnvCache(tmpDir, cache, server) {
		t.Fatal("Failed to save cache with typeshed data")
	}

	// Load and verify typeshed data
	loaded := loadPythonEnvCache(tmpDir, "3_10")
	if loaded == nil {
		t.Fatal("Failed to load cache")
	}

	if loaded.Typeshed == nil {
		t.Fatal("Typeshed data not found in cache")
	}

	if loaded.Typeshed.MaxSupported.Major != 3 || loaded.Typeshed.MaxSupported.Minor != 14 {
		t.Errorf("Expected max supported 3.14, got %d.%d",
			loaded.Typeshed.MaxSupported.Major, loaded.Typeshed.MaxSupported.Minor)
	}

	if len(loaded.Typeshed.SkipModules) != 1 || loaded.Typeshed.SkipModules[0] != "json" {
		t.Errorf("Expected skip modules [json], got %v", loaded.Typeshed.SkipModules)
	}
}
