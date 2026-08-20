package report

import (
	"fmt"
	"io"

	"github.com/riftcell/epp-test/pkg/runner"
)

// writeText writes a human-readable plain-text report to w.
//
// Each scenario prints a status line ([PASS]/[FAIL]), name, registrar, and
// elapsed time. Failing steps print an indented detail line. A summary line
// is appended at the end.
func writeText(w io.Writer, results []runner.ScenarioResult) error {
	passed := 0
	failed := 0

	for _, r := range results {
		status := "PASS"
		if !r.Passed {
			status = "FAIL"
		}

		if _, err := fmt.Fprintf(w, "[%s] %s (%s) %.3fs\n",
			status, r.Name, r.Registrar, r.Elapsed.Seconds()); err != nil {
			return fmt.Errorf("report: write text: %w", err)
		}

		for _, step := range r.Steps {
			if !step.Passed {
				if _, err := fmt.Fprintf(w, "  FAIL step=%s op=%s err=%s\n",
					step.Name, step.Op, step.Err); err != nil {
					return fmt.Errorf("report: write text step: %w", err)
				}
			}
		}

		if r.Passed {
			passed++
		} else {
			failed++
		}
	}

	if _, err := fmt.Fprintf(w, "Total: %d  Passed: %d  Failed: %d\n",
		len(results), passed, failed); err != nil {
		return fmt.Errorf("report: write text summary: %w", err)
	}

	return nil
}
