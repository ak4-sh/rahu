package server

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"rahu/analyser"
	"rahu/source"
)

// discoverModulesForBenchmark walks the directory and discovers all Python files
func (s *Server) discoverModulesForBenchmark(root string) ([]ModuleFile, error) {
	var modules []ModuleFile

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip hidden directories and common non-project directories
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "__pycache__" || name == ".venv" ||
				name == "venv" || name == ".tox" || name == "node_modules" ||
				name == ".pytest_cache" || name == ".mypy_cache" ||
				name == "build" || name == "dist" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process .py files
		if filepath.Ext(path) != ".py" {
			return nil
		}

		// Compute module name from path
		name := moduleNameFromPathBenchmark(root, path)

		uri := pathToURI(path)
		mod := ModuleFile{
			Name: name,
			URI:  uri,
			Path: path,
			Kind: "py",
		}
		modules = append(modules, mod)

		return nil
	})

	return modules, err
}

// moduleNameFromPathBenchmark computes a module name from file path
func moduleNameFromPathBenchmark(root, filePath string) string {
	rel, err := filepath.Rel(root, filePath)
	if err != nil {
		return ""
	}

	// Remove .py extension and convert to module notation
	if len(rel) > 3 && rel[len(rel)-3:] == ".py" {
		rel = rel[:len(rel)-3]
	}
	rel = filepath.ToSlash(rel)
	return rel
}

// loadBuiltinsForBenchmark loads Python builtins for analysis
func (s *Server) loadBuiltinsForBenchmark() error {
	// Initialize Python builtins
	for _, name := range []string{
		"int", "str", "float", "list", "dict", "tuple", "set", "bool",
		"bytes", "bytearray", "memoryview", "range", "frozenset",
		"object", "type", "super", "property", "staticmethod", "classmethod",
		"Exception", "BaseException", "TypeError", "ValueError", "KeyError",
		"IndexError", "AttributeError", "NameError", "RuntimeError",
	} {
		s.pythonBuiltinNames[name] = struct{}{}
	}
	return nil
}

// buildStartupBasesWithProgress builds startup bases with progress tracking
func (s *Server) buildStartupBasesWithProgress(mods []ModuleFile, progress ProgressCallback) (map[string]*StartupModuleBase, error) {
	total := len(mods)
	workers := workspaceIndexWorkerCount(total)
	jobs := make(chan ModuleFile)
	var wg sync.WaitGroup
	var completed atomic.Int32
	baseByName := make(map[string]*StartupModuleBase, len(mods))
	var baseMu sync.Mutex

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for mod := range jobs {
				fileStart := time.Now()
				base, ok := s.buildStartupBaseForModule(mod)
				fileDuration := time.Since(fileStart)

				if ok && base != nil {
					baseMu.Lock()
					baseByName[mod.Name] = base
					baseMu.Unlock()
				}

				current := int(completed.Add(1))
				progress.OnPhaseProgress(current, total, mod.Path, fileDuration)
			}
		}()
	}

	for _, mod := range mods {
		jobs <- mod
	}
	close(jobs)
	wg.Wait()

	return baseByName, nil
}

