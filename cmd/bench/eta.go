package main

import (
	"fmt"
	"time"
)

// ETACalculator estimates remaining time based on current progress
type ETACalculator struct {
	startTime      time.Time
	totalItems     int
	processedItems int
	samples        []timeSample
	maxSamples     int
}

type timeSample struct {
	items     int
	timestamp time.Time
}

// NewETACalculator creates a new ETA calculator
func NewETACalculator(total int) *ETACalculator {
	return &ETACalculator{
		startTime:  time.Now(),
		totalItems: total,
		maxSamples: 10,
	}
}

// Update updates the progress
func (e *ETACalculator) Update(processed int) {
	e.processedItems = processed

	sample := timeSample{
		items:     processed,
		timestamp: time.Now(),
	}

	e.samples = append(e.samples, sample)
	if len(e.samples) > e.maxSamples {
		e.samples = e.samples[1:]
	}
}

// Remaining calculates the estimated remaining time
func (e *ETACalculator) Remaining() time.Duration {
	if e.processedItems >= e.totalItems {
		return 0
	}

	if len(e.samples) < 2 {
		// Not enough data, use simple linear projection
		elapsed := time.Since(e.startTime)
		if e.processedItems == 0 {
			return 0
		}
		rate := float64(e.processedItems) / elapsed.Seconds()
		remainingItems := e.totalItems - e.processedItems
		return time.Duration(float64(remainingItems) / rate * float64(time.Second))
	}

	// Use recent samples for better estimate
	first := e.samples[0]
	last := e.samples[len(e.samples)-1]

	itemsInWindow := last.items - first.items
	timeWindow := last.timestamp.Sub(first.timestamp)

	if itemsInWindow == 0 || timeWindow == 0 {
		return e.linearEstimate()
	}

	rate := float64(itemsInWindow) / timeWindow.Seconds()
	remainingItems := e.totalItems - e.processedItems
	remainingSeconds := float64(remainingItems) / rate

	return time.Duration(remainingSeconds * float64(time.Second))
}

func (e *ETACalculator) linearEstimate() time.Duration {
	elapsed := time.Since(e.startTime)
	if e.processedItems == 0 {
		return 0
	}
	rate := float64(e.processedItems) / elapsed.Seconds()
	remainingItems := e.totalItems - e.processedItems
	return time.Duration(float64(remainingItems) / rate * float64(time.Second))
}

// Elapsed returns the elapsed time
func (e *ETACalculator) Elapsed() time.Duration {
	return time.Since(e.startTime)
}

// FormatDuration formats a duration in human-readable form
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.0fms", float64(d.Milliseconds()))
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	mins := int(d.Minutes())
	secs := int(d.Seconds()) % 60
	if secs == 0 {
		return fmt.Sprintf("%dm", mins)
	}
	return fmt.Sprintf("%dm %ds", mins, secs)
}
