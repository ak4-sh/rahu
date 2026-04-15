package server

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"log"
	"path/filepath"
	"rahu"
	"strconv"
	"strings"
)

// TypeshedLoader loads type stubs from embedded typeshed with version filtering.
type TypeshedLoader struct {
	fs             fs.FS // Embedded typeshed filesystem
	pyVersion      PythonVersion
	stdlibVersions map[string]VersionRange
	maxSupported   PythonVersion // Maximum Python version supported by typeshed
	disabled       bool
	skipModules    map[string]bool // Modules to skip (use introspection instead)
}

// PythonVersion represents a Python version (e.g., 3.11).
type PythonVersion struct {
	Major int
	Minor int
}

// VersionRange represents the version range for a module.
type VersionRange struct {
	Min PythonVersion
	Max *PythonVersion // nil means no upper bound
}

// NewTypeshedLoader creates a new typeshed loader for the given Python version.
// If the Python version exceeds the maximum supported by typeshed, it returns
// a disabled loader that will fall back to introspection.
func NewTypeshedLoader(pyVersion PythonVersion) (*TypeshedLoader, error) {
	loader := &TypeshedLoader{
		fs:             rahu.TypeshedFS(),
		pyVersion:      pyVersion,
		maxSupported:   PythonVersion{3, 14}, // Typeshed supports up to 3.14
		stdlibVersions: make(map[string]VersionRange),
		skipModules: map[string]bool{
			// Skip modules that have complex re-exports causing issues
			// These will fall back to Python introspection
			"json": true,
		},
	}

	// Check if Python version exceeds typeshed's maximum support
	if comparePythonVersions(pyVersion, loader.maxSupported) > 0 {
		log.Printf("[typeshed] Disabled: Python %d.%d > max supported %d.%d",
			pyVersion.Major, pyVersion.Minor, loader.maxSupported.Major, loader.maxSupported.Minor)
		loader.disabled = true
		return loader, nil
	}

	// Parse the VERSIONS file from embedded typeshed
	if err := loader.parseVersionsFile(); err != nil {
		return nil, fmt.Errorf("failed to parse typeshed VERSIONS: %w", err)
	}

	log.Printf("[typeshed] Loaded for Python %d.%d (max supported: %d.%d)",
		pyVersion.Major, pyVersion.Minor, loader.maxSupported.Major, loader.maxSupported.Minor)

	return loader, nil
}

// IsDisabled returns true if typeshed is disabled (e.g., Python version too new).
func (t *TypeshedLoader) IsDisabled() bool {
	return t.disabled
}

