package server

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"rahu/lsp"
	"rahu/source"
)

type pythonModuleInfo struct {
	Kind    string   `json:"kind"`
	Members []string `json:"members"`
	Origin  string   `json:"origin"` // file path when kind == "source"
}

func pythonSyntheticModuleURI(kind, name string) lsp.DocumentURI {
	return lsp.DocumentURI(kind + ":///" + strings.ReplaceAll(name, ".", "/"))
}

func isSyntheticModule(mod ModuleFile) bool {
	return mod.Kind == "builtin" || mod.Kind == "frozen"
}

func isValidSyntheticIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func syntheticModuleText(info pythonModuleInfo) string {
	members := append([]string(nil), info.Members...)
	sort.Strings(members)
	lines := make([]string, 0, len(members)+1)
	lines = append(lines, "__name__ = None")
	for _, name := range members {
		if !isValidSyntheticIdentifier(name) || name == "__name__" {
			continue
		}
		lines = append(lines, name+" = None")
	}
	return strings.Join(lines, "\n") + "\n"
}

func externalModuleCandidates(root, name string) []string {
	if root == "" || name == "" {
		return nil
	}
	rel := filepath.FromSlash(strings.ReplaceAll(name, ".", "/"))
	return []string{
		filepath.Join(root, rel, "__init__.pyi"),
		filepath.Join(root, rel, "__init__.py"),
		filepath.Join(root, rel+".pyi"),
		filepath.Join(root, rel+".py"),
	}
}

func (s *Server) cacheExternalModuleLocked(mod ModuleFile) {
	s.externalModulesByName[mod.Name] = mod
	s.externalModulesByURI[mod.URI] = mod
}

func inspectPythonModuleInfo(python, name string) (pythonModuleInfo, bool) {
	if python == "" || name == "" {
		return pythonModuleInfo{}, false
	}
	cmd := exec.Command(python, "-c", `import importlib, importlib.util, json, sys
name = sys.argv[1]
spec = importlib.util.find_spec(name)
payload = {"kind": "", "members": [], "origin": ""}
if spec is not None:
    origin = spec.origin or ""
    if origin == "built-in":
        payload["kind"] = "builtin"
    elif origin == "frozen":
        payload["kind"] = "frozen"
    elif origin.endswith((".py", ".pyi")):
        payload["kind"] = "source"
        payload["origin"] = origin
    if payload["kind"] in ("builtin", "frozen"):
        try:
            module = importlib.import_module(name)
            payload["members"] = sorted({member for member in dir(module) if isinstance(member, str)})
        except Exception:
            pass
print(json.dumps(payload))`, name)
	output, err := cmd.Output()
	if err != nil {
		return pythonModuleInfo{}, false
	}
	var info pythonModuleInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return pythonModuleInfo{}, false
	}
	return info, info.Kind != ""
}

func inspectPythonModuleMembers(python, name string) ([]string, bool) {
	if python == "" || name == "" {
		return nil, false
	}
	cmd := exec.Command(python, "-c", `import importlib, json, sys
name = sys.argv[1]
module = importlib.import_module(name)
print(json.dumps(sorted({member for member in dir(module) if isinstance(member, str)})))`, name)
	output, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	var members []string
	if err := json.Unmarshal(output, &members); err != nil {
		return nil, false
	}
	return members, true
}

func (s *Server) pythonModuleInfo(name string) (pythonModuleInfo, bool) {
	s.indexMu.RLock()
	if info, ok := s.pythonModuleInfoByName[name]; ok {
		s.indexMu.RUnlock()
		return info, true
	}
	python := s.pythonExecutable
	_, builtin := s.pythonBuiltinNames[name]
	s.indexMu.RUnlock()

	if python == "" {
		return pythonModuleInfo{}, false
	}

	if builtin {
		info := pythonModuleInfo{Kind: "builtin"}
		if inspected, ok := inspectPythonModuleInfo(python, name); ok && inspected.Kind != "" {
			info = inspected
		}
		s.indexMu.Lock()
		s.pythonModuleInfoByName[name] = info
		s.indexMu.Unlock()
		return info, true
	}

	info, ok := inspectPythonModuleInfo(python, name)
	if !ok || (info.Kind != "builtin" && info.Kind != "frozen") {
		return pythonModuleInfo{}, false
	}
	s.indexMu.Lock()
	s.pythonModuleInfoByName[name] = info
	s.indexMu.Unlock()
	return info, true
}

func (s *Server) pythonModuleMembers(name string) ([]string, bool) {
	s.indexMu.RLock()
	if info, ok := s.pythonModuleInfoByName[name]; ok && info.Members != nil {
		s.indexMu.RUnlock()
		return append([]string(nil), info.Members...), true
	}
	python := s.pythonExecutable
	s.indexMu.RUnlock()
	if python == "" {
		return nil, false
	}
	members, ok := inspectPythonModuleMembers(python, name)
	if !ok {
		return nil, false
	}
	s.indexMu.Lock()
	info := s.pythonModuleInfoByName[name]
	info.Members = append([]string(nil), members...)
	s.pythonModuleInfoByName[name] = info
	s.indexMu.Unlock()
	return members, true
}

func (s *Server) syntheticModuleSource(mod ModuleFile) (string, *source.LineIndex, bool) {
	if !isSyntheticModule(mod) {
		return "", nil, false
	}
	info, ok := s.pythonModuleInfo(mod.Name)
	if !ok {
		return "", nil, false
	}
	text := syntheticModuleText(info)
	return text, source.NewLineIndex(text), true
}

func (s *Server) resolveExternalModule(name string) (ModuleFile, bool) {
	s.indexMu.RLock()
	if mod, ok := s.externalModulesByName[name]; ok {
		s.indexMu.RUnlock()
		return mod, true
	}
	roots := append([]string(nil), s.externalSearchRoots...)
	python := s.pythonExecutable
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

	// Fall back to Python introspection for modules that don't have a file at
	// the expected path — e.g. `os.path` which is an alias for `posixpath`.
	inspected, ok := inspectPythonModuleInfo(python, name)
	if !ok || inspected.Kind == "" {
		return ModuleFile{}, false
	}

	var mod ModuleFile
	switch inspected.Kind {
	case "source":
		if inspected.Origin == "" {
			return ModuleFile{}, false
		}
		mod = ModuleFile{Name: name, URI: pathToURI(inspected.Origin), Path: inspected.Origin}
	default: // "builtin" or "frozen"
		s.indexMu.Lock()
		s.pythonModuleInfoByName[name] = inspected
		s.indexMu.Unlock()
		mod = ModuleFile{Name: name, URI: pythonSyntheticModuleURI(inspected.Kind, name), Kind: inspected.Kind}
	}

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
