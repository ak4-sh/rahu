package server

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
)

type pythonEnvInfo struct {
	Executable string
	Paths      []string `json:"path"`
	Builtins   []string `json:"builtins"`
}

func discoverPythonExecutable(rootPath string) string {
	candidates := make([]string, 0, 4)
	if rootPath != "" {
		candidates = append(candidates,
			filepath.Join(rootPath, ".venv", "bin", "python"),
			filepath.Join(rootPath, "venv", "bin", "python"),
		)
	}
	candidates = append(candidates, "python3", "python")
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if filepath.IsAbs(candidate) {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
			continue
		}
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}
	return ""
}

func discoverPythonEnv(rootPath string) pythonEnvInfo {
	python := discoverPythonExecutable(rootPath)
	if python == "" {
		return pythonEnvInfo{}
	}
	cmd := exec.Command(python, "-c", `import json, sys; print(json.dumps({"path": sys.path, "builtins": sorted(sys.builtin_module_names)}))`)
	if rootPath != "" {
		cmd.Dir = rootPath
	}
	output, err := cmd.Output()
	if err != nil {
		return pythonEnvInfo{}
	}
	var env pythonEnvInfo
	if err := json.Unmarshal(output, &env); err != nil {
		return pythonEnvInfo{}
	}
	env.Executable = python
	return env
}

func normalizeExternalSearchRoots(rootPath string, paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	roots := make([]string, 0, len(paths))
	rootAbs := ""
	if rootPath != "" {
		rootAbs, _ = filepath.Abs(rootPath)
	}
	for _, p := range paths {
		if p == "" {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if rootAbs != "" && abs == rootAbs {
			continue
		}
		info, err := os.Stat(abs)
		if err != nil || !info.IsDir() {
			continue
		}
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		roots = append(roots, abs)
	}
	return roots
}