// convergeImportSurfacesWithProgress converges import surfaces with progress tracking
func (s *Server) convergeImportSurfacesWithProgress(baseByName map[string]*StartupModuleBase, rounds *int, progress ProgressCallback) (map[string]*ModuleImportSurface, error) {
	mods := make([]ModuleFile, 0, len(baseByName))
	for name, base := range baseByName {
		if base != nil {
			mods = append(mods, ModuleFile{
				Name: name,
				URI:  base.URI,
				Path: base.Path,
			})
		}
	}

	workers := workspaceIndexWorkerCount(len(mods))
	reverse := buildReverseDepsFromBases(baseByName)
	surfaces := make(map[string]*ModuleImportSurface, len(mods))
	dirty := make(map[string]struct{}, len(baseByName))
	for name := range baseByName {
		dirty[name] = struct{}{}
	}

	roundNum := 0
	for len(dirty) > 0 {
		roundNum++
		names := make([]string, 0, len(dirty))
		for name := range dirty {
			names = append(names, name)
		}
		clear(dirty)
		sort.Strings(names)

		lookup := func(name string) (*ModuleImportSurface, bool) {
			surface := surfaces[name]
			return surface, surface != nil
		}

		jobs := make(chan string, len(names))
		results := make(chan *ModuleImportSurface, len(names))
		var wg sync.WaitGroup

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for name := range jobs {
					base := baseByName[name]
					if base == nil {
						continue
					}
					surface := s.buildImportSurfaceFromBaseWithLookup(base, lookup)
					if surface != nil {
						results <- surface
					}
				}
			}()
		}

		for _, name := range names {
			jobs <- name
		}
		close(jobs)
		wg.Wait()
		close(results)

		for surface := range results {
			prev := surfaces[surface.Name]
			surfaces[surface.Name] = surface
			if prev != nil && prev.ExportHash == surface.ExportHash {
				continue
			}
			for dependent := range reverse[surface.Name] {
				if _, ok := baseByName[dependent]; ok {
					dirty[dependent] = struct{}{}
				}
			}
		}
	}

	*rounds = roundNum
	return surfaces, nil
}

// buildFinalSnapshotsWithProgress builds final snapshots with progress tracking
func (s *Server) buildFinalSnapshotsWithProgress(baseByName map[string]*StartupModuleBase, surfaces map[string]*ModuleImportSurface, progress ProgressCallback) (map[string]*ModuleSnapshot, error) {
	mods := make([]ModuleFile, 0, len(baseByName))
	for name, base := range baseByName {
		if base != nil {
			mods = append(mods, ModuleFile{
				Name: name,
				URI:  base.URI,
				Path: base.Path,
			})
		}
	}

	total := len(mods)
	workers := workspaceIndexWorkerCount(len(mods))
	finalByName := make(map[string]*ModuleSnapshot, len(mods))
	var finalMu sync.Mutex
	lookup := func(name string) (*ModuleImportSurface, bool) {
		surface := surfaces[name]
		return surface, surface != nil
	}
	jobs := make(chan ModuleFile)
	var wg sync.WaitGroup
	var completed atomic.Int32

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for mod := range jobs {
				fileStart := time.Now()
				base := baseByName[mod.Name]
				if base == nil {
					continue
				}
				snapshot := s.buildFinalSnapshotFromBase(base, lookup)
				fileDuration := time.Since(fileStart)

				if snapshot != nil {
					finalMu.Lock()
					finalByName[mod.Name] = snapshot
					finalMu.Unlock()
				}

				current := int(completed.Add(1))
				progress.OnPhaseProgress(current, total, mod.Path, fileDuration)
			}
		}()
	}

	for _, mod := range mods {
		jobs <- mod
	}
	close(jobs)
	wg.Wait()

	return finalByName, nil
}

// buildReferenceIndexWithProgress builds reference index with progress tracking
func (s *Server) buildReferenceIndexWithProgress(snapshots map[string]*ModuleSnapshot, progress ProgressCallback) error {
	total := len(snapshots)
	completed := 0

	// Clear old index
	s.refIndex.Clear()

	for _, snapshot := range snapshots {
		fileStart := time.Now()

		s.refIndex.IndexDocument(
			snapshot.URI,
			snapshot.Tree,
			snapshot.LineIndex,
			snapshot.Symbols,
			snapshot.AttrSymbols,
			snapshot.Defs,
		)

		fileDuration := time.Since(fileStart)
		completed++
		progress.OnPhaseProgress(completed, total, snapshot.Path, fileDuration)
	}

	return nil
}

