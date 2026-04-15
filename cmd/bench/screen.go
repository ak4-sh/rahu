package main

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"charm.land/lipgloss/v2"
)

// AltScreen manages the alternate terminal screen buffer with live stats
type AltScreen struct {
	mu     sync.RWMutex
	width  int
	height int

	projectPath string
	totalFiles  int
	workers     int

	currentPhase     string
	phaseStart       time.Time
	phaseTotal       int
	phaseDone        int
	currentFile      string
	currentFileStart time.Time

	eta *ETACalculator

	// Stats
	memoryMB      uint64
	heapDeltaMB   int64
	numGC         uint32
	activeWorkers int

	// Logs
	logs    []LogEntry
	maxLogs int
}

// LogEntry represents a single log line
type LogEntry struct {
	Time    time.Time
	Level   string
	Message string
}

// NewAltScreen creates a new alternate screen manager
func NewAltScreen(projectPath string, totalFiles, workers int) *AltScreen {
	if workers == 0 {
		workers = runtime.GOMAXPROCS(0)
	}

	return &AltScreen{
		projectPath: projectPath,
		totalFiles:  totalFiles,
		workers:     workers,
		maxLogs:     10,
		logs:        make([]LogEntry, 0, 10),
		width:       70,
	}
}

// Enter switches to the alternate screen buffer
func (a *AltScreen) Enter() {
	// ANSI escape sequences for alternate screen
	fmt.Print("\x1b[?1049h") // Enter alternate buffer
	fmt.Print("\x1b[2J")     // Clear screen
	fmt.Print("\x1b[H")      // Move cursor to top-left
	fmt.Print("\x1b[?25l")   // Hide cursor

	// Initial render
	a.Render()
}

// Exit returns to the normal screen buffer
func (a *AltScreen) Exit() {
	fmt.Print("\x1b[?25h")   // Show cursor
	fmt.Print("\x1b[?1049l") // Exit alternate buffer
}

// StartPhase begins tracking a new phase
func (a *AltScreen) StartPhase(name string, total int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.currentPhase = name
	a.phaseStart = time.Now()
	a.phaseTotal = total
	a.phaseDone = 0
	a.eta = NewETACalculator(total)
	a.currentFile = ""

	a.Render()
}

// UpdateProgress updates the current progress
func (a *AltScreen) UpdateProgress(done int, currentFile string, duration interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.phaseDone = done
	a.currentFile = currentFile
	a.currentFileStart = time.Now()
	a.eta.Update(done)

	// Auto-detect slow files (>300ms)
	if dur, ok := duration.(interface{ Milliseconds() int64 }); ok {
		if dur.Milliseconds() > 300 {
			a.addLogInternal("WARN", fmt.Sprintf("Slow file: %s (%dms)", currentFile, dur.Milliseconds()))
		}
	}

	a.Render()
}

// UpdateStats updates memory and worker stats
func (a *AltScreen) UpdateStats(memoryMB uint64, heapDeltaMB int64, numGC uint32, activeWorkers int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.memoryMB = memoryMB
	a.heapDeltaMB = heapDeltaMB
	a.numGC = numGC
	a.activeWorkers = activeWorkers

	a.Render()
}

// AddLog adds a log entry
func (a *AltScreen) AddLog(level, message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.addLogInternal(level, message)
	a.Render()
}

// addLogInternal adds a log entry without locking or rendering
func (a *AltScreen) addLogInternal(level, message string) {
	entry := LogEntry{
		Time:    time.Now(),
		Level:   level,
		Message: message,
	}

	a.logs = append(a.logs, entry)
	if len(a.logs) > a.maxLogs {
		a.logs = a.logs[1:]
	}
}

// Render redraws the entire screen
func (a *AltScreen) Render() {
	// Move to top-left and clear
	fmt.Print("\x1b[H")
	fmt.Print("\x1b[J")

	// Build the UI
	ui := a.buildUI()
	fmt.Print(ui)
}

// buildUI constructs the full terminal UI
func (a *AltScreen) buildUI() string {
	var sections []string

	// Header
	sections = append(sections, a.buildHeader())

	// Progress section
	if a.currentPhase != "" {
		sections = append(sections, a.buildProgressSection())
	}

	// Stats section
	sections = append(sections, a.buildStatsSection())

	// Logs section
	if len(a.logs) > 0 {
		sections = append(sections, a.buildLogsSection())
	}

	// Footer
	sections = append(sections, a.buildFooter())

	return strings.Join(sections, "\n")
}

