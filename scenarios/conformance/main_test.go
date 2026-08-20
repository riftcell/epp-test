//go:build unit

// Package conformance_test contains the test harness for the prebuilt EPP/RRI
// conformance scenario library.
package conformance_test

import (
	"os"
	"testing"

	"go.uber.org/goleak"

	"github.com/riftcell/epp-test/pkg/report"
	"github.com/riftcell/epp-test/pkg/runner"
)

// TestMain runs all conformance scenarios, then flushes all report formats to
// the directory specified by TEST_REPORT_DIR (default: ./reports/).
//
// Format selection is controlled by TEST_REPORT_FORMATS (comma-separated;
// default: all four formats). This satisfies D-09 / RPT-05.
//
// goleak.VerifyTestMain ensures the mock server goroutines from each test are
// fully cleaned up before the process exits.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, goleak.Cleanup(func(exitCode int) {
		results := runner.Flush()
		dir := os.Getenv("TEST_REPORT_DIR")
		if dir == "" {
			dir = "./reports/"
		}
		formats := os.Getenv("TEST_REPORT_FORMATS")
		if formats == "" {
			formats = "junit,json,text,html"
		}
		if len(results) > 0 {
			_ = report.Write(dir, formats, results)
		}
		os.Exit(exitCode)
	}))
}
