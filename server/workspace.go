package server

import (
	"context"
	"io/fs"
	"log"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"rahu/lsp"
)

type startupIndexTimings struct {
	moduleCount      int
	priorityModules  int
	priorityRounds   int
	priorityReady    time.Duration
	moduleWalk       time.Duration
	prioritySetBuild time.Duration
	deferredComplete time.Duration
	phaseABuild      time.Duration
	phaseABuildTotal atomic.Int64
	phaseBBind       time.Duration
	phaseBBindTotal  atomic.Int64
	refIndexBuild    time.Duration
	reverseDeps      time.Duration
}

func (t *startupIndexTimings) log() {
	if t == nil {
		return
	}
	log.Printf(
		"INDEX: modules=%d priority_modules=%d priority_rounds=%d priority_ready=%s deferred_complete=%s walk=%s priority_set=%s phase_a=%s phase_a_total=%s phase_b=%s phase_b_total=%s ref_index=%s reverse_deps=%s",
		t.moduleCount,
		t.priorityModules,
		t.priorityRounds,
		t.priorityReady,
		t.deferredComplete,
		t.moduleWalk,
		t.prioritySetBuild,
		t.phaseABuild,
		totalDurationFromAtomic(&t.phaseABuildTotal),
		t.phaseBBind,
		totalDurationFromAtomic(&t.phaseBBindTotal),
		t.refIndexBuild,
		t.reverseDeps,
	)
}

func totalDurationFromAtomic(v *atomic.Int64) time.Duration {
	if v == nil {
		return 0
	}
	return time.Duration(v.Load())
}

func addDurationAtomic(v *atomic.Int64, d time.Duration) {
	if v == nil {
		return
	}
	v.Add(int64(d))
}

func uriToPath(uri lsp.DocumentURI) (string, bool) {
	u, err := url.Parse(string(uri))
	if err != nil || u.Scheme != "file" || u.Path == "" {
		return "", false
	}

	path, err := filepath.Abs(filepath.FromSlash(u.Path))
	if err != nil {
		return "", false
	}

	return path, true
}

func pathToURI(path string) lsp.DocumentURI {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	return lsp.DocumentURI((&url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(abs),
	}).String())
}

func moduleNameFromPath(rootPath, filePath string) (string, bool) {
	return moduleNameFromImportRoot(rootPath, filePath)
}

func moduleNameFromImportRoot(importRoot, filePath string) (string, bool) {
	ext := filepath.Ext(filePath)
	if importRoot == "" || (ext != ".py" && ext != ".pyi") {
		return "", false
	}

	rootAbs, err := filepath.Abs(importRoot)
	if err != nil {
		return "", false
	}
	fileAbs, err := filepath.Abs(filePath)
	if err != nil {
		return "", false
	}

	rel, err := filepath.Rel(rootAbs, fileAbs)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}

	base := filepath.Base(rel)
	if base == "__init__.py" || base == "__init__.pyi" {
		dir := filepath.Dir(rel)
		if dir == "." {
			return "", false
		}
		rel = dir
	} else {
		rel = strings.TrimSuffix(rel, ext)
	}

	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", false
		}
	}

	return strings.Join(parts, "."), true
}

func hasPythonProjectMarker(path string) bool {
	for _, name := range []string{"pyproject.toml", "setup.py", "setup.cfg"} {
		info, err := os.Stat(filepath.Join(path, name))
		if err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func hasImportablePythonModule(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			if strings.HasPrefix(name, ".") || shouldSkipWorkspaceDir(name) {
				continue
			}
			if _, ok := moduleNameFromImportRoot(path, filepath.Join(path, name, "__init__.py")); ok {
				return true
			}
			if _, ok := moduleNameFromImportRoot(path, filepath.Join(path, name, "__init__.pyi")); ok {
				return true
			}
			continue
		}
		if isPythonModulePath(name) {
			return true
		}
	}
	return false
}

func detectImportRoot(projectRoot string) string {
	if projectRoot == "" {
		return ""
	}
	srcRoot := filepath.Join(projectRoot, "src")
	if info, err := os.Stat(srcRoot); err == nil && info.IsDir() && hasImportablePythonModule(srcRoot) {
		return srcRoot
	}
	return projectRoot
}

