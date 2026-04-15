package server

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"rahu"
	"rahu/lsp"
	"rahu/source"
)

type pythonModuleInfo struct {
	Kind         string              `json:"kind"`
	Members      []string            `json:"members"`
	Classes      []string            `json:"classes"`
	ClassMethods map[string][]string `json:"class_methods"` // class_name -> methods
	Origin       string              `json:"origin"`        // file path when kind == "source"
}

func pythonSyntheticModuleURI(kind, name string) lsp.DocumentURI {
	return lsp.DocumentURI(kind + ":///" + strings.ReplaceAll(name, ".", "/"))
}

func typeshedStubURI(stubPath string) lsp.DocumentURI {
	return lsp.DocumentURI("typeshed:///" + stubPath)
}

func isTypeshedURI(uri lsp.DocumentURI) bool {
	return strings.HasPrefix(string(uri), "typeshed:///")
}

func isSyntheticModule(mod ModuleFile) bool {
	return mod.Kind == "builtin" || mod.Kind == "frozen" || isTypeshedURI(mod.URI)
}

func isSyntheticURI(uri lsp.DocumentURI) bool {
	s := string(uri)
	return strings.HasPrefix(s, "builtin:///") || strings.HasPrefix(s, "frozen:///") || strings.HasPrefix(s, "typeshed:///")
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
	classes := make(map[string]struct{}, len(info.Classes))
	for _, name := range info.Classes {
		if name == "" {
			continue
		}
		classes[name] = struct{}{}
	}
	lines := make([]string, 0, len(members)+1)
	lines = append(lines, "__name__ = None")
	for _, name := range members {
		if !isValidSyntheticIdentifier(name) || name == "__name__" {
			continue
		}
		if _, ok := classes[name]; ok {
			lines = append(lines, "class "+name+":")
			// Add method stubs if available
			if methods, ok := info.ClassMethods[name]; ok && len(methods) > 0 {
				for _, method := range methods {
					lines = append(lines, "    def "+method+"(self, *args, **kwargs): pass")
				}
			} else {
				lines = append(lines, "    pass")
			}
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
	cmd := exec.Command(python, "-c", `import importlib, importlib.util, inspect, json, sys
name = sys.argv[1]
spec = importlib.util.find_spec(name)
payload = {"kind": "", "members": [], "classes": [], "class_methods": {}, "origin": ""}
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
            members = sorted({member for member in dir(module) if isinstance(member, str)})
            payload["members"] = members
            classes = sorted({member for member in members if inspect.isclass(getattr(module, member, None))})
            payload["classes"] = classes
            # Capture class methods
            class_methods = {}
            for cls_name in classes:
                try:
                    cls = getattr(module, cls_name)
                    methods = []
                    for attr in dir(cls):
                        if attr.startswith('_'):
                            continue
                        try:
                            val = getattr(cls, attr)
                            if callable(val):
                                methods.append(attr)
                        except:
                            pass
                    if methods:
                        class_methods[cls_name] = sorted(methods)
                except:
                    pass
            payload["class_methods"] = class_methods
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

	// Handle typeshed stubs
	if isTypeshedURI(mod.URI) {
		return s.typeshedStubSource(mod)
	}

	// Handle builtin/frozen modules
	info, ok := s.pythonModuleInfo(mod.Name)
	if !ok {
		return "", nil, false
	}
	text := syntheticModuleText(info)
	return text, source.NewLineIndex(text), true
}

// typeshedStubSource reads a typeshed stub from the embedded filesystem.
func (s *Server) typeshedStubSource(mod ModuleFile) (string, *source.LineIndex, bool) {
	if s.typeshedLoader == nil {
		return "", nil, false
	}

	// Extract the stub path from the URI
	uri := string(mod.URI)
	if !strings.HasPrefix(uri, "typeshed:///") {
		return "", nil, false
	}
	stubPath := strings.TrimPrefix(uri, "typeshed:///")

	// Read from embedded FS
	data, err := fs.ReadFile(rahu.TypeshedFS(), stubPath+".pyi")
	if err != nil {
		// Try as a package __init__.pyi
		data, err = fs.ReadFile(rahu.TypeshedFS(), stubPath+"/__init__.pyi")
		if err != nil {
			return "", nil, false
		}
	}

	text := string(data)
	return text, source.NewLineIndex(text), true
}

func (s *Server) resolveExternalModule(name string) (ModuleFile, bool) {
	s.indexMu.RLock()
	if mod, ok := s.externalModulesByName[name]; ok {
		s.indexMu.RUnlock()
		return mod, true
	}

	// Try typeshed first
	if s.typeshedLoader != nil && !s.typeshedLoader.IsDisabled() {
		if f, ok := s.typeshedLoader.FindStub(name); ok {
			s.indexMu.RUnlock()
			defer f.Close()

			// Read stub content to determine the path for URI
			// The typeshed loader returns a file from the embedded FS
			// We create a virtual URI for it
			stubPath := s.typeshedLoader.GetStubPath(name)
			uri := typeshedStubURI(stubPath)
			mod := ModuleFile{
				Name: name,
				URI:  uri,
				Path: "", // Typeshed stubs don't have a real filesystem path
				Kind: "typeshed",
			}
			s.indexMu.Lock()
			s.cacheExternalModuleLocked(mod)
			s.indexMu.Unlock()
			return mod, true
		}
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

// inspectPythonMethod performs runtime introspection of a class method.
// Returns signature and docstring from the actual Python implementation.
func inspectPythonMethod(python, module, class, method string) (pythonMethodInfo, bool) {
	if python == "" || module == "" || class == "" || method == "" {
		return pythonMethodInfo{}, false
	}
	cmd := exec.Command(python, "-c", `import inspect, importlib, json, sys
try:
    mod = importlib.import_module(sys.argv[1])
    cls = getattr(mod, sys.argv[2])
    meth = getattr(cls, sys.argv[3])
    # Get signature if available
    try:
        sig = str(inspect.signature(meth))
    except:
        sig = "(...)"
    # Get docstring
    doc = inspect.getdoc(meth) or ""
    result = {"signature": sig, "docstring": doc}
except Exception as e:
    result = {"signature": "", "docstring": "", "error": str(e)}
print(json.dumps(result))`, module, class, method)

	output, err := cmd.Output()
	if err != nil {
		return pythonMethodInfo{}, false
	}

	var result struct {
		Signature string `json:"signature"`
		Docstring string `json:"docstring"`
		Error     string `json:"error"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return pythonMethodInfo{}, false
	}
	if result.Error != "" {
		return pythonMethodInfo{}, false
	}

	return pythonMethodInfo{
		Module:    module,
		Class:     class,
		Method:    method,
		Signature: result.Signature,
		Docstring: result.Docstring,
		CachedAt:  time.Now(),
	}, true
}

// getMethodInfo returns cached method info or performs lazy introspection.
// Results are cached for 1 hour.
func (s *Server) getMethodInfo(module, class, method string) (pythonMethodInfo, bool) {
	key := module + "." + class + "." + method

	// Try cache first
	s.methodCacheMu.RLock()
	info, ok := s.pythonMethodCache[key]
	s.methodCacheMu.RUnlock()

	if ok && time.Since(info.CachedAt) < 1*time.Hour {
		return info, true
	}

	// Lazy introspection
	info, ok = inspectPythonMethod(s.pythonExecutable, module, class, method)
	if !ok {
		return pythonMethodInfo{}, false
	}

	// Cache result
	s.methodCacheMu.Lock()
	s.pythonMethodCache[key] = info
	s.methodCacheMu.Unlock()

	return info, true
}
