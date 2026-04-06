package server

import (
	"context"
	"io/fs"
	"log"
	"net/url"
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
	phaseABuild      time.Duration
	phaseABuildTotal atomic.Int64
	phaseBBind       time.Duration
	phaseBBindTotal  time.Duration
	refIndexBuild    time.Duration
	reverseDeps      time.Duration
}

func (t *startupIndexTimings) log() {
	if t == nil {
		return
	}
	log.Printf(
		"INDEX: modules=%d phase_a=%s phase_a_total=%s phase_b=%s phase_b_total=%s ref_index=%s reverse_deps=%s",
		t.moduleCount,
		t.phaseABuild,
		totalDurationFromAtomic(&t.phaseABuildTotal),
		t.phaseBBind,
		t.phaseBBindTotal,
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
	if rootPath == "" || filepath.Ext(filePath) != ".py" {
		return "", false
	}

	rootAbs, err := filepath.Abs(rootPath)
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
	if base == "__init__.py" {
		dir := filepath.Dir(rel)
		if dir == "." {
			return "", false
		}
		rel = dir
	} else {
		rel = strings.TrimSuffix(rel, ".py")
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

func (s *Server) buildModuleIndex() {
	_ = s.buildModuleIndexWithContext(context.Background())
}

func (s *Server) buildModuleIndexWithContext(ctx context.Context) error {
	s.miscMu.Lock()
	rootPath := s.rootPath
	s.miscMu.Unlock()

	modulesByName := make(map[string]ModuleFile)
	modulesByURI := make(map[lsp.DocumentURI]ModuleFile)
	if rootPath == "" {
		s.indexMu.Lock()
		s.modulesByName = modulesByName
		s.modulesByURI = modulesByURI
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

		if err != nil || d == nil || d.IsDir() {
			return nil
		}

		name, ok := moduleNameFromPath(rootPath, path)
		if !ok {
			return nil
		}

		if _, exists := modulesByName[name]; exists {
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
	_ = s.buildWorkspaceSnapshotsWithPriority(context.Background())
}

func (s *Server) buildWorkspaceSnapshotsWithPriority(ctx context.Context) error {
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
	total := len(mods)
	workers := workspaceIndexWorkerCount(total)
	jobs := make(chan ModuleFile)
	var wg sync.WaitGroup
	var completed atomic.Int32
	baseByName := make(map[string]*ModuleSnapshot, len(mods))
	var baseMu sync.Mutex

	phaseAStart := time.Now()
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
					snapshot, ok := s.buildBaseSnapshotForModule(mod)
					addDurationAtomic(&timings.phaseABuildTotal, time.Since(started))
					if ok && snapshot != nil {
						baseMu.Lock()
						baseByName[mod.Name] = snapshot
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
			return ctx.Err()
		case jobs <- mod:
		}
	}
	close(jobs)
	wg.Wait()
	timings.phaseABuild = time.Since(phaseAStart)
	if err := ctx.Err(); err != nil {
		return err
	}

	lookup := func(name string) (*ModuleSnapshot, bool) {
		baseMu.Lock()
		snapshot := baseByName[name]
		baseMu.Unlock()
		return snapshot, snapshot != nil
	}

	phaseBStart := time.Now()
	for _, mod := range mods {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		baseMu.Lock()
		snapshot := baseByName[mod.Name]
		baseMu.Unlock()
		if snapshot == nil {
			continue
		}
		started := time.Now()
		snapshot.SemErrs = append(snapshot.SemErrs, s.bindWorkspaceImportsWithLookup(snapshot.Tree, snapshot.Defs, snapshot.URI, lookup)...)
		snapshot.Exports = extractExports(snapshot.Global)
		snapshot.ExportHash = computeExportHash(snapshot.Exports)
		timings.phaseBBindTotal += time.Since(started)
	}
	timings.phaseBBind = time.Since(phaseBStart)

	s.refIndex.Clear()
	phaseRefStart := time.Now()
	for _, mod := range mods {
		baseMu.Lock()
		snapshot := baseByName[mod.Name]
		baseMu.Unlock()
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
	if snapshot == nil || s.Get(snapshot.URI) == nil {
		return
	}

	s.SetAnalysis(snapshot.URI, snapshot.Tree, snapshot.Global, snapshot.Defs, snapshot.Symbols, snapshot.AttrSymbols, snapshot.SemErrs)
	s.publishDiagnostics(snapshot.URI, toDiagnostics(snapshot.LineIndex, snapshot.ParseErrs, snapshot.SemErrs))
}

func (s *Server) refreshModuleAndDependents(uri lsp.DocumentURI) {
	oldRoot, _ := s.getModuleSnapshotByURI(uri)
	oldRootHash := uint64(0)
	if oldRoot != nil {
		oldRootHash = oldRoot.ExportHash
	}

	rootSnapshot, ok := s.rebuildModuleByURI(uri)
	if !ok || rootSnapshot == nil {
		return
	}
	s.applySnapshotToOpenDocument(rootSnapshot)
	if oldRootHash != 0 && oldRootHash == rootSnapshot.ExportHash {
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

			snapshot, ok := s.rebuildModuleByURI(depURI)
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