func appendPythonProjectRoot(roots []PythonProjectRoot, projectRoot string) []PythonProjectRoot {
	if projectRoot == "" {
		return roots
	}
	projectAbs, err := filepath.Abs(projectRoot)
	if err != nil {
		return roots
	}
	for _, root := range roots {
		if root.ProjectRoot == projectAbs {
			return roots
		}
	}
	return append(roots, PythonProjectRoot{ProjectRoot: projectAbs, ImportRoot: detectImportRoot(projectAbs)})
}

func findPythonProjectRoots(rootPath string) []PythonProjectRoot {
	if rootPath == "" {
		return nil
	}
	rootAbs, err := filepath.Abs(rootPath)
	if err != nil {
		return []PythonProjectRoot{{ProjectRoot: rootPath, ImportRoot: rootPath}}
	}
	roots := []PythonProjectRoot{{ProjectRoot: rootAbs, ImportRoot: rootAbs}}
	_ = filepath.WalkDir(rootAbs, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if path != rootAbs && shouldSkipWorkspaceDir(d.Name()) {
			return filepath.SkipDir
		}
		if path != rootAbs && hasPythonProjectMarker(path) {
			roots = appendPythonProjectRoot(roots, path)
		}
		return nil
	})
	sort.Slice(roots, func(i, j int) bool {
		if len(roots[i].ImportRoot) == len(roots[j].ImportRoot) {
			return roots[i].ImportRoot < roots[j].ImportRoot
		}
		return len(roots[i].ImportRoot) > len(roots[j].ImportRoot)
	})
	return roots
}

func moduleNameForPath(filePath string, roots []PythonProjectRoot) (string, bool) {
	fileAbs, err := filepath.Abs(filePath)
	if err != nil {
		return "", false
	}
	for _, root := range roots {
		importRoot := root.ImportRoot
		if importRoot == "" {
			continue
		}
		importAbs, err := filepath.Abs(importRoot)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(importAbs, fileAbs)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			continue
		}
		return moduleNameFromImportRoot(importAbs, fileAbs)
	}
	return "", false
}

func modulePathPriority(path string) int {
	base := filepath.Base(path)
	switch {
	case base == "__init__.pyi":
		return 0
	case base == "__init__.py":
		return 1
	case strings.HasSuffix(base, ".pyi"):
		return 2
	case strings.HasSuffix(base, ".py"):
		return 3
	default:
		return 4
	}
}

func shouldPreferModulePath(candidate, existing string) bool {
	if existing == "" {
		return true
	}
	return modulePathPriority(candidate) < modulePathPriority(existing)
}

func shouldSkipWorkspaceDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", ".venv", "venv", "dist", "build", "target", ".next", ".turbo", ".cache", "coverage":
		return true
	default:
		return false
	}
}

func isPythonModulePath(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".py" || ext == ".pyi"
}

func (s *Server) buildModuleIndex() {
	_ = s.buildModuleIndexWithContext(context.Background())
}

