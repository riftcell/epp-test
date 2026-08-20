package runner

import (
	"sync"
	"time"
)

// ScenarioResult is the structured result of one scenario run.
//
// It is appended to the package-level collector by RunScenario and consumed by
// report formatters (JUnit, JSON, text, HTML) after m.Run() completes.
type ScenarioResult struct {
	Name      string
	Registrar string
	File      string
	RFC       string
	Passed    bool
	Elapsed   time.Duration
	Steps     []StepResult
}

// StepResult captures the outcome of a single step within a scenario.
type StepResult struct {
	Name    string
	Op      string
	Passed  bool
	Elapsed time.Duration
	Err     string // empty when the step passed
}

// Package-level collector guarded by mu.
//
// This is intentional package-level state: the collector is designed for exactly
// one global Flush per TestMain, mirroring the same pattern used by
// go.uber.org/goleak for global goroutine tracking (RESEARCH Pattern 5).
// All writes go through Append; all reads go through Flush.
var (
	mu      sync.Mutex
	results []ScenarioResult
)

// Append adds r to the global collector in a thread-safe manner.
// It is safe to call from multiple goroutines concurrently.
func Append(r ScenarioResult) {
	mu.Lock()
	defer mu.Unlock()
	results = append(results, r)
}

// Flush returns all collected results and resets the collector to empty.
//
// The returned slice is a copy — callers cannot mutate internal state.
// A second Flush immediately after the first returns an empty slice.
// It is safe to call from multiple goroutines concurrently.
func Flush() []ScenarioResult {
	mu.Lock()
	defer mu.Unlock()
	out := make([]ScenarioResult, len(results))
	copy(out, results)
	results = nil
	return out
}
