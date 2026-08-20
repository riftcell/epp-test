//go:build unit

package report

import (
	"bytes"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/riftcell/epp-test/pkg/runner"
)

// testResults returns a small slice of ScenarioResult for use in all tests.
func testResults() []runner.ScenarioResult {
	return []runner.ScenarioResult{
		{
			Name:      "domain_lifecycle",
			Registrar: "internetx",
			File:      "scenarios/conformance/domain_lifecycle.yaml",
			RFC:       "RFC 5731",
			Passed:    false,
			Elapsed:   1234 * time.Millisecond,
			Steps: []runner.StepResult{
				{
					Name:    "check_avail",
					Op:      "check_domain",
					Passed:  true,
					Elapsed: 100 * time.Millisecond,
					Err:     "",
				},
				{
					Name:    "create",
					Op:      "create_domain",
					Passed:  false,
					Elapsed: 200 * time.Millisecond,
					Err:     "expected EPP code 1000, got 2302",
				},
			},
		},
	}
}

// --- Task 1: JUnit XML tests ---

// TestJUnit_Hierarchy verifies that writeJUnit produces the three-level
// testsuites > testsuite > testcase structure with correct attributes, and that
// a <failure> element appears only on the failing step.
func TestJUnit_Hierarchy(t *testing.T) {
	results := testResults()
	var buf bytes.Buffer
	require.NoError(t, writeJUnit(&buf, results))

	out := buf.String()

	// Root element must be <testsuites>
	assert.Contains(t, out, "<testsuites", "root element must be testsuites")
	assert.Contains(t, out, `name="epp-conformance"`, "testsuites name attribute")

	// One <testsuite> per ScenarioResult
	assert.Contains(t, out, "<testsuite", "must contain testsuite element")
	assert.Contains(t, out, `name="domain_lifecycle"`, "testsuite name")

	// One <testcase> per StepResult
	assert.Contains(t, out, `name="check_avail"`, "passing step testcase")
	assert.Contains(t, out, `name="create"`, "failing step testcase")

	// <failure> only on the failing step
	assert.Contains(t, out, "<failure", "must have a failure element")
	assert.Contains(t, out, "expected EPP code 1000, got 2302", "failure message")

	// tests and failures attributes at testsuites level
	assert.Contains(t, out, `tests="2"`, "total tests count")
	assert.Contains(t, out, `failures="1"`, "total failures count")
}

