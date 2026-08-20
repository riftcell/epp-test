package report

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/riftcell/epp-test/pkg/runner"
)

// xmlTestSuites is the root element of a JUnit XML report.
// Exact struct tags from RESEARCH Pattern 6.
type xmlTestSuites struct {
	XMLName  xml.Name       `xml:"testsuites"`
	Name     string         `xml:"name,attr,omitempty"`
	Tests    int            `xml:"tests,attr"`
	Failures int            `xml:"failures,attr"`
	Errors   int            `xml:"errors,attr"`
	Time     string         `xml:"time,attr"` // seconds, e.g. "1.234"
	Suites   []xmlTestSuite `xml:"testsuite"`
}

// xmlTestSuite represents one scenario's results.
type xmlTestSuite struct {
	XMLName   xml.Name      `xml:"testsuite"`
	Name      string        `xml:"name,attr"`
	Tests     int           `xml:"tests,attr"`
	Failures  int           `xml:"failures,attr"`
	Errors    int           `xml:"errors,attr"`
	Skipped   int           `xml:"skipped,attr"`
	Time      string        `xml:"time,attr"`
	Timestamp string        `xml:"timestamp,attr"` // ISO 8601: "2026-06-28T15:04:05"
	Cases     []xmlTestCase `xml:"testcase"`
}

// xmlTestCase represents one step's results.
type xmlTestCase struct {
	XMLName   xml.Name    `xml:"testcase"`
	Name      string      `xml:"name,attr"`
	Classname string      `xml:"classname,attr"` // scenario name with dots
	Time      string      `xml:"time,attr"`      // seconds, e.g. "0.123"
	Failure   *xmlFailure `xml:"failure,omitempty"`
}

// xmlFailure represents a step assertion failure.
type xmlFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Body    string `xml:",chardata"`
}

// writeJUnit writes a JUnit XML report for all results to w.
//
// The output uses the three-level testsuites > testsuite > testcase hierarchy
// with UTC timestamps (no timezone, no fractional seconds) per RESEARCH Pitfall 4.
func writeJUnit(w io.Writer, results []runner.ScenarioResult) error {
	var totalTests, totalFailures int
	var totalElapsed float64

	// Capture a single timestamp for all suites (consistent report time).
	now := time.Now().UTC().Format("2006-01-02T15:04:05")

	suites := make([]xmlTestSuite, 0, len(results))
	for _, r := range results {
		suiteFailures := 0
		cases := make([]xmlTestCase, 0, len(r.Steps))

		for _, step := range r.Steps {
			tc := xmlTestCase{
				Name:      step.Name,
				Classname: "conformance." + sanitizeClassname(r.Name),
				Time:      fmt.Sprintf("%.3f", step.Elapsed.Seconds()),
			}
			if !step.Passed {
				tc.Failure = &xmlFailure{
					Message: step.Err,
					Type:    "AssertionError",
					Body:    "step: " + step.Name + " op: " + step.Op,
				}
				suiteFailures++
			}
			cases = append(cases, tc)
		}

		suite := xmlTestSuite{
			Name:      r.Name,
			Tests:     len(r.Steps),
			Failures:  suiteFailures,
			Errors:    0,
			Skipped:   0,
			Time:      fmt.Sprintf("%.3f", r.Elapsed.Seconds()),
			Timestamp: now,
			Cases:     cases,
		}
		suites = append(suites, suite)

		totalTests += len(r.Steps)
		totalFailures += suiteFailures
		totalElapsed += r.Elapsed.Seconds()
	}

	root := xmlTestSuites{
		Name:     "epp-conformance",
		Tests:    totalTests,
		Failures: totalFailures,
		Errors:   0,
		Time:     fmt.Sprintf("%.3f", totalElapsed),
		Suites:   suites,
	}

	if _, err := fmt.Fprint(w, xml.Header); err != nil {
		return fmt.Errorf("report: write xml header: %w", err)
	}

	data, err := xml.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("report: marshal junit xml: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("report: write junit xml: %w", err)
	}

	return nil
}

// sanitizeClassname converts a scenario name to a classname-safe string by
// replacing "/" and " " with "_".
func sanitizeClassname(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, " ", "_")
	return name
}
