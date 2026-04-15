package server

import (
	"io"
	"log"
	"runtime"
	"time"

	"rahu/lsp"
)

// BenchmarkConfig configures a benchmark run
type BenchmarkConfig struct {
	Dir       string
	Workers   int
	LogWriter io.Writer
	Progress  ProgressCallback

	// Cold-start simulation options
	MeasureEnvSetup   bool // Measure Python environment discovery (LSP cold-start)
	SimulateFirstFile bool // Simulate first file open after indexing
	VerboseEnv        bool // Log detailed Python environment info
}

// ProgressCallback receives progress updates during benchmarking
type ProgressCallback interface {
	OnPhaseStart(name string, totalItems int)
	OnPhaseProgress(processed, total int, currentFile string, duration time.Duration)
	OnPhaseEnd(name string, duration time.Duration, rounds int)
	OnLog(level, message string)
}

// NopProgressCallback is a no-op implementation for when no callback is needed
type NopProgressCallback struct{}

func (n *NopProgressCallback) OnPhaseStart(name string, totalItems int) {}
func (n *NopProgressCallback) OnPhaseProgress(processed, total int, currentFile string, duration time.Duration) {
}
func (n *NopProgressCallback) OnPhaseEnd(name string, duration time.Duration, rounds int) {}
func (n *NopProgressCallback) OnLog(level, message string)                                {}

// BenchmarkResult contains detailed timing and memory stats
type BenchmarkResult struct {
	Timestamp time.Time `json:"timestamp"`
	Directory string    `json:"directory"`
	GoVersion string    `json:"goVersion"`
	NumCPU    int       `json:"numCPU"`

	Stats struct {
		TotalFiles       int `json:"totalFiles"`
		WorkspaceModules int `json:"workspaceModules"`
		NonWorkspace     int `json:"nonWorkspace"`
	} `json:"stats"`

	Phases      []PhaseResult `json:"phases"`
	TotalTimeMs int64         `json:"totalTimeMs"`

	Memory struct {
		StartHeapMB uint64 `json:"startHeapMB"`
		PeakHeapMB  uint64 `json:"peakHeapMB"`
		EndHeapMB   uint64 `json:"endHeapMB"`
		NumGC       uint32 `json:"numGC"`
	} `json:"memory"`

	// Optional cold-start measurements (nil if not enabled)
	EnvironmentSetup *EnvSetupResult  `json:"environmentSetup,omitempty"`
	FirstFileOpen    *FirstFileResult `json:"firstFileOpen,omitempty"`

	Error string `json:"error,omitempty"`
}

// EnvSetupResult measures LSP initialization timing
type EnvSetupResult struct {
	// Timing breakdown
	PythonDiscoveryMs int64 `json:"pythonDiscoveryMs"`
	PythonQueryMs     int64 `json:"pythonQueryMs"`
	TypeshedInitMs    int64 `json:"typeshedInitMs"`
	BuiltinCacheMs    int64 `json:"builtinCacheMs"`
	TotalMs           int64 `json:"totalMs"`

	// Python environment details
	PythonVersion    string `json:"pythonVersion,omitempty"`
	PythonExecutable string `json:"pythonExecutable,omitempty"`
	SysPathCount     int    `json:"sysPathCount,omitempty"`
	BuiltinsCount    int    `json:"builtinsCount,omitempty"`
	TypeshedEnabled  bool   `json:"typeshedEnabled,omitempty"`
	CacheHit         bool   `json:"cacheHit,omitempty"`
	CacheValid       bool   `json:"cacheValid,omitempty"`
}

// FirstFileResult measures simulated textDocument/didOpen
type FirstFileResult struct {
	FileURI           string `json:"fileUri"`
	FileSizeBytes     int    `json:"fileSizeBytes"`
	LinesOfCode       int    `json:"linesOfCode"`
	IsWorkspaceModule bool   `json:"isWorkspaceModule"`
	ImportCount       int    `json:"importCount"`

	// Timing breakdown
	OpenDocumentMs    int64 `json:"openDocumentMs"`
	BaseAnalysisMs    int64 `json:"baseAnalysisMs"`
	PublishDiagsMs    int64 `json:"publishDiagsMs"`
	AsyncRefinementMs int64 `json:"asyncRefinementMs"`
	TotalToReadyMs    int64 `json:"totalToReadyMs"`

	// Results
	ParseErrorsCount int `json:"parseErrorsCount"`
	SemErrorsCount   int `json:"semErrorsCount"`
	TotalDiagsCount  int `json:"totalDiagsCount"`
}