// TestJUnit_Timestamp verifies the timestamp attribute matches the UTC ISO 8601
// format without timezone and without fractional seconds (RESEARCH Pitfall 4).
func TestJUnit_Timestamp(t *testing.T) {
	results := testResults()
	var buf bytes.Buffer
	require.NoError(t, writeJUnit(&buf, results))

	out := buf.String()

	// Extract timestamp attribute value
	tsRe := regexp.MustCompile(`timestamp="([^"]+)"`)
	matches := tsRe.FindStringSubmatch(out)
	require.Len(t, matches, 2, "timestamp attribute must be present")

	isoRe := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}$`)
	assert.True(t, isoRe.MatchString(matches[1]),
		"timestamp %q must match YYYY-MM-DDTHH:MM:SS (no timezone, no fractional seconds)", matches[1])
}

// TestJSON_RoundTrip verifies that writeJSON produces JSON that unmarshals back
// into []runner.ScenarioResult with the same Name/Passed/Steps values.
func TestJSON_RoundTrip(t *testing.T) {
	results := testResults()
	var buf bytes.Buffer
	require.NoError(t, writeJSON(&buf, results))

	var decoded []runner.ScenarioResult
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))

	require.Len(t, decoded, 1)
	assert.Equal(t, results[0].Name, decoded[0].Name)
	assert.Equal(t, results[0].Passed, decoded[0].Passed)
	require.Len(t, decoded[0].Steps, 2)
	assert.Equal(t, results[0].Steps[0].Name, decoded[0].Steps[0].Name)
	assert.Equal(t, results[0].Steps[1].Name, decoded[0].Steps[1].Name)
	assert.Equal(t, results[0].Steps[1].Passed, decoded[0].Steps[1].Passed)
}

// --- Task 2: Text and HTML writers + Write dispatcher ---

// TestText_PassFail verifies writeText produces lines containing PASS/FAIL and
// the scenario names.
func TestText_PassFail(t *testing.T) {
	results := []runner.ScenarioResult{
		{
			Name:      "domain_lifecycle",
			Registrar: "internetx",
			Passed:    true,
			Elapsed:   500 * time.Millisecond,
		},
		{
			Name:      "negative_tests",
			Registrar: "eurid",
			Passed:    false,
			Elapsed:   300 * time.Millisecond,
			Steps: []runner.StepResult{
				{Name: "dupe_create", Op: "create_domain", Passed: false, Err: "object exists"},
			},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, writeText(&buf, results))
	out := buf.String()

	assert.Contains(t, out, "PASS", "must contain PASS")
	assert.Contains(t, out, "FAIL", "must contain FAIL")
	assert.Contains(t, out, "domain_lifecycle", "must contain passing scenario name")
	assert.Contains(t, out, "negative_tests", "must contain failing scenario name")
}

// TestHTML_Escaping verifies writeHTML escapes scenario names that contain
// angle brackets and ampersands (RESEARCH Pitfall 5 / html/template).
func TestHTML_Escaping(t *testing.T) {
	results := []runner.ScenarioResult{
		{
			Name:      "<domain:check> & test",
			Registrar: "nicat",
			Passed:    true,
			Elapsed:   100 * time.Millisecond,
		},
	}

	var buf bytes.Buffer
	require.NoError(t, writeHTML(&buf, results))
	out := buf.String()

	// html/template must escape the angle brackets and ampersand
	assert.Contains(t, out, "&lt;domain:check&gt; &amp; test",
		"html/template must escape < > & in scenario names")
	assert.NotContains(t, out, "<domain:check>",
		"raw angle brackets must not appear in output")
}

// TestWrite_AllFormats verifies Write(dir, "junit,json,text,html", results)
// creates all four files in the output directory, each non-empty.
func TestWrite_AllFormats(t *testing.T) {
	dir := t.TempDir()
	results := testResults()

	require.NoError(t, Write(dir, "junit,json,text,html", results))

	for _, name := range []string{"junit.xml", "report.json", "report.txt", "report.html"} {
		data, err := os.ReadFile(dir + "/" + name)
		require.NoError(t, err, "file %s must exist", name)
		assert.NotEmpty(t, data, "file %s must be non-empty", name)
	}
}

// TestWrite_SingleFormat verifies that Write(dir, "junit", results) creates
// ONLY junit.xml and does not create the other three files.
func TestWrite_SingleFormat(t *testing.T) {
	dir := t.TempDir()
	results := testResults()

	require.NoError(t, Write(dir, "junit", results))

	// junit.xml must exist
	_, err := os.Stat(dir + "/junit.xml")
	require.NoError(t, err, "junit.xml must exist")

	// Other files must NOT exist
	for _, name := range []string{"report.json", "report.txt", "report.html"} {
		_, err := os.Stat(dir + "/" + name)
		assert.True(t, os.IsNotExist(err), "file %s must not exist when format is junit only", name)
	}
}

// TestWrite_DefaultFormats verifies that Write(dir, "", results) defaults to
// all four formats.
func TestWrite_DefaultFormats(t *testing.T) {
	dir := t.TempDir()
	results := testResults()

	require.NoError(t, Write(dir, "", results))

	for _, name := range []string{"junit.xml", "report.json", "report.txt", "report.html"} {
		_, err := os.Stat(dir + "/" + name)
		require.NoError(t, err, "file %s must exist when formats is empty (default all)", name)
	}

	_ = strings.Contains // suppress unused import warning
}
