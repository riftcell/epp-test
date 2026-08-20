//go:build unit

package runner_test

import (
	"fmt"
	"os"

	"github.com/riftcell/epp-test/pkg/runner"
)

// ExampleRunScenario demonstrates loading and parsing a YAML scenario file.
// In a real test, pass *testing.T to RunScenario to execute the scenario against
// a Registrar implementation.
func ExampleRunScenario() {
	// Write a minimal scenario to a temp file for the example.
	content := `name: example
steps:
  - name: ping
    op: ping
`
	f, err := os.CreateTemp("", "scenario-*.yaml")
	if err != nil {
		panic(err)
	}
	defer os.Remove(f.Name()) //nolint:errcheck // best-effort remove in Example cleanup, see CONVENTIONS.md §3
	if _, err := f.WriteString(content); err != nil {
		panic(err)
	}
	if err := f.Close(); err != nil {
		panic(err)
	}

	sc, err := runner.LoadScenario(f.Name())
	if err != nil {
		panic(err)
	}
	fmt.Println(sc.Name)
	// Output: example
}