func (s *Server) buildModuleIndexWithContext(ctx context.Context) error {
	s.miscMu.Lock()
	rootPath := s.rootPath
	s.miscMu.Unlock()

	modulesByName := make(map[string]ModuleFile)
	modulesByURI := make(map[lsp.DocumentURI]ModuleFile)
	projectRoots := findPythonProjectRoots(rootPath)
	if rootPath == "" {
		s.indexMu.Lock()
		s.modulesByName = modulesByName
		s.modulesByURI = modulesByURI
		s.pythonProjectRoots = nil
		s.indexMu.Unlock()
		return nil
	}

	var walkErr error
	_ = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		// Check for cancellation
		select {
		case <-ctx.Done():
			walkErr = ctx.Err()
			return ctx.Err()
		default:
		}

		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			if shouldSkipWorkspaceDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isPythonModulePath(path) {
			return nil
		}

		name, ok := moduleNameForPath(path, projectRoots)
		if !ok {
			return nil
		}

		if existing, exists := modulesByName[name]; exists && !shouldPreferModulePath(path, existing.Path) {
			return nil
		}

		module := ModuleFile{
			Name: name,
			URI:  pathToURI(path),
			Path: path,
		}
		modulesByName[name] = module
		modulesByURI[module.URI] = module
		return nil
	})

	if walkErr != nil {
		return walkErr
	}

	// Update module index
	s.indexMu.Lock()
	s.modulesByName = modulesByName
	s.modulesByURI = modulesByURI
	s.pythonProjectRoots = projectRoots
	s.externalModulesByName = make(map[string]ModuleFile)
	s.externalModulesByURI = make(map[lsp.DocumentURI]ModuleFile)
	s.indexMu.Unlock()

	// Reset dependencies
	s.depsMu.Lock()
	s.moduleImportsByURI = make(map[lsp.DocumentURI][]string)
	s.reverseDepsByModule = make(map[string]map[lsp.DocumentURI]struct{})
	s.depsMu.Unlock()

	// Reset snapshots
	s.snapshotsMu.Lock()
	s.buildingModules = make(map[string]chan struct{})
	s.openModuleCounts = make(map[lsp.DocumentURI]int)
	s.moduleSnapshotsByName = make(map[string]*ModuleSnapshot)
	s.moduleSnapshotsByURI = make(map[lsp.DocumentURI]*ModuleSnapshot)
	s.snapshotLRU = newSnapshotLRU()
	s.snapshotsMu.Unlock()

	s.docsMu.RLock()
	openURIs := make([]lsp.DocumentURI, 0, len(s.docs))
	for uri := range s.docs {
		openURIs = append(openURIs, uri)
	}
	s.docsMu.RUnlock()

	s.snapshotsMu.Lock()
	for _, uri := range openURIs {
		if _, ok := modulesByURI[uri]; ok {
			s.openModuleCounts[uri]++
		}
	}
	s.snapshotsMu.Unlock()

	return nil
}

func (s *Server) buildWorkspaceSnapshots() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = s.buildWorkspaceSnapshotsWithPriority(ctx, cancel)
}

func (s *Server) buildWorkspaceSnapshotsWithPriority(ctx context.Context, cancel context.CancelFunc) error {
	s.indexMu.RLock()
	mods := make([]ModuleFile, 0, len(s.modulesByName))
	for _, mod := range s.modulesByName {
		mods = append(mods, mod)
	}
	s.indexMu.RUnlock()

	s.miscMu.Lock()
	priorityDir := s.priorityDir
	s.miscMu.Unlock()

	// Sort by priority: priorityDir first, then parents, then rest
	sortModulesByPriority(mods, priorityDir)

	timings := &startupIndexTimings{moduleCount: len(mods)}

	phaseAStart := time.Now()
	baseByName, err := s.buildStartupBases(ctx, mods, timings)
	if err != nil {
		return err
	}
	timings.phaseABuild = time.Since(phaseAStart)

	priorityStart := time.Now()
	openMods := s.startupOpenWorkspaceModules()
	priorityNames := buildPriorityModuleSet(openMods, baseByName)
	timings.prioritySetBuild = time.Since(priorityStart)
	timings.priorityModules = len(priorityNames)
	priorityMods := filterModulesByNameSet(mods, priorityNames)

	s.miscMu.Lock()
	if s.startup == nil {
		s.startup = newStartupReadiness()
	}
	s.startup.priorityModuleNames = maps.Clone(priorityNames)
	s.startup.priorityOpenURIs = make(map[lsp.DocumentURI]struct{}, len(openMods))
	for _, mod := range openMods {
		s.startup.priorityOpenURIs[mod.URI] = struct{}{}
	}
	s.startup.priorityModuleCount = len(priorityNames)
	s.miscMu.Unlock()

	phaseBStart := time.Now()
	if len(priorityMods) != 0 {
		initialDirty := maps.Clone(priorityNames)
		prioritySurfaces := s.convergeImportSurfaces(ctx, priorityMods, baseByName, initialDirty, timings)
		priorityFinal := s.buildFinalSnapshots(ctx, priorityMods, baseByName, prioritySurfaces, timings)
		s.publishPrioritySnapshots(priorityMods, priorityFinal)
		s.miscMu.Lock()
		if s.startup != nil && s.startup.priorityReadyAt.IsZero() {
			s.startup.priorityReadyAt = time.Now()
			s.startup.prioritySurfaceRounds = timings.priorityRounds
		}
		s.miscMu.Unlock()
		s.markPriorityReadyIfSatisfied()
	}
	timings.priorityReady = time.Since(phaseBStart)

	deferredStart := time.Now()
	allNames := make(map[string]struct{}, len(mods))
	for _, mod := range mods {
		allNames[mod.Name] = struct{}{}
	}
	surfaceByName := s.convergeImportSurfaces(ctx, mods, baseByName, allNames, timings)
	finalByName := s.buildFinalSnapshots(ctx, mods, baseByName, surfaceByName, timings)
	timings.phaseBBind = time.Since(phaseBStart)
	timings.deferredComplete = time.Since(deferredStart)

	s.refIndex.Clear()
	phaseRefStart := time.Now()
	for _, mod := range mods {
		snapshot := finalByName[mod.Name]
		if snapshot == nil {
			continue
		}
		s.publishModuleSnapshot(mod, snapshot)
		s.refIndex.IndexDocument(snapshot.URI, snapshot.Tree, snapshot.LineIndex, snapshot.Symbols, snapshot.AttrSymbols, snapshot.Defs)
	}
	timings.refIndexBuild = time.Since(phaseRefStart)
	s.enforceSnapshotLRULimit()

	reverseDepsStart := time.Now()
	s.rebuildReverseDeps()
	timings.reverseDeps = time.Since(reverseDepsStart)
	if s.conn != nil {
		timings.log()
	}
	return nil
}

