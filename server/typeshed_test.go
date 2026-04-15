package server

import (
	"testing"
)

func TestParseVersionRange(t *testing.T) {
	tests := []struct {
		input   string
		wantMin PythonVersion
		wantMax *PythonVersion
		wantErr bool
	}{
		{"3.8", PythonVersion{3, 8}, nil, false},
		{"3.8-", PythonVersion{3, 8}, nil, false},
		{"3.8+", PythonVersion{3, 8}, nil, false},
		{"2.7-3.8", PythonVersion{2, 7}, &PythonVersion{3, 8}, false},
		{"3.10", PythonVersion{3, 10}, nil, false},
		{"3", PythonVersion{}, nil, true},
		{"", PythonVersion{}, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseVersionRange(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseVersionRange(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if got.Min != tt.wantMin {
				t.Errorf("parseVersionRange(%q).Min = %v, want %v", tt.input, got.Min, tt.wantMin)
			}
			if (got.Max == nil) != (tt.wantMax == nil) {
				t.Errorf("parseVersionRange(%q).Max = %v, want %v", tt.input, got.Max, tt.wantMax)
				return
			}
			if got.Max != nil && tt.wantMax != nil && *got.Max != *tt.wantMax {
				t.Errorf("parseVersionRange(%q).Max = %v, want %v", tt.input, *got.Max, *tt.wantMax)
			}
		})
	}
}

func TestComparePythonVersions(t *testing.T) {
	tests := []struct {
		a    PythonVersion
		b    PythonVersion
		want int
	}{
		{PythonVersion{3, 8}, PythonVersion{3, 8}, 0},
		{PythonVersion{3, 8}, PythonVersion{3, 9}, -1},
		{PythonVersion{3, 9}, PythonVersion{3, 8}, 1},
		{PythonVersion{2, 7}, PythonVersion{3, 0}, -1},
		{PythonVersion{3, 10}, PythonVersion{3, 2}, 1},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := comparePythonVersions(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("comparePythonVersions(%v, %v) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestIsModuleAvailable(t *testing.T) {
	// Create a loader for Python 3.10
	loader := &TypeshedLoader{
		pyVersion:      PythonVersion{3, 10},
		maxSupported:   PythonVersion{3, 14},
		stdlibVersions: make(map[string]VersionRange),
	}

	// Add some test version ranges
	loader.stdlibVersions["tomllib"] = VersionRange{Min: PythonVersion{3, 11}, Max: nil}
	loader.stdlibVersions["dataclasses"] = VersionRange{Min: PythonVersion{3, 7}, Max: nil}
	loader.stdlibVersions["asyncio"] = VersionRange{Min: PythonVersion{3, 4}, Max: nil}
	loader.stdlibVersions["removed_mod"] = VersionRange{Min: PythonVersion{2, 7}, Max: &PythonVersion{3, 8}}

	tests := []struct {
		module   string
		expected bool
	}{
		{"tomllib", false},     // Not available in 3.10 (added in 3.11)
		{"dataclasses", true},  // Available in 3.10
		{"asyncio", true},      // Available in 3.10
		{"removed_mod", false}, // Removed after 3.8
		{"unknown_mod", true},  // Not listed, assumed available
	}

	for _, tt := range tests {
		t.Run(tt.module, func(t *testing.T) {
			got := loader.isModuleAvailable(tt.module)
			if got != tt.expected {
				t.Errorf("isModuleAvailable(%q) = %v, want %v", tt.module, got, tt.expected)
			}
		})
	}
}

func TestIsStdlibModule(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"os", true},
		{"sys", true},
		{"asyncio", true},
		{"urllib3", false},  // Third-party
		{"requests", false}, // Third-party
		{"my_custom_lib", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isStdlibModule(tt.name)
			if got != tt.want {
				t.Errorf("isStdlibModule(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestTypeshedDisabledForNewerPython(t *testing.T) {
	loader, err := NewTypeshedLoader(PythonVersion{3, 15}) // Future Python version
	if err != nil {
		t.Fatalf("NewTypeshedLoader failed: %v", err)
	}

	if !loader.IsDisabled() {
		t.Error("Expected loader to be disabled for Python 3.15")
	}

	// Should not find any stubs when disabled
	f, ok := loader.FindStub("os")
	if ok {
		f.Close()
		t.Error("Expected FindStub to return false when disabled")
	}
}

func TestTypeshedSkipsCertainModules(t *testing.T) {
	loader, err := NewTypeshedLoader(PythonVersion{3, 10})
	if err != nil {
		t.Fatalf("Failed to create loader: %v", err)
	}

	// Test that json module is skipped (falls back to introspection)
	f, ok := loader.FindStub("json")
	if ok {
		f.Close()
		t.Error("Expected json to be skipped (in skip list)")
	}

	// Test that other modules like os work
	f, ok = loader.FindStub("os")
	if !ok {
		t.Error("Expected to find os stub")
	} else {
		f.Close()
	}
}