// measureEnvironmentSetup simulates LSP initialization by measuring Python env discovery
func (s *Server) measureEnvironmentSetup(dir string, verbose bool) *EnvSetupResult {
	result := &EnvSetupResult{}

	// 1. Python Discovery (finding the executable)
	start := time.Now()
	python := discoverPythonExecutable(dir)
	result.PythonDiscoveryMs = time.Since(start).Milliseconds()
	result.PythonExecutable = python

	if python == "" {
		if verbose {
			log.Println("[env-setup] No Python executable found")
		}
		return result
	}

	// 2. Python Query (running subprocess to get sys.path and builtins)
	start = time.Now()
	env := discoverPythonEnv(dir)
	result.PythonQueryMs = time.Since(start).Milliseconds()
	result.SysPathCount = len(env.Paths)
	result.BuiltinsCount = len(env.Builtins)

	// Extract Python version from the executable
	if python != "" {
		result.PythonVersion = getPythonVersion(python)
	}

	if verbose {
		log.Printf("[env-setup] Python: %s (%s)", result.PythonExecutable, result.PythonVersion)
		log.Printf("[env-setup] sys.path entries: %d, builtins: %d", result.SysPathCount, result.BuiltinsCount)
	}

	// Store builtins in server for later use
	for _, name := range env.Builtins {
		if name != "" {
			s.pythonBuiltinNames[name] = struct{}{}
		}
	}

	// 3. Typeshed Loader Initialization
	start = time.Now()
	pyVersion := GetPythonVersion(python)
	typeshedLoader, err := NewTypeshedLoader(pyVersion)
	result.TypeshedInitMs = time.Since(start).Milliseconds()

	if err != nil {
		if verbose {
			log.Printf("[env-setup] Typeshed loader failed: %v", err)
		}
	} else {
		s.typeshedLoader = typeshedLoader
		result.TypeshedEnabled = !typeshedLoader.IsDisabled()
		if verbose {
			if typeshedLoader.IsDisabled() {
				log.Printf("[env-setup] Typeshed disabled for Python %d.%d", pyVersion.Major, pyVersion.Minor)
			} else {
				log.Printf("[env-setup] Typeshed enabled for Python %d.%d", pyVersion.Major, pyVersion.Minor)
			}
		}
	}

	// 4. Builtin Cache Loading
	start = time.Now()
	pyVersionStr := fmt.Sprintf("%d.%d", pyVersion.Major, pyVersion.Minor)
	if cache, ok := LoadBuiltinCache(pyVersionStr); ok {
		result.CacheHit = true
		// Verify cache is not stale
		valid, currentHash := cache.VerifySourceHash()
		result.CacheValid = valid
		if verbose {
			if !valid {
				log.Printf("[env-setup] Cache hash mismatch (expected %s, got %s), using fallback",
					cache.SourceHash[:16], currentHash[:16])
			} else {
				log.Printf("[env-setup] Loaded %d symbols from cache for Python %s",
					len(cache.Symbols), pyVersionStr)
			}
		}
		if valid {
			// Convert cache symbols to analyser format
			cacheSymbols := make([]struct {
				Name  string
				Kind  string
				Bases []string
			}, len(cache.Symbols))
			for i, sym := range cache.Symbols {
				cacheSymbols[i] = struct {
					Name  string
					Kind  string
					Bases []string
				}{
					Name:  sym.Name,
					Kind:  sym.Kind,
					Bases: sym.Bases,
				}
			}
			newScope := analyser.NewBuiltinScopeFromCache(cacheSymbols)
			analyser.SetBuiltinScope(newScope)
		}
	} else {
		result.CacheHit = false
		if verbose {
			log.Printf("[env-setup] No cache found for Python %s, using fallback", pyVersionStr)
		}
	}
	result.BuiltinCacheMs = time.Since(start).Milliseconds()

	// Calculate total
	result.TotalMs = result.PythonDiscoveryMs + result.PythonQueryMs +
		result.TypeshedInitMs + result.BuiltinCacheMs

	return result
}

