package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"rahu/server"
)

// printHuman prints the benchmark results in human-readable format
func printHuman(result server.BenchmarkResult) {
	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4")).
		Background(lipgloss.Color("#FAFAFA")).
		Padding(1, 2).
		Width(70).
		Align(lipgloss.Center)

	separatorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4"))

	fmt.Println(separatorStyle.Render(strings.Repeat("═", 70)))
	fmt.Println(titleStyle.Render("🐍 Rahu Indexing Benchmark"))
	fmt.Println(separatorStyle.Render(strings.Repeat("═", 70)))
	fmt.Println()

	// Directory info
	fmt.Printf("Directory: %s\n", result.Directory)
	fmt.Printf("Files: %d (%d workspace, %d non-workspace)\n",
		result.Stats.TotalFiles,
		result.Stats.WorkspaceModules,
		result.Stats.NonWorkspace)
	fmt.Printf("CPU: %d cores | Go: %s\n", result.NumCPU, result.GoVersion)

	// Python environment info (if measured)
	if result.EnvironmentSetup != nil && result.EnvironmentSetup.PythonExecutable != "" {
		env := result.EnvironmentSetup
		fmt.Printf("Python: %s (%s, %d sys.path entries, %d builtins)\n",
			env.PythonExecutable, env.PythonVersion, env.SysPathCount, env.BuiltinsCount)
	}
	fmt.Println()

	// Environment Setup section (if measured)
	if result.EnvironmentSetup != nil {
		env := result.EnvironmentSetup
		fmt.Println("Environment Setup (Cold-Start):")
		fmt.Println(separatorStyle.Render(strings.Repeat("─", 70)))

		envPhases := []struct {
			name     string
			duration int64
		}{
			{"Python Discovery", env.PythonDiscoveryMs},
			{"Python Query", env.PythonQueryMs},
			{"Typeshed Loader", env.TypeshedInitMs},
			{"Builtin Cache", env.BuiltinCacheMs},
		}

		for _, phase := range envPhases {
			fmt.Printf("  %-25s %10s\n", phase.name, formatDuration(phase.duration))
		}

		fmt.Println(separatorStyle.Render(strings.Repeat("─", 70)))
		totalStr := lipgloss.NewStyle().Bold(true).Render(formatDuration(env.TotalMs))
		fmt.Printf("  %-25s %10s\n", "Subtotal:", totalStr)

		// Extra details
		if env.PythonExecutable != "" {
			cacheStatus := "miss"
			if env.CacheHit {
				if env.CacheValid {
					cacheStatus = "hit (valid)"
				} else {
					cacheStatus = "hit (stale)"
				}
			}
			fmt.Printf("  Cache: %s | Typeshed: %v\n", cacheStatus, env.TypeshedEnabled)
		}
		fmt.Println()
	}

	// First File Open section (if simulated)
	if result.FirstFileOpen != nil {
		ff := result.FirstFileOpen
		fmt.Println("First File Open Simulation:")
		fmt.Println(separatorStyle.Render(strings.Repeat("─", 70)))

		// File info
		fileName := ff.FileURI
		if idx := strings.LastIndex(fileName, "/"); idx != -1 {
			fileName = fileName[idx+1:]
		}
		sizeKB := float64(ff.FileSizeBytes) / 1024
		fmt.Printf("  File: %s (%.1f KB, %d lines, %d imports)\n",
			fileName, sizeKB, ff.LinesOfCode, ff.ImportCount)
		fmt.Println(separatorStyle.Render(strings.Repeat("─", 70)))

		// Timing breakdown
		fmt.Printf("  %-25s %10s\n", "Open Document", formatDuration(ff.OpenDocumentMs))
		fmt.Printf("  %-25s %10s\n", "Base Analysis", formatDuration(ff.BaseAnalysisMs))
		fmt.Printf("  %-25s %10s\n", "Publish Diagnostics", formatDuration(ff.PublishDiagsMs))
		if ff.IsWorkspaceModule {
			fmt.Printf("  %-25s %10s\n", "Async Refinement", formatDuration(ff.AsyncRefinementMs))
		}

		fmt.Println(separatorStyle.Render(strings.Repeat("─", 70)))
		totalStr := lipgloss.NewStyle().Bold(true).Render(formatDuration(ff.TotalToReadyMs))
		fmt.Printf("  %-25s %10s\n", "Time to First Ready:", totalStr)

		// Diagnostics info
		if ff.TotalDiagsCount > 0 {
			fmt.Printf("  Diagnostics: %d (%d parse, %d semantic)\n",
				ff.TotalDiagsCount, ff.ParseErrorsCount, ff.SemErrorsCount)
		} else {
			fmt.Println("  Diagnostics: none")
		}
		fmt.Println()
	}

	// Phase breakdown table
	fmt.Println("Phase Breakdown:")
	fmt.Println(separatorStyle.Render(strings.Repeat("─", 70)))

	// Table header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	fmt.Printf("  %s %s %s %s\n",
		headerStyle.Render(" # "),
		headerStyle.Render(" Phase "),
		headerStyle.Render(" Time "),
		headerStyle.Render(" % "))
	fmt.Println(separatorStyle.Render(strings.Repeat("─", 70)))

	// Calculate total time
	totalMs := result.TotalTimeMs
	if totalMs == 0 {
		for _, phase := range result.Phases {
			totalMs += phase.DurationMs
		}
	}

	// Find the bottleneck (longest phase)
	var maxDuration int64
	var bottleneckIdx int
	for i, phase := range result.Phases {
		if phase.DurationMs > maxDuration {
			maxDuration = phase.DurationMs
			bottleneckIdx = i
		}
	}

	// Print phases
	for i, phase := range result.Phases {
		num := fmt.Sprintf("  %d", phase.Order)
		name := formatPhaseName(phase.Name)
		duration := formatDuration(phase.DurationMs)
		percent := float64(phase.DurationMs) / float64(totalMs) * 100
		percentStr := fmt.Sprintf(" %5.1f%%", percent)

		// Highlight bottleneck
		if i == bottleneckIdx {
			name += " ← BOTTLENECK"
			name = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E06C75")).
				Render(name)
		}

		fmt.Printf("  %s %-25s %10s %s\n", num, name, duration, percentStr)

		// Show count info if available
		if phase.Count > 0 {
			if phase.Name == "importSurface" {
				fmt.Printf("     (%d rounds)\n", phase.Count)
			} else {
				fmt.Printf("     (%d files)\n", phase.Count)
			}
		}
	}

	fmt.Println(separatorStyle.Render(strings.Repeat("─", 70)))
	totalStr := formatDuration(result.TotalTimeMs)
	fmt.Printf("  Total: %s\n", lipgloss.NewStyle().Bold(true).Render(totalStr))
	fmt.Println()

	// Memory profile
	fmt.Println("Memory Profile:")
	memStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#56B6C2"))

	fmt.Printf("  Start Heap:  %s\n", memStyle.Render(fmt.Sprintf("%d MB", result.Memory.StartHeapMB)))
	fmt.Printf("  Peak Heap:   %s", memStyle.Render(fmt.Sprintf("%d MB", result.Memory.PeakHeapMB)))
	if result.Memory.PeakHeapMB > result.Memory.StartHeapMB {
		delta := result.Memory.PeakHeapMB - result.Memory.StartHeapMB
		fmt.Printf(" (+%d MB)", delta)
	}
	fmt.Println()
	fmt.Printf("  End Heap:    %s\n", memStyle.Render(fmt.Sprintf("%d MB", result.Memory.EndHeapMB)))
	fmt.Printf("  GC Cycles:   %d\n", result.Memory.NumGC)
	fmt.Println()
}

// formatDuration converts milliseconds to human-readable string
func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	if ms < 60000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	mins := ms / 60000
	secs := (ms % 60000) / 1000
	if secs == 0 {
		return fmt.Sprintf("%dm", mins)
	}
	return fmt.Sprintf("%dm %ds", mins, secs)
}