func workspaceIndexWorkerCount(total int) int {
	return workspaceIndexWorkerCountWithAvailable(total, runtime.GOMAXPROCS(0))
}

func workspaceIndexWorkerCountWithAvailable(total, available int) int {
	if total <= 1 {
		return 1
	}
	if available < 1 {
		available = 1
	}
	workers := min(total, available)
	return min(workers, 8)
}

func sortModulesByPriority(mods []ModuleFile, priorityDir string) {
	if priorityDir == "" {
		return
	}
	sort.SliceStable(mods, func(i, j int) bool {
		return modulePriority(mods[i].Path, priorityDir) < modulePriority(mods[j].Path, priorityDir)
	})
}

func modulePriority(path, priorityDir string) int {
	dir := filepath.Dir(path)
	if dir == priorityDir {
		return 0 // Highest: same directory as user
	}
	if strings.HasPrefix(priorityDir, dir+string(filepath.Separator)) {
		return 1 // Second: parent directories
	}
	return 2 // Lowest: everything else
}

func (s *Server) startupOpenWorkspaceModules() []ModuleFile {
	s.docsMu.RLock()
	openURIs := make([]lsp.DocumentURI, 0, len(s.docs))
	for uri := range s.docs {
		openURIs = append(openURIs, uri)
	}
	s.docsMu.RUnlock()

	mods := make([]ModuleFile, 0, len(openURIs))
	seen := make(map[string]struct{}, len(openURIs))
	for _, uri := range openURIs {
		mod, ok := s.LookupModuleByURI(uri)
		if !ok || mod.Name == "" {
			continue
		}
		if _, exists := seen[mod.Name]; exists {
			continue
		}
		seen[mod.Name] = struct{}{}
		mods = append(mods, mod)
	}
	return mods
}

func buildPriorityModuleSet(openMods []ModuleFile, baseByName map[string]*StartupModuleBase) map[string]struct{} {
	priority := make(map[string]struct{}, len(openMods))
	queue := make([]string, 0, len(openMods))
	for _, mod := range openMods {
		if mod.Name == "" {
			continue
		}
		if _, exists := priority[mod.Name]; exists {
			continue
		}
		priority[mod.Name] = struct{}{}
		queue = append(queue, mod.Name)
	}

	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		base := baseByName[name]
		if base == nil {
			continue
		}
		for _, dep := range base.Imports {
			if dep == "" {
				continue
			}
			if _, ok := baseByName[dep]; !ok {
				continue
			}
			if _, exists := priority[dep]; exists {
				continue
			}
			priority[dep] = struct{}{}
			queue = append(queue, dep)
		}
	}

	return priority
}

func filterModulesByNameSet(mods []ModuleFile, names map[string]struct{}) []ModuleFile {
	if len(names) == 0 {
		return nil
	}
	filtered := make([]ModuleFile, 0, len(names))
	for _, mod := range mods {
		if _, ok := names[mod.Name]; ok {
			filtered = append(filtered, mod)
		}
	}
	return filtered
}