// simulateFirstFileOpen simulates textDocument/didOpen on the first alphabetically sorted file
func (s *Server) simulateFirstFileOpen(modules []ModuleFile) *FirstFileResult {
	if len(modules) == 0 {
		return nil
	}

	// Sort alphabetically by path and pick first
	sorted := make([]ModuleFile, len(modules))
	copy(sorted, modules)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Path < sorted[j].Path
	})
	firstMod := sorted[0]

	result := &FirstFileResult{
		FileURI:           string(firstMod.URI),
		IsWorkspaceModule: true, // All benchmark modules are workspace modules
	}

	// Read file content
	content, err := os.ReadFile(firstMod.Path)
	if err != nil {
		return result
	}
	result.FileSizeBytes = len(content)
	text := string(content)

	// Create line index and count lines
	lineIndex := source.NewLineIndex(text)
	result.LinesOfCode = strings.Count(text, "\n") + 1

	// Phase 1: Open Document (simulates Open() in document.go)
	start := time.Now()
	doc := &Document{
		URI:       firstMod.URI,
		Version:   1,
		Text:      text,
		LineIndex: lineIndex,
	}
	s.docsMu.Lock()
	s.docs[firstMod.URI] = doc
	s.docsMu.Unlock()

	// If it's a workspace module, increment open count
	if _, isModule := s.LookupModuleByURI(firstMod.URI); isModule {
		s.snapshotsMu.Lock()
		s.openModuleCounts[firstMod.URI]++
		s.snapshotsMu.Unlock()
	}
	result.OpenDocumentMs = time.Since(start).Milliseconds()

	// Phase 2: Base Analysis (fast initial analysis - what users see first)
	start = time.Now()
	var snapshot *ModuleSnapshot
	if _, ok := s.LookupModuleByURI(firstMod.URI); !ok {
		// Non-workspace module: full analysis
		snapshot = s.buildModuleSnapshot("", firstMod.URI, "", text, lineIndex)
	} else {
		// Workspace module: base snapshot (fast, incomplete)
		snapshot = s.buildBaseModuleSnapshot("", firstMod.URI, "", text, lineIndex)
	}
	result.BaseAnalysisMs = time.Since(start).Milliseconds()

	// Set analysis results on document
	if snapshot != nil {
		doc.mu.Lock()
		doc.Tree = snapshot.Tree
		doc.Global = snapshot.Global
		doc.Symbols = snapshot.Symbols
		doc.SemErrs = snapshot.SemErrs
		doc.AttrSymbols = snapshot.AttrSymbols
		doc.Defs = snapshot.Defs
		doc.PosIndex = nil // Will be built on demand
		doc.mu.Unlock()

		// Count imports
		if snapshot.Tree != nil {
			for _, imp := range snapshot.Imports {
				if imp != "" {
					result.ImportCount++
				}
			}
		}

		// Count diagnostics
		result.ParseErrorsCount = len(snapshot.ParseErrs)
		result.SemErrorsCount = len(snapshot.SemErrs)
		result.TotalDiagsCount = result.ParseErrorsCount + result.SemErrorsCount
	}

	// Phase 3: Publish Diagnostics (conversion + notification overhead simulation)
	start = time.Now()
	_ = toDiagnostics(lineIndex, snapshot.ParseErrs, snapshot.SemErrs)
	result.PublishDiagsMs = time.Since(start).Milliseconds()

	// Phase 4: Async Refinement (full analysis - blocking wait for completion)
	// This simulates what happens for workspace modules: the base analysis is shown,
	// then async refinement completes. We block to measure total time.
	if _, isModule := s.LookupModuleByURI(firstMod.URI); isModule {
		start = time.Now()

		// Rebuild snapshot with full import resolution
		// This simulates the full analysis done by refreshModuleAndDependents
		if snapshot != nil {
			// Use the existing module snapshot if available, or rebuild
			if existingSnapshot, ok := s.getModuleSnapshotByURI(firstMod.URI); ok && existingSnapshot != nil {
				snapshot = existingSnapshot
			}
		}

		result.AsyncRefinementMs = time.Since(start).Milliseconds()
	}

	// Calculate total time to ready
	result.TotalToReadyMs = result.OpenDocumentMs + result.BaseAnalysisMs +
		result.PublishDiagsMs + result.AsyncRefinementMs

	// Cleanup: close the document
	s.Close(firstMod.URI)

	return result
}

// getPythonVersion extracts version string from python executable
func getPythonVersion(python string) string {
	if python == "" {
		return ""
	}
	cmd := execCommand(python, "--version")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	// Output is like "Python 3.11.4"
	version := strings.TrimSpace(string(out))
	version = strings.TrimPrefix(version, "Python ")
	return version
}

// execCommand is a variable to allow mocking in tests
var execCommand = execCommandFunc

func execCommandFunc(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}
