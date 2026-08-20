//go:build unit

package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScenarioUnmarshal(t *testing.T) {
	raw := []byte(`
name: domain lifecycle
rfc: "RFC 5731 §3.2"
matrix: [internetx, nicat, eurid]
steps:
  - name: check
    op: check_domain
    params:
      names: [example.at]
  - name: create
    op: create_domain
    params:
      name: example.at
`)
	s, err := unmarshalScenario(raw)
	require.NoError(t, err)
	require.Equal(t, "domain lifecycle", s.Name)
	require.Equal(t, "RFC 5731 §3.2", s.RFC)
	require.Equal(t, []string{"internetx", "nicat", "eurid"}, s.Matrix)
	require.Len(t, s.Steps, 2)
}

func TestScenarioExpectParsing(t *testing.T) {
	raw := []byte(`
name: check test
steps:
  - name: check
    op: check_domain
    params:
      names: [example.at]
    expect:
      code: 1000
      fields:
        available: true
`)
	s, err := unmarshalScenario(raw)
	require.NoError(t, err)
	require.Len(t, s.Steps, 1)
	step := s.Steps[0]
	require.NotNil(t, step.Expect)
	require.Equal(t, 1000, step.Expect.Code)
	require.Equal(t, true, step.Expect.Fields["available"])
}

func TestLoadScenario(t *testing.T) {
	yaml := `name: temp scenario
rfc: "RFC 5730"
matrix: [internetx]
steps:
  - name: ping
    op: ping
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))

	s, err := LoadScenario(path)
	require.NoError(t, err)
	require.NotNil(t, s)
	require.Equal(t, "temp scenario", s.Name)
	require.Equal(t, "RFC 5730", s.RFC)
	require.Equal(t, []string{"internetx"}, s.Matrix)
	require.Len(t, s.Steps, 1)

	// Non-existent path returns a wrapped error mentioning the path.
	missingPath := filepath.Join(dir, "nonexistent.yaml")
	_, err = LoadScenario(missingPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), missingPath)
}

func TestDeepMerge(t *testing.T) {
	base := map[string]any{
		"name":   "d.at",
		"period": 1,
		"ext":    map[string]any{"a": 1},
	}
	override := map[string]any{
		"ext": map[string]any{"b": 2},
	}

	merged := deepMerge(base, override)

	// Base keys survive.
	require.Equal(t, "d.at", merged["name"])
	require.Equal(t, 1, merged["period"])

	// Nested map merges: both a and b present.
	ext, ok := merged["ext"].(map[string]any)
	require.True(t, ok, "ext must be map[string]any")
	require.Equal(t, 1, ext["a"], "base nested key must survive")
	require.Equal(t, 2, ext["b"], "override nested key must be added")

	// Original maps must not be mutated.
	origExt := base["ext"].(map[string]any)
	require.Len(t, origExt, 1, "base map must not be mutated")

	// Scalar override replaces base.
	base2 := map[string]any{"name": "old", "period": 1}
	override2 := map[string]any{"name": "new"}
	merged2 := deepMerge(base2, override2)
	require.Equal(t, "new", merged2["name"])
	require.Equal(t, 1, merged2["period"])
}

// unmarshalScenario is a thin helper for tests that need raw YAML bytes
// without going through the file system.
func unmarshalScenario(data []byte) (*Scenario, error) {
	tmp, err := os.CreateTemp("", "scenario-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("create temp: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		return nil, fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("close temp: %w", err)
	}
	return LoadScenario(tmp.Name())
}