func buildReverseDepsFromBases(baseByName map[string]*StartupModuleBase) map[string]map[string]struct{} {
	reverse := make(map[string]map[string]struct{}, len(baseByName))
	for name, base := range baseByName {
		if base == nil {
			continue
		}
		for _, dep := range base.Imports {
			if dep == "" {
				continue
			}
			if reverse[dep] == nil {
				reverse[dep] = make(map[string]struct{})
			}
			reverse[dep][name] = struct{}{}
		}
	}
	return reverse
}

func (s *Server) buildStartupBases(ctx context.Context, mods []ModuleFile, timings *startupIndexTimings) (map[string]*StartupModuleBase, error) {
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
			for {
				select {
				case <-ctx.Done():
					return
				case mod, ok := <-jobs:
					if !ok {
						return
					}
					started := time.Now()
					base, ok := s.buildStartupBaseForModule(mod)
					addDurationAtomic(&timings.phaseABuildTotal, time.Since(started))
					if ok && base != nil {
						baseMu.Lock()
						baseByName[mod.Name] = base
						baseMu.Unlock()
					}
					current := int(completed.Add(1))
					if current%10 == 0 || current == total {
						s.reportIndexingProgress(current, total)
					}
				}
			}
		}()
	}

	for _, mod := range mods {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return nil, ctx.Err()
		case jobs <- mod:
		}
	}
	close(jobs)
	wg.Wait()
	return baseByName, nil
}

func (s *Server) convergeImportSurfaces(ctx context.Context, mods []ModuleFile, baseByName map[string]*StartupModuleBase, initialDirty map[string]struct{}, timings *startupIndexTimings) map[string]*ModuleImportSurface {
	workers := workspaceIndexWorkerCount(len(mods))
	reverse := buildReverseDepsFromBases(baseByName)
	surfaces := make(map[string]*ModuleImportSurface, len(mods))
	dirty := make(map[string]struct{}, len(initialDirty))
	for name := range initialDirty {
		dirty[name] = struct{}{}
	}
	rounds := 0

	for len(dirty) > 0 {
		select {
		case <-ctx.Done():
			return surfaces
		default:
		}
		rounds++
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
					started := time.Now()
					surface := s.buildImportSurfaceFromBaseWithLookup(base, lookup)
					addDurationAtomic(&timings.phaseBBindTotal, time.Since(started))
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

	if timings != nil {
		timings.priorityRounds = rounds
	}
	return surfaces
}

func (s *Server) buildFinalSnapshots(ctx context.Context, mods []ModuleFile, baseByName map[string]*StartupModuleBase, surfaces map[string]*ModuleImportSurface, timings *startupIndexTimings) map[string]*ModuleSnapshot {
	workers := workspaceIndexWorkerCount(len(mods))
	finalByName := make(map[string]*ModuleSnapshot, len(mods))
	var finalMu sync.Mutex
	lookup := func(name string) (*ModuleImportSurface, bool) {
		surface := surfaces[name]
		return surface, surface != nil
	}
	jobs := make(chan ModuleFile, len(mods))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for mod := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				base := baseByName[mod.Name]
				if base == nil {
					continue
				}
				started := time.Now()
				snapshot := s.buildFinalSnapshotFromBase(base, lookup)
				addDurationAtomic(&timings.phaseBBindTotal, time.Since(started))
				if snapshot != nil {
					finalMu.Lock()
					finalByName[mod.Name] = snapshot
					finalMu.Unlock()
				}
			}
		}()
	}
	for _, mod := range mods {
		jobs <- mod
	}
	close(jobs)
	wg.Wait()
	return finalByName
}

func (s *Server) publishPrioritySnapshots(mods []ModuleFile, finalByName map[string]*ModuleSnapshot) {
	for _, mod := range mods {
		snapshot := finalByName[mod.Name]
		if snapshot == nil {
			continue
		}
		s.publishModuleSnapshot(mod, snapshot)
		s.refIndex.IndexDocument(snapshot.URI, snapshot.Tree, snapshot.LineIndex, snapshot.Symbols, snapshot.AttrSymbols, snapshot.Defs)
		s.applySnapshotToOpenDocument(snapshot)
	}
	if len(mods) != 0 {
		s.enforceSnapshotLRULimit()
	}
}

func (s *Server) snapshotMatchesOpenDocument(snapshot *ModuleSnapshot) bool {
	if snapshot == nil {
		return false
	}
	doc := s.Get(snapshot.URI)
	if doc == nil {
		return false
	}
	doc.mu.RLock()
	defer doc.mu.RUnlock()
	return computeTextHash(doc.Text) == snapshot.TextHash
}

