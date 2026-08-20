package report

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/riftcell/epp-test/pkg/runner"
)

// writeJSON writes all results to w as an indented JSON array of
// runner.ScenarioResult objects.
func writeJSON(w io.Writer, results []runner.ScenarioResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(results); err != nil {
		return fmt.Errorf("report: write json: %w", err)
	}
	return nil
}
