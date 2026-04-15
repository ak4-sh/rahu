package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"rahu/server"
)

type Config struct {
	Dir               string
	Workers           int
	Human             bool
	MeasureEnvSetup   bool
	SimulateFirstFile bool
	VerboseEnv        bool
}

func parseFlags() Config {
	var cfg Config
	var showHelp bool

	flag.StringVar(&cfg.Dir, "dir", ".", "Directory to analyze")
	flag.IntVar(&cfg.Workers, "workers", 0, "Number of workers (0 = auto, default 0)")
	flag.BoolVar(&cfg.Human, "h", false, "Human readable output (default is JSON)")
	flag.BoolVar(&cfg.MeasureEnvSetup, "env-setup", false, "Measure Python environment discovery (LSP cold-start simulation)")
	flag.BoolVar(&cfg.SimulateFirstFile, "first-file", false, "Simulate first file open after indexing (measures time-to-first-diagnostics)")
	flag.BoolVar(&cfg.VerboseEnv, "verbose-env", false, "Log detailed Python environment info (version, sys.path count, etc)")
	flag.BoolVar(&showHelp, "help", false, "Show help message")
	flag.Parse()

	// Show help and exit
	if showHelp {
		printHelp()
		os.Exit(0)
	}

	// Validate directory
	if stat, err := os.Stat(cfg.Dir); err != nil || !stat.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: %s is not a valid directory\n", cfg.Dir)
		os.Exit(1)
	}

	return cfg
}

func printHelp() {
	fmt.Println("Rahu Benchmark Tool")
	fmt.Println()
	fmt.Println("Usage: rahu-bench [options]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -dir string       Directory to analyze (default \".\")")
	fmt.Println("  -workers int      Number of workers (0 = auto, default 0)")
	fmt.Println("  -h                Human readable output (default is JSON)")
	fmt.Println("  -env-setup        Measure Python environment discovery (LSP cold-start)")
	fmt.Println("  -first-file       Simulate first file open (time-to-first-diagnostics)")
	fmt.Println("  -verbose-env      Log detailed Python environment info")
	fmt.Println("  --help            Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  rahu-bench -dir ./my-project              # Output JSON results")
	fmt.Println("  rahu-bench -dir ./my-project -h           # Output human-readable results")
	fmt.Println("  rahu-bench -dir ./my-project -env-setup -first-file -h  # Full cold-start simulation")
	fmt.Println("  rahu-bench --help                         # Show this help")
}

type benchmarkResultWithError struct {
	result server.BenchmarkResult
	err    error
}

func main() {
	cfg := parseFlags()

	srv := server.NewBenchmarkServer()

	// Human mode: skip alternate screen, run synchronously
	if cfg.Human {
		result, err := srv.RunBenchmark(server.BenchmarkConfig{
			Dir:               cfg.Dir,
			Workers:           cfg.Workers,
			Progress:          nil, // No progress callback in human mode
			MeasureEnvSetup:   cfg.MeasureEnvSetup,
			SimulateFirstFile: cfg.SimulateFirstFile,
			VerboseEnv:        cfg.VerboseEnv,
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		printHuman(result)
		return
	}

	// JSON mode: use alternate screen with progress display
	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Enter alternate screen
	screen := NewAltScreen(cfg.Dir, 0, cfg.Workers)
	screen.Enter()

	// Ensure we always exit alternate screen
	cancelled := false
	defer func() {
		screen.Exit()
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Panic: %v\n", r)
			os.Exit(1)
		}
		if cancelled {
			os.Exit(130) // Standard Ctrl+C exit code
		}
	}()

	// Setup progress callback
	progress := &ScreenProgressCallback{
		screen: screen,
	}

	// Run benchmark in goroutine
	resultChan := make(chan benchmarkResultWithError, 1)
	go func() {
		result, err := srv.RunBenchmark(server.BenchmarkConfig{
			Dir:               cfg.Dir,
			Workers:           cfg.Workers,
			Progress:          progress,
			MeasureEnvSetup:   cfg.MeasureEnvSetup,
			SimulateFirstFile: cfg.SimulateFirstFile,
			VerboseEnv:        cfg.VerboseEnv,
		})
		resultChan <- benchmarkResultWithError{result, err}
	}()

	// Handle completion or interruption
	select {
	case <-sigChan:
		cancelled = true
		screen.AddLog("WARN", "Interrupted by user - exiting...")
		screen.Render()
		return

	case rwe := <-resultChan:
		if rwe.err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", rwe.err)
			os.Exit(1)
		}

		printJSON(rwe.result)
	}
}

func printJSON(result server.BenchmarkResult) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.Encode(result)
}

// ScreenProgressCallback implements server.ProgressCallback
type ScreenProgressCallback struct {
	screen *AltScreen
}

func (s *ScreenProgressCallback) OnPhaseStart(name string, totalItems int) {
	s.screen.StartPhase(name, totalItems)
}

func (s *ScreenProgressCallback) OnPhaseProgress(processed, total int, currentFile string, duration time.Duration) {
	s.screen.UpdateProgress(processed, currentFile, duration)
}

func (s *ScreenProgressCallback) OnPhaseEnd(name string, duration time.Duration, rounds int) {
	s.screen.AddLog("INFO", fmt.Sprintf("Completed %s in %s", formatPhaseName(name), FormatDuration(duration)))
}

func (s *ScreenProgressCallback) OnLog(level, message string) {
	s.screen.AddLog(level, message)
}

func formatPhaseName(name string) string {
	// Simple conversion: camelCase to Title Case
	if name == "" {
		return ""
	}

	// Capitalize first letter
	result := string(name[0])
	if len(name) > 1 {
		if name[0] >= 'a' && name[0] <= 'z' {
			result = string(name[0] - 32) // Convert to uppercase
		} else {
			result = string(name[0])
		}
		result += name[1:]
	}

	// Add spaces before capitals
	var spaced string
	for i, ch := range result {
		if i > 0 && ch >= 'A' && ch <= 'Z' {
			spaced += " " + string(ch)
		} else {
			spaced += string(ch)
		}
	}

	return spaced
}