func (s *Server) rebuildReverseDeps() {
	s.depsMu.Lock()
	defer s.depsMu.Unlock()

	reverse := make(map[string]map[lsp.DocumentURI]struct{})
	for uri, imports := range s.moduleImportsByURI {
		for _, dep := range imports {
			if dep == "" {
				continue
			}
			if reverse[dep] == nil {
				reverse[dep] = make(map[lsp.DocumentURI]struct{})
			}
			reverse[dep][uri] = struct{}{}
		}
	}

	s.reverseDepsByModule = reverse
}

func (s *Server) applySnapshotToOpenDocument(snapshot *ModuleSnapshot) {
	if snapshot == nil || s.Get(snapshot.URI) == nil || !s.snapshotMatchesOpenDocument(snapshot) {
		return
	}

	s.SetAnalysis(snapshot.URI, snapshot.Tree, snapshot.Global, snapshot.Defs, snapshot.Symbols, snapshot.AttrSymbols, snapshot.SemErrs)
	s.markOpenDocumentSnapshotApplied(snapshot.URI)
	s.publishDiagnostics(snapshot.URI, toDiagnostics(snapshot.LineIndex, snapshot.ParseErrs, snapshot.SemErrs))
}

func (s *Server) refreshModuleAndDependents(uri lsp.DocumentURI) {
	oldRoot, _ := s.getModuleSnapshotByURI(uri)
	oldRootHash := uint64(0)
	if oldRoot != nil {
		oldRootHash = oldRoot.ExportHash
	}

	rootSnapshot, ok := s.rebuildModuleSnapshotOnly(uri)
	if !ok || rootSnapshot == nil {
		return
	}
	s.applySnapshotToOpenDocument(rootSnapshot)
	if oldRootHash != 0 && oldRootHash == rootSnapshot.ExportHash {
		// Exports unchanged: no dependents need rebuilding, but root's import
		// list may have changed (e.g. user added "import os"), so update revdeps.
		s.rebuildReverseDeps()
		return
	}

	queue := []string{rootSnapshot.Name}
	visitedModules := map[string]struct{}{rootSnapshot.Name: {}}
	visitedURIs := map[lsp.DocumentURI]struct{}{uri: {}}

	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]

		s.depsMu.RLock()
		dependents := s.reverseDepsByModule[name]
		uris := make([]lsp.DocumentURI, 0, len(dependents))
		for depURI := range dependents {
			uris = append(uris, depURI)
		}
		s.depsMu.RUnlock()

		for _, depURI := range uris {
			if _, seen := visitedURIs[depURI]; seen {
				continue
			}
			visitedURIs[depURI] = struct{}{}

			oldSnapshot, _ := s.getModuleSnapshotByURI(depURI)
			oldHash := uint64(0)
			if oldSnapshot != nil {
				oldHash = oldSnapshot.ExportHash
			}

			snapshot, ok := s.rebuildModuleSnapshotOnly(depURI)
			if !ok || snapshot == nil {
				continue
			}
			s.applySnapshotToOpenDocument(snapshot)
			if oldHash != 0 && oldHash == snapshot.ExportHash {
				continue
			}
			if snapshot.Name != "" {
				if _, seen := visitedModules[snapshot.Name]; !seen {
					visitedModules[snapshot.Name] = struct{}{}
					queue = append(queue, snapshot.Name)
				}
			}
		}
	}

	// Rebuild reverse deps once after all snapshots are updated, capturing any
	// import-list changes in the root module.
	s.rebuildReverseDeps()
}

func (s *Server) LookupModule(name string) (ModuleFile, bool) {
	s.indexMu.RLock()
	module, ok := s.modulesByName[name]
	if !ok {
		module, ok = s.externalModulesByName[name]
	}
	s.indexMu.RUnlock()
	if ok {
		return module, true
	}
	return s.resolveExternalModule(name)
}

func (s *Server) LookupModuleByURI(uri lsp.DocumentURI) (ModuleFile, bool) {
	s.indexMu.RLock()
	module, ok := s.modulesByURI[uri]
	if !ok {
		module, ok = s.externalModulesByURI[uri]
	}
	s.indexMu.RUnlock()
	return module, ok
}