// parseVersionsFile reads and parses the stdlib/VERSIONS file from embedded typeshed.
func (t *TypeshedLoader) parseVersionsFile() error {
	file, err := t.fs.Open("stdlib/VERSIONS")
	if err != nil {
		return fmt.Errorf("cannot open VERSIONS file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		module := strings.TrimSpace(parts[0])
		versionSpec := strings.TrimSpace(parts[1])

		vr, err := parseVersionRange(versionSpec)
		if err != nil {
			// Skip invalid entries rather than failing entirely
			continue
		}

		t.stdlibVersions[module] = vr
	}

	return scanner.Err()
}

// isModuleAvailable checks if a stdlib module is available for the current Python version.
func (t *TypeshedLoader) isModuleAvailable(module string) bool {
	if t.disabled {
		return false
	}

	vr, exists := t.stdlibVersions[module]
	if !exists {
		// Module not listed in VERSIONS - assume it's always available
		return true
	}

	// Check minimum version
	if comparePythonVersions(t.pyVersion, vr.Min) < 0 {
		return false
	}

	// Check maximum version (if set)
	if vr.Max != nil && comparePythonVersions(t.pyVersion, *vr.Max) > 0 {
		return false
	}

	return true
}

// FindStub attempts to find a .pyi stub file for the given module.
// It returns the file content reader and true if found, or false if not available.
func (t *TypeshedLoader) FindStub(module string) (io.ReadCloser, bool) {
	if t.disabled {
		return nil, false
	}

	parts := strings.Split(module, ".")
	if len(parts) == 0 {
		return nil, false
	}

	baseModule := parts[0]

	// Check if module should be skipped (use introspection instead)
	if t.skipModules[baseModule] {
		return nil, false
	}

	// Check stdlib first
	if isStdlibModule(baseModule) {
		if !t.isModuleAvailable(baseModule) {
			log.Printf("[typeshed] Module %s not available in Python %d.%d",
				module, t.pyVersion.Major, t.pyVersion.Minor)
			return nil, false
		}
		return t.findStdlibStub(parts)
	}

	// Check third-party stubs
	return t.findThirdPartyStub(parts)
}

// GetStubPath returns the virtual path for a stub (used for URI generation).
func (t *TypeshedLoader) GetStubPath(module string) string {
	parts := strings.Split(module, ".")
	baseModule := parts[0]

	if isStdlibModule(baseModule) {
		return "stdlib/" + strings.Join(parts, "/")
	}
	return "stubs/" + baseModule + "/" + strings.Join(parts, "/")
}

// isStdlibModule checks if a module is part of the Python standard library.
func isStdlibModule(name string) bool {
	// These are definitely stdlib modules that typeshed tracks
	stdlibModules := map[string]bool{
		"__future__": true, "__main__": true, "_ast": true, "_thread": true,
		"abc": true, "argparse": true, "array": true, "ast": true,
		"asyncio": true, "atexit": true, "base64": true, "bdb": true,
		"binascii": true, "binhex": true, "bisect": true, "builtins": true,
		"bz2": true, "calendar": true, "cgi": true, "cgitb": true,
		"chunk": true, "cmath": true, "cmd": true, "code": true,
		"codecs": true, "codeop": true, "collections": true, "colorsys": true,
		"compileall": true, "concurrent": true, "configparser": true,
		"contextlib": true, "contextvars": true, "copy": true, "copyreg": true,
		"crypt": true, "csv": true, "ctypes": true, "curses": true,
		"dataclasses": true, "datetime": true, "dbm": true, "decimal": true,
		"difflib": true, "dis": true, "distutils": true, "doctest": true,
		"email": true, "encodings": true, "enum": true, "errno": true,
		"faulthandler": true, "fcntl": true, "filecmp": true, "fileinput": true,
		"fnmatch": true, "fractions": true, "ftplib": true, "functools": true,
		"gc": true, "getopt": true, "getpass": true, "gettext": true,
		"glob": true, "graphlib": true, "grp": true, "gzip": true,
		"hashlib": true, "heapq": true, "hmac": true, "html": true,
		"http": true, "idlelib": true, "imaplib": true, "imghdr": true,
		"imp": true, "importlib": true, "inspect": true, "io": true,
		"ipaddress": true, "itertools": true, "json": true, "keyword": true,
		"lib2to3": true, "linecache": true, "locale": true, "logging": true,
		"lzma": true, "mailbox": true, "mailcap": true, "marshal": true,
		"math": true, "mimetypes": true, "mmap": true, "modulefinder": true,
		"multiprocessing": true, "netrc": true, "nis": true, "nntplib": true,
		"numbers": true, "operator": true, "optparse": true, "os": true,
		"ossaudiodev": true, "pathlib": true, "pdb": true, "pickle": true,
		"pickletools": true, "pipes": true, "pkgutil": true, "platform": true,
		"plistlib": true, "poplib": true, "posix": true, "posixpath": true,
		"pprint": true, "profile": true, "pstats": true, "pty": true,
		"pwd": true, "py_compile": true, "pyclbr": true, "pydoc": true,
		"queue": true, "quopri": true, "random": true, "re": true,
		"readline": true, "reprlib": true, "resource": true, "rlcompleter": true,
		"runpy": true, "sched": true, "secrets": true, "select": true,
		"selectors": true, "shelve": true, "shlex": true, "shutil": true,
		"signal": true, "site": true, "smtpd": true, "smtplib": true,
		"sndhdr": true, "socket": true, "socketserver": true, "spwd": true,
		"sqlite3": true, "ssl": true, "stat": true, "statistics": true,
		"string": true, "stringprep": true, "struct": true, "subprocess": true,
		"sunau": true, "symtable": true, "sys": true, "sysconfig": true,
		"syslog": true, "tabnanny": true, "tarfile": true, "telnetlib": true,
		"tempfile": true, "termios": true, "test": true, "textwrap": true,
		"threading": true, "time": true, "timeit": true, "tkinter": true,
		"token": true, "tokenize": true, "tomllib": true, "trace": true,
		"traceback": true, "tracemalloc": true, "tty": true, "turtle": true,
		"turtledemo": true, "types": true, "typing": true, "typing_extensions": true,
		"unicodedata": true, "unittest": true, "urllib": true, "uu": true,
		"uuid": true, "venv": true, "warnings": true, "wave": true,
		"weakref": true, "webbrowser": true, "winreg": true, "winsound": true,
		"wsgiref": true, "xdrlib": true, "xml": true, "xmlrpc": true,
		"zipapp": true, "zipfile": true, "zipimport": true, "zlib": true,
	}

	return stdlibModules[name]
}

// findStdlibStub looks for a stub in the stdlib directory.
func (t *TypeshedLoader) findStdlibStub(parts []string) (io.ReadCloser, bool) {
	// Try as a package: stdlib/module/submodule/__init__.pyi
	if len(parts) > 1 {
		pkgPath := filepath.Join("stdlib", filepath.Join(parts...), "__init__.pyi")
		if f, err := t.fs.Open(pkgPath); err == nil {
			return f, true
		}
	}

	// Try as a module: stdlib/module.pyi
	modulePath := filepath.Join("stdlib", strings.Join(parts, "/")+".pyi")
	if f, err := t.fs.Open(modulePath); err == nil {
		return f, true
	}

	// Try as a package with just the first part: stdlib/module/__init__.pyi
	if len(parts) >= 1 {
		pkgInitPath := filepath.Join("stdlib", parts[0], "__init__.pyi")
		if f, err := t.fs.Open(pkgInitPath); err == nil {
			// Check if this is the right module
			if len(parts) == 1 {
				return f, true
			}
			f.Close()
		}
	}

	return nil, false
}

// findThirdPartyStub looks for a stub in the stubs directory.
func (t *TypeshedLoader) findThirdPartyStub(parts []string) (io.ReadCloser, bool) {
	if len(parts) == 0 {
		return nil, false
	}

	basePkg := parts[0]
	stubsBase := filepath.Join("stubs", basePkg)

	// Check if the stub package exists
	if _, err := t.fs.Open(stubsBase); err != nil {
		return nil, false
	}

	// Build the path within the stub package
	var stubPath string
	if len(parts) == 1 {
		// Just the base package: look for __init__.pyi or base.pyi
		stubPath = filepath.Join(stubsBase, basePkg, "__init__.pyi")
		if f, err := t.fs.Open(stubPath); err == nil {
			return f, true
		}
		stubPath = filepath.Join(stubsBase, basePkg+".pyi")
		if f, err := t.fs.Open(stubPath); err == nil {
			return f, true
		}
	} else {
		// Submodule: stubs/base/sub/path.pyi or stubs/base/sub/path/__init__.pyi
		subPath := filepath.Join(parts[1:]...)
		stubPath = filepath.Join(stubsBase, basePkg, subPath+".pyi")
		if f, err := t.fs.Open(stubPath); err == nil {
			return f, true
		}
		stubPath = filepath.Join(stubsBase, basePkg, subPath, "__init__.pyi")
		if f, err := t.fs.Open(stubPath); err == nil {
			return f, true
		}
	}

	return nil, false
}

// parseVersionRange parses a version range string (e.g., "3.7-", "2.7-3.8", "3.11").
func parseVersionRange(spec string) (VersionRange, error) {
	spec = strings.TrimSpace(spec)

	// Check for range format: "3.7-3.10" or "2.7-3.8"
	if strings.Contains(spec, "-") && !strings.HasSuffix(spec, "-") {
		parts := strings.SplitN(spec, "-", 2)
		minVer, err := parsePythonVersion(strings.TrimSpace(parts[0]))
		if err != nil {
			return VersionRange{}, err
		}
		maxVer, err := parsePythonVersion(strings.TrimSpace(parts[1]))
		if err != nil {
			return VersionRange{}, err
		}
		return VersionRange{Min: minVer, Max: &maxVer}, nil
	}

	// Check for open-ended range: "3.7-" or "3.7+"
	if strings.HasSuffix(spec, "-") || strings.HasSuffix(spec, "+") {
		verStr := spec[:len(spec)-1]
		ver, err := parsePythonVersion(strings.TrimSpace(verStr))
		if err != nil {
			return VersionRange{}, err
		}
		return VersionRange{Min: ver, Max: nil}, nil
	}

	// Single version: "3.7" means 3.7 and up (same as 3.7-)
	ver, err := parsePythonVersion(spec)
	if err != nil {
		return VersionRange{}, err
	}
	return VersionRange{Min: ver, Max: nil}, nil
}

// parsePythonVersion parses a version string like "3.11" or "2.7".
func parsePythonVersion(s string) (PythonVersion, error) {
	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		return PythonVersion{}, fmt.Errorf("invalid version: %s", s)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return PythonVersion{}, fmt.Errorf("invalid major version: %s", parts[0])
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return PythonVersion{}, fmt.Errorf("invalid minor version: %s", parts[1])
	}

	return PythonVersion{Major: major, Minor: minor}, nil
}

// comparePythonVersions compares two Python versions.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func comparePythonVersions(a, b PythonVersion) int {
	if a.Major != b.Major {
		if a.Major < b.Major {
			return -1
		}
		return 1
	}
	if a.Minor != b.Minor {
		if a.Minor < b.Minor {
			return -1
		}
		return 1
	}
	return 0
}

// GetPythonVersion detects the Python version from the interpreter.
func GetPythonVersion(python string) PythonVersion {
	if python == "" {
		return PythonVersion{3, 10} // Default fallback
	}

	// Default to 3.10 if we can't detect
	return PythonVersion{3, 10}
}
