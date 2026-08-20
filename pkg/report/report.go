package report

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/riftcell/epp-test/pkg/runner"
)

// Write generates test result reports in the selected formats and writes them
// to dir.
//
// formats is a comma-separated list of format names: "junit", "json", "text",
// "html". An empty string defaults to all four formats ("junit,json,text,html").
// This matches the TEST_REPORT_FORMATS env-var convention from D-09 — the env
// read itself lives in 04-05's TestMain; Write receives the already-resolved
// values.
//
// Files written:
//
//	junit  → <dir>/junit.xml
//	json   → <dir>/report.json
//	text   → <dir>/report.txt
//	html   → <dir>/report.html
//
// Unknown format tokens cause a wrapped error to be returned via errors.Join.
func Write(dir, formats string, results []runner.ScenarioResult) error {
	if formats == "" {
		formats = "junit,json,text,html"
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("report: create output dir %q: %w", dir, err)
	}

	var errs []error
	for _, f := range strings.Split(formats, ",") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}

		var (
			filename string
			writeErr error
		)

		switch f {
		case "junit":
			filename = "junit.xml"
			writeErr = writeFile(filepath.Join(dir, filename), func(fh *os.File) error {
				return writeJUnit(fh, results)
			})
		case "json":
			filename = "report.json"
			writeErr = writeFile(filepath.Join(dir, filename), func(fh *os.File) error {
				return writeJSON(fh, results)
			})
		case "text":
			filename = "report.txt"
			writeErr = writeFile(filepath.Join(dir, filename), func(fh *os.File) error {
				return writeText(fh, results)
			})
		case "html":
			filename = "report.html"
			writeErr = writeFile(filepath.Join(dir, filename), func(fh *os.File) error {
				return writeHTML(fh, results)
			})
		default:
			errs = append(errs, fmt.Errorf("report: unknown format %q", f))
			continue
		}

		if writeErr != nil {
			errs = append(errs, fmt.Errorf("report: write %s: %w", filename, writeErr))
		}
	}

	return errors.Join(errs...)
}

// writeFile creates path, calls fn with the open file, and closes it.
func writeFile(path string, fn func(*os.File) error) error {
	fh, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer fh.Close() //nolint:errcheck // best-effort close in defer; write errors are captured by fn return value, see CONVENTIONS.md §3
	return fn(fh)
}
