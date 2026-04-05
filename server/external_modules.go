package server

import (
	"os"
	"path/filepath"
	"strings"
)

func externalModuleCandidates(root, name string) []string {
	if root == "" || name == "" {
		return nil
	}
	rel := filepath.FromSlash(strings.ReplaceAll(name, ".", "/"))
	return []string{
		filepath.Join(root, rel, "__init__.py"),
		filepath.Join(root, rel+".py"),
	}
}

func (s *Server) cacheExternalModuleLocked(mod ModuleFile) {
	s.externalModulesByName[mod.Name] = mod
	s.externalModulesByURI[mod.URI] = mod
}

func (s *Server) resolveExternalModule(name string) (ModuleFile, bool) {
	s.indexMu.RLock()
	if mod, ok := s.externalModulesByName[name]; ok {
		s.indexMu.RUnlock()
		return mod, true
	}
	roots := append([]string(nil), s.externalSearchRoots...)
	s.indexMu.RUnlock()

	for _, root := range roots {
		for _, candidate := range externalModuleCandidates(root, name) {
			info, err := os.Stat(candidate)
			if err != nil || info.IsDir() {
				continue
			}
			mod := ModuleFile{Name: name, URI: pathToURI(candidate), Path: candidate}
			s.indexMu.Lock()
			if existing, ok := s.modulesByName[name]; ok {
				s.indexMu.Unlock()
				return existing, true
			}
			if existing, ok := s.externalModulesByName[name]; ok {
				s.indexMu.Unlock()
				return existing, true
			}
			s.cacheExternalModuleLocked(mod)
			s.indexMu.Unlock()
			return mod, true
		}
	}

	return ModuleFile{}, false
}
