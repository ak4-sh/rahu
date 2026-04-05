package server

import (
	"context"
	"io/fs"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"rahu/lsp"
)

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

	total := len(mods)
	workers := workspaceIndexWorkerCount(total)
	jobs := make(chan ModuleFile)
	var wg sync.WaitGroup
	var completed atomic.Int32

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
					_, _ = s.analyzeModuleFile(mod)
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
	if err := ctx.Err(); err != nil {
		return err
	}

	s.rebuildReverseDeps()
	return nil
}

func workspaceIndexWorkerCount(total int) int {
	if total <= 1 {
		return 1
	}
	if total >= 4 {
		return 4
	}
	return total
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