// PhaseResult represents a single phase timing
type PhaseResult struct {
	Name       string `json:"name"`
	DurationMs int64  `json:"durationMs"`
	Order      int    `json:"order"`
	Count      int    `json:"count,omitempty"` // files processed or rounds
}

// NewBenchmarkServer creates a server for benchmarking (no LSP connection)
func NewBenchmarkServer() *Server {
	s := &Server{
		conn:                   nil, // No LSP connection needed
		docs:                   make(map[lsp.DocumentURI]*Document),
		debounce:               make(map[lsp.DocumentURI]*time.Timer),
		modulesByName:          make(map[string]ModuleFile),
		modulesByURI:           make(map[lsp.DocumentURI]ModuleFile),
		externalModulesByName:  make(map[string]ModuleFile),
		externalModulesByURI:   make(map[lsp.DocumentURI]ModuleFile),
		pythonBuiltinNames:     make(map[string]struct{}),
		pythonModuleInfoByName: make(map[string]pythonModuleInfo),
		moduleImportsByURI:     make(map[lsp.DocumentURI][]string),
		reverseDepsByModule:    make(map[string]map[lsp.DocumentURI]struct{}),
		buildingModules:        make(map[string]chan struct{}),
		openModuleCounts:       make(map[lsp.DocumentURI]int),
		moduleSnapshotsByName:  make(map[string]*ModuleSnapshot),
		moduleSnapshotsByURI:   make(map[lsp.DocumentURI]*ModuleSnapshot),
		snapshotLRU:            newSnapshotLRU(),
		maxCachedModules:       defaultMaxCachedModules,
		refIndex:               NewRefIndex(),
		pythonMethodCache:      make(map[string]pythonMethodInfo),
		scheduleAsync: func(fn func()) {
			go fn()
		},
		startup: &startupReadiness{
			priorityModuleNames: make(map[string]struct{}),
			priorityOpenURIs:    make(map[lsp.DocumentURI]struct{}),
			firstDiagAtByURI:    make(map[lsp.DocumentURI]time.Time),
			firstApplyAtByURI:   make(map[lsp.DocumentURI]time.Time),
		},
	}
	return s
}

