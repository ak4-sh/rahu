package server

import (
	"io/fs"
	"net/url"
	"path/filepath"
	"strings"

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
	s.mu.RLock()
	rootPath := s.rootPath
	s.mu.RUnlock()

	modulesByName := make(map[string]ModuleFile)
	modulesByURI := make(map[lsp.DocumentURI]ModuleFile)
	if rootPath == "" {
		s.mu.Lock()
		s.modulesByName = modulesByName
		s.modulesByURI = modulesByURI
		s.mu.Unlock()
		return
	}

	_ = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
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

	s.mu.Lock()
	s.modulesByName = modulesByName
	s.modulesByURI = modulesByURI
	s.moduleImportsByURI = make(map[lsp.DocumentURI][]string)
	s.reverseDepsByModule = make(map[string]map[lsp.DocumentURI]struct{})
	s.buildingModules = make(map[string]bool)
	s.moduleSnapshotsByName = make(map[string]*ModuleSnapshot)
	s.moduleSnapshotsByURI = make(map[lsp.DocumentURI]*ModuleSnapshot)
	s.mu.Unlock()
}

func (s *Server) buildWorkspaceSnapshots() {
	s.mu.RLock()
	mods := make([]ModuleFile, 0, len(s.modulesByName))
	for _, mod := range s.modulesByName {
		mods = append(mods, mod)
	}
	s.mu.RUnlock()

	for _, mod := range mods {
		_, _ = s.analyzeModuleFile(mod)
	}

	s.rebuildReverseDeps()
}

func (s *Server) rebuildReverseDeps() {
	s.mu.Lock()
	defer s.mu.Unlock()

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
	rootSnapshot, ok := s.rebuildModuleByURI(uri)
	if !ok || rootSnapshot == nil {
		return
	}
	s.applySnapshotToOpenDocument(rootSnapshot)

	queue := []string{rootSnapshot.Name}
	visitedModules := map[string]struct{}{rootSnapshot.Name: {}}
	visitedURIs := map[lsp.DocumentURI]struct{}{uri: {}}

	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]

		s.mu.RLock()
		dependents := s.reverseDepsByModule[name]
		uris := make([]lsp.DocumentURI, 0, len(dependents))
		for depURI := range dependents {
			uris = append(uris, depURI)
		}
		s.mu.RUnlock()

		for _, depURI := range uris {
			if _, seen := visitedURIs[depURI]; seen {
				continue
			}
			visitedURIs[depURI] = struct{}{}

			snapshot, ok := s.rebuildModuleByURI(depURI)
			if !ok || snapshot == nil {
				continue
			}
			s.applySnapshotToOpenDocument(snapshot)
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
	s.mu.RLock()
	defer s.mu.RUnlock()

	module, ok := s.modulesByName[name]
	return module, ok
}

func (s *Server) LookupModuleByURI(uri lsp.DocumentURI) (ModuleFile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	module, ok := s.modulesByURI[uri]
	return module, ok
}