// buildHeader creates the title and info line
func (a *AltScreen) buildHeader() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4")).
		Background(lipgloss.Color("#FAFAFA")).
		Padding(1, 2).
		Width(a.width).
		Align(lipgloss.Center)

	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ABB2BF")).
		Width(a.width).
		Align(lipgloss.Center)

	title := titleStyle.Render("🐍 Rahu Benchmark")

	info := fmt.Sprintf("%s • %d files • %d workers • %s",
		truncatePath(a.projectPath, 30),
		a.totalFiles,
		a.workers,
		runtime.Version())
	info = infoStyle.Render(info)

	return title + "\n" + info
}

// buildProgressSection creates the progress bar and file info
func (a *AltScreen) buildProgressSection() string {
	percent := 0.0
	if a.phaseTotal > 0 {
		percent = float64(a.phaseDone) / float64(a.phaseTotal) * 100
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(1, 2).
		Width(a.width)

	// Phase name
	phaseStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#25A065")).
		Background(lipgloss.Color("#E8F5E9")).
		Padding(0, 1)

	phaseName := formatPhaseName(a.currentPhase)
	phase := phaseStyle.Render("▶ " + phaseName)

	// Progress bar
	barWidth := a.width - 15
	filled := int(percent / 100 * float64(barWidth))

	barStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3B4252"))

	bar := barStyle.Render(strings.Repeat("━", filled)) +
		emptyStyle.Render(strings.Repeat("─", barWidth-filled))

	percentStr := fmt.Sprintf("%3.0f%%", percent)
	progressLine := bar + " " + percentStr

	// ETA
	elapsed := time.Since(a.phaseStart)
	remaining := a.eta.Remaining()

	etaStyle := lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color("#98C379"))

	etaStr := fmt.Sprintf("⏱️  %s elapsed", FormatDuration(elapsed))
	if remaining > 0 {
		etaStr += fmt.Sprintf(" • ~%s remaining", FormatDuration(remaining))
	}
	eta := etaStyle.Render(etaStr)

	// Current file
	var fileStr string
	if a.currentFile != "" {
		fileStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#C678DD"))

		fileTime := time.Since(a.currentFileStart)
		fileStr = fmt.Sprintf("📄 %s (%s)",
			fileStyle.Render(truncatePath(a.currentFile, 35)),
			FormatDuration(fileTime))
	}

	// Count
	countStr := fmt.Sprintf("✅ %d / %d", a.phaseDone, a.phaseTotal)

	content := phase + "\n\n" + progressLine + "\n" + eta
	if fileStr != "" {
		content += "\n\n" + fileStr + "\n" + countStr
	} else {
		content += "\n\n" + countStr
	}

	return boxStyle.Render(content)
}

// buildStatsSection creates the stats grid
func (a *AltScreen) buildStatsSection() string {
	statsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#3B4252")).
		Padding(1, 2).
		Width(a.width)

	cellWidth := (a.width - 6) / 4

	cellStyle := lipgloss.NewStyle().
		Width(cellWidth).
		Align(lipgloss.Center)

	memoryCell := cellStyle.Render(fmt.Sprintf("Memory\n%d MB", a.memoryMB))
	heapCell := cellStyle.Render(fmt.Sprintf("Heap Δ\n%d MB", a.heapDeltaMB))
	gcCell := cellStyle.Render(fmt.Sprintf("GC\n%d", a.numGC))
	workersCell := cellStyle.Render(fmt.Sprintf("Workers\n%d/%d", a.activeWorkers, a.workers))

	grid := lipgloss.JoinHorizontal(lipgloss.Top, memoryCell, heapCell, gcCell, workersCell)

	return statsStyle.Render(grid)
}

// buildLogsSection creates the recent logs display
func (a *AltScreen) buildLogsSection() string {
	sepStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#5C6370"))

	content := sepStyle.Render("─ Recent Activity ─") + "\n"

	for _, log := range a.logs {
		content += formatLogEntry(log) + "\n"
	}

	return lipgloss.NewStyle().
		Padding(0, 1).
		Render(content)
}

// buildFooter creates the footer with instructions
func (a *AltScreen) buildFooter() string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#5C6370")).
		Italic(true).
		Render("Press Ctrl+C to cancel")
}

// formatLogEntry formats a single log entry
func formatLogEntry(entry LogEntry) string {
	timeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#5C6370"))

	var levelStyled string
	switch entry.Level {
	case "INFO":
		levelStyled = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#61AFEF")).
			Render("•")
	case "WARN":
		levelStyled = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5C07B")).
			Render("⚠")
	case "ERROR":
		levelStyled = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#E06C75")).
			Render("✖")
	default:
		levelStyled = "•"
	}

	return fmt.Sprintf("%s %s %s",
		timeStyle.Render(entry.Time.Format("15:04:05")),
		levelStyled,
		entry.Message)
}

// truncatePath truncates a path if too long
func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	if maxLen <= 3 {
		return path[len(path)-maxLen:]
	}
	return "..." + path[len(path)-maxLen+3:]
}