// RunBenchmark executes the full indexing pipeline with detailed timing
func (s *Server) RunBenchmark(cfg BenchmarkConfig) (BenchmarkResult, error) {
	result := BenchmarkResult{
		Timestamp: time.Now(),
		Directory: cfg.Dir,
		GoVersion: runtime.Version(),
		NumCPU:    runtime.GOMAXPROCS(0),
	}

	// Setup logging
	if cfg.LogWriter != nil {
		oldOutput := log.Writer()
		log.SetOutput(cfg.LogWriter)
		defer log.SetOutput(oldOutput)
	}

	if cfg.Progress == nil {
		cfg.Progress = &NopProgressCallback{}
	}

	startTime := time.Now()
	startMem := getMemStats()

	// Track peak memory in background
	stopMemMonitor := make(chan struct{})
	var peakMem uint64
	go monitorMemory(&peakMem, 100*time.Millisecond, stopMemMonitor)

	phaseOrder := 1
	addPhase := func(name string, duration time.Duration, count int) {
		result.Phases = append(result.Phases, PhaseResult{
			Name:       name,
			DurationMs: duration.Milliseconds(),
			Order:      phaseOrder,
			Count:      count,
		})
		phaseOrder++
	}

	// Phase 0 (Optional): Environment Setup (LSP cold-start simulation)
	var modules []ModuleFile
	var err error
	if cfg.MeasureEnvSetup {
		cfg.Progress.OnPhaseStart("environmentSetup", 0)
		phaseStart := time.Now()

		envSetup := s.measureEnvironmentSetup(cfg.Dir, cfg.VerboseEnv)
		result.EnvironmentSetup = envSetup

		addPhase("environmentSetup", time.Since(phaseStart), 0)
		cfg.Progress.OnPhaseEnd("environmentSetup", time.Since(phaseStart), 0)
	} else {
		// Standard path: initialize minimal builtins for benchmark
		if err = s.loadBuiltinsForBenchmark(); err != nil {
			result.Error = err.Error()
			return result, err
		}
	}

	// Phase 1: Module Discovery
	cfg.Progress.OnPhaseStart("moduleDiscovery", 0)
	phaseStart := time.Now()

	modules, err = s.discoverModulesForBenchmark(cfg.Dir)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	result.Stats.TotalFiles = len(modules)
	addPhase("moduleDiscovery", time.Since(phaseStart), len(modules))
	cfg.Progress.OnPhaseEnd("moduleDiscovery", time.Since(phaseStart), 0)

	// Phase 2: Build Startup Bases
	cfg.Progress.OnPhaseStart("buildStartupBases", len(modules))
	phaseStart = time.Now()

	bases, err := s.buildStartupBasesWithProgress(modules, cfg.Progress)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	result.Stats.WorkspaceModules = len(bases)
	addPhase("buildStartupBases", time.Since(phaseStart), len(bases))
	cfg.Progress.OnPhaseEnd("buildStartupBases", time.Since(phaseStart), 0)

	// Phase 3: Import Surface Convergence
	cfg.Progress.OnPhaseStart("importSurface", len(bases))
	phaseStart = time.Now()

	rounds := 0
	surfaces, err := s.convergeImportSurfacesWithProgress(bases, &rounds, cfg.Progress)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	addPhase("importSurface", time.Since(phaseStart), rounds)
	cfg.Progress.OnPhaseEnd("importSurface", time.Since(phaseStart), rounds)

	// Phase 4: Final Snapshots
	cfg.Progress.OnPhaseStart("finalSnapshots", len(surfaces))
	phaseStart = time.Now()

	snapshots, err := s.buildFinalSnapshotsWithProgress(bases, surfaces, cfg.Progress)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	addPhase("finalSnapshots", time.Since(phaseStart), len(snapshots))
	cfg.Progress.OnPhaseEnd("finalSnapshots", time.Since(phaseStart), 0)

	// Phase 5: Reference Indexing
	cfg.Progress.OnPhaseStart("referenceIndexing", len(snapshots))
	phaseStart = time.Now()

	if err := s.buildReferenceIndexWithProgress(snapshots, cfg.Progress); err != nil {
		result.Error = err.Error()
		return result, err
	}

	addPhase("referenceIndexing", time.Since(phaseStart), len(snapshots))
	cfg.Progress.OnPhaseEnd("referenceIndexing", time.Since(phaseStart), 0)

	// Phase 6: Reverse Dependencies
	phaseStart = time.Now()
	s.rebuildReverseDeps()
	addPhase("reverseDeps", time.Since(phaseStart), 0)

	// Phase 7: Reanalyze Open Documents (none in benchmark mode)
	phaseStart = time.Now()
	// No open documents in benchmark mode
	addPhase("reanalyzeOpenDocs", time.Since(phaseStart), 0)

	// Phase 8 (Optional): First File Open Simulation
	if cfg.SimulateFirstFile && len(modules) > 0 {
		cfg.Progress.OnPhaseStart("firstFileOpen", 1)
		phaseStart := time.Now()

		firstFileResult := s.simulateFirstFileOpen(modules)
		result.FirstFileOpen = firstFileResult

		addPhase("firstFileOpen", time.Since(phaseStart), 1)
		cfg.Progress.OnPhaseEnd("firstFileOpen", time.Since(phaseStart), 0)
	}

	// Stop memory monitoring
	close(stopMemMonitor)

	// Final stats
	result.TotalTimeMs = time.Since(startTime).Milliseconds()
	endMem := getMemStats()

	result.Memory.StartHeapMB = startMem.HeapAlloc / 1024 / 1024
	result.Memory.PeakHeapMB = peakMem / 1024 / 1024
	result.Memory.EndHeapMB = endMem.HeapAlloc / 1024 / 1024
	result.Memory.NumGC = endMem.NumGC - startMem.NumGC

	return result, nil
}

func getMemStats() runtime.MemStats {
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	return m
}

func monitorMemory(peak *uint64, interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			if m.HeapAlloc > *peak {
				*peak = m.HeapAlloc
			}
		case <-stop:
			return
		}
	}
}
