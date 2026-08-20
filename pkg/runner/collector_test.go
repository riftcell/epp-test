//go:build unit

package runner

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// resetCollector resets package-level state between tests.
func resetCollector() {
	mu.Lock()
	results = nil
	mu.Unlock()
}

func TestFlushEmpty(t *testing.T) {
	resetCollector()
	got := Flush()
	require.Len(t, got, 0, "Flush on empty collector must return empty slice")
}

func TestAppendAndFlushOrder(t *testing.T) {
	resetCollector()

	r1 := ScenarioResult{Name: "scenario-1", Registrar: "internetx", Passed: true, Elapsed: 100 * time.Millisecond}
	r2 := ScenarioResult{Name: "scenario-2", Registrar: "nicat", Passed: false, Elapsed: 200 * time.Millisecond}

	Append(r1)
	Append(r2)

	got := Flush()
	require.Len(t, got, 2, "Flush must return exactly 2 results")
	require.Equal(t, r1, got[0], "first result must match r1")
	require.Equal(t, r2, got[1], "second result must match r2")
}

func TestFlushResetsCollector(t *testing.T) {
	resetCollector()

	r := ScenarioResult{Name: "scenario-reset", Passed: true}
	Append(r)

	first := Flush()
	require.Len(t, first, 1, "first Flush must return 1 result")

	second := Flush()
	require.Len(t, second, 0, "second Flush immediately after must return empty slice")
}

func TestConcurrentAppend(t *testing.T) {
	resetCollector()

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			Append(ScenarioResult{Name: "concurrent", Passed: true})
		}()
	}

	wg.Wait()

	got := Flush()
	require.Len(t, got, goroutines, "all concurrent Appends must be captured without data races or lost writes")
}
